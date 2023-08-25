package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/GallifreyGoTutoural/ggt-rpc/codec"
	"io"
	"log"
	"net"
	"reflect"
	"strings"
	"sync"
	"time"
)

const MagicNumber = 0x3bef5c

type Option struct {
	MagicNumber          int           // MagicNumber marks this's a ggtrpc request
	CodecType            codec.Type    // client may choose different Codec to encode body
	ConnectionTimeoutSec time.Duration // 0 means no limit
	HandleTimeoutSec     time.Duration // 0 means no limit
}

var DefaultOption = &Option{
	MagicNumber:          MagicNumber,
	CodecType:            codec.GobType,
	ConnectionTimeoutSec: 10 * time.Second,
	HandleTimeoutSec:     0,
}

// Server represents an RPC Server.
type Server struct {
	serviceMap sync.Map
}

// NewServer returns a new Server.
func NewServer() *Server {
	return &Server{}
}

// DefaultServer is the default instance of *Server.
var DefaultServer = NewServer()

// Accept accepts connections on the listener and serves requests for each incoming connection.
func (server *Server) Accept(lis net.Listener) {
	for {
		conn, err := lis.Accept()
		if err != nil {
			log.Printf("rpc server: accept error: %v", err)
			return
		}
		go server.ServerConn(conn)
	}
}

// Accept accepts connections on the listener and serves requests for each incoming connection.
func Accept(lis net.Listener) {
	DefaultServer.Accept(lis)
}

// ServerConn runs the server on a single connection.
// ServerConn blocks, serving the connection until the client hangs up.
func (server *Server) ServerConn(conn net.Conn) {
	defer func() {
		_ = conn.Close()
	}()
	var opt Option
	if err := json.NewDecoder(conn).Decode(&opt); err != nil {
		log.Printf("rpc server: options error: %v", err)
		return
	}
	if opt.MagicNumber != MagicNumber {
		log.Printf("rpc server: invalid magic number %x", opt.MagicNumber)
		return
	}
	f := codec.NewCodecFuncMap[opt.CodecType]
	if f == nil {
		log.Printf("rpc server: invalid codec type %s", opt.CodecType)
		return
	}
	server.serveCodec(f(conn), &opt)

}

var invalidRequest = struct{}{}

func (server *Server) serveCodec(cc codec.Codec, option *Option) {
	sending := new(sync.Mutex) // make sure to send a complete response
	wg := new(sync.WaitGroup)  // wait until all request are handled

	for {
		req, err := server.readRequest(cc)
		if err != nil {
			if req == nil {
				break // it's not possible to recover, so close the connection
			}
			server.sendResponse(cc, req.h, invalidRequest, sending)
			continue
		}
		wg.Add(1)
		go server.handleRequest(cc, req, sending, wg, option.HandleTimeoutSec)

	}
	wg.Wait()
	_ = cc.Close()
}

type request struct {
	h            *codec.Header // header of request
	argv, replyv reflect.Value // argv and replyv of request
	mtype        *methodType   // type of request
	svc          *service      // service of request
}

func (server *Server) readRequestHeader(cc codec.Codec) (*codec.Header, error) {
	var h codec.Header
	if err := cc.ReadHeader(&h); err != nil {
		if err != io.EOF && !errors.Is(err, io.ErrUnexpectedEOF) {
			log.Println("rpc server: read header error:", err)
		}
		return nil, err
	}
	return &h, nil
}

func (server *Server) readRequest(cc codec.Codec) (*request, interface{}) {
	h, err := server.readRequestHeader(cc)
	if err != nil {
		return nil, err
	}
	req := &request{h: h}

	req.svc, req.mtype, err = server.findService(h.ServiceMethod)
	if err != nil {
		return req, err
	}
	req.argv = req.mtype.newArgv()
	req.replyv = req.mtype.newReplyv()

	// make sure argv is a pointer, read request body to argv
	argvi := req.argv.Interface()
	if req.argv.Type().Kind() != reflect.Pointer {
		argvi = req.argv.Addr().Interface()
	}

	if err = cc.ReadBody(argvi); err != nil {
		log.Println("rpc server: read argv err:", err)
		return req, err
	}
	return req, nil

}

func (server *Server) sendResponse(cc codec.Codec, h *codec.Header, body interface{}, sending *sync.Mutex) {
	sending.Lock()
	defer sending.Unlock()
	if err := cc.Write(h, body); err != nil {
		log.Println("rpc server: write response error:", err)
	}
}

func (server *Server) handleRequest(cc codec.Codec, req *request, sending *sync.Mutex, wg *sync.WaitGroup, timeout time.Duration) {
	defer wg.Done()
	done := make(chan struct{})
	var sendOnce sync.Once

	go func() {
		var errString string
		var body interface{}
		defer func() {
			sendOnce.Do(func() {
				req.h.Error = errString
				server.sendResponse(cc, req.h, body, sending)
			})
			close(done)
		}()
		// invoke the service method
		err := req.svc.call(req.mtype, req.argv, req.replyv)
		if err != nil {
			errString = err.Error()
			body = invalidRequest
			return
		}
	}()
	if timeout == 0 {
		<-done
		return
	}
	select {
	case <-time.After(timeout):
		sendOnce.Do(func() {
			req.h.Error = fmt.Sprintf("rpc server: request handle timeout: expect within %s", timeout)
			server.sendResponse(cc, req.h, invalidRequest, sending)
		})

	case <-done:
		return
	}
}

// Register publishes in the server the set of methods of the receiver value that satisfy the following conditions:
// - exported method of exported type
// - two arguments, both of exported type or builtin type
// - the second argument is a pointer
// - one return value, of type error
func (server *Server) Register(rcvr interface{}) error {
	s := newService(rcvr)
	// if service already exist, return error
	// otherwise, store the service
	if _, dup := server.serviceMap.LoadOrStore(s.name, s); dup {
		return errors.New("rpc: service already defined: " + s.name)
	}
	return nil
}

// Register publishes the receiver's methods in the DefaultServer.
func Register(rcvr interface{}) error { return DefaultServer.Register(rcvr) }

// findService looks up the request service.
func (server *Server) findService(serviceMethod string) (svc *service, mType *methodType, err error) {

	dot := strings.LastIndex(serviceMethod, ".")
	if dot < 0 {
		err = errors.New("rpc server: service/method request ill-formed: " + serviceMethod)
		return
	}
	// get service name and method name
	serviceName, methodName := serviceMethod[:dot], serviceMethod[dot+1:]

	// get service
	svci, ok := server.serviceMap.Load(serviceName)
	if !ok {
		err = errors.New("rpc server: can't find service " + serviceName)
		return
	}
	svc = svci.(*service)

	// get method
	mType = svc.method[methodName]
	if mType == nil {
		err = errors.New("rpc server: can't find method " + methodName)
	}
	return

}

// following code is used to test

type Foo int
type Args struct{ Num1, Num2 int }

// Sum exported method
func (f Foo) Sum(args Args, reply *int) error {
	*reply = args.Num1 + args.Num2
	return nil
}

// unexported method
func (f Foo) sum(args Args, reply *int) error {
	*reply = args.Num1 + args.Num2
	return nil
}
