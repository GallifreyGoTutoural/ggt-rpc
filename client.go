package main

import (
	"encoding/json"
	"errors"
	"github.com/GallifreyGoTutoural/ggt-rpc/codec"
	"io"
	"log"
	"net"
	"sync"
)

// Call represents an active RPC.
type Call struct {
	Seq           uint64      // sequence number chosen by client
	ServiceMethod string      // format "Service.Method"
	Args          interface{} // arguments to the function
	Reply         interface{} // reply from the function
	Error         error       // if error occurs, it will be set
	Done          chan *Call  // strobes when call is complete
}

func (call *Call) done() {
	call.Done <- call
}

// Client represents an RPC Client.
// There may be multiple outstanding Calls associated with a single Client, and a Client may be used by multiple goroutines simultaneously.
type Client struct {
	cc       codec.Codec
	opt      *Option
	sending  sync.Mutex // protect following
	header   codec.Header
	mu       sync.Mutex // protect following
	seq      uint64
	pending  map[uint64]*Call
	closing  bool // user has called Close
	shutdown bool // server has told us to stop
}

var ErrShutdown = errors.New("connection is shut down")

func (client *Client) Close() error {
	client.mu.Lock()
	defer client.mu.Unlock()
	if client.closing {
		return ErrShutdown
	}
	client.closing = true
	return client.cc.Close()
}

// insures Client implements io.Closer
var _ io.Closer = (*Client)(nil)

// IsAvailable returns true if the client does work; in other words, it's not shutdown and not closing.
func (client *Client) IsAvailable() bool {
	client.mu.Lock()
	defer client.mu.Unlock()
	return !client.shutdown && !client.closing
}

func (client *Client) registerCall(call *Call) (uint64, error) {
	client.mu.Lock()
	defer client.mu.Unlock()
	if client.closing || client.shutdown {
		return 0, ErrShutdown
	}
	call.Seq = client.seq
	client.pending[call.Seq] = call
	client.seq++
	return call.Seq, nil
}

func (client *Client) removeCall(seq uint64) *Call {
	client.mu.Lock()
	defer client.mu.Unlock()
	call := client.pending[seq]
	delete(client.pending, seq)
	return call
}

// terminateCalls terminates all pending calls.
func (client *Client) terminateCalls(err error) {
	client.sending.Lock()
	defer client.sending.Unlock()
	client.mu.Lock()
	defer client.mu.Unlock()
	client.shutdown = true
	for _, call := range client.pending {
		call.Error = err
		call.done()
	}
}

// parseOptions parses the options, and returns the default option if opts is nil or opts[0] is nil.
func parseOptions(opts ...*Option) (*Option, error) {
	// if opts is nil or opts[0] is nil, return default option
	if len(opts) == 0 || opts[0] == nil {
		return DefaultOption, nil
	}
	// if opts has more than one option, return error
	if len(opts) != 1 {
		return nil, errors.New("number of options is more than 1")
	}

	// if opts[0] is not nil, return opts[0]
	opt := opts[0]
	opt.MagicNumber = DefaultOption.MagicNumber
	if opt.CodecType == "" {
		opt.CodecType = DefaultOption.CodecType
	}
	return opt, nil
}

// Dial connects to an RPC server at the specified network address.
func Dial(network, address string, opts ...*Option) (client *Client, err error) {
	// parse options
	opt, err := parseOptions(opts...)
	if err != nil {
		return nil, err
	}
	// build connection
	conn, err := net.Dial(network, address)
	if err != nil {
		return nil, err
	}
	// close connection if error occurs
	defer func() {
		if err != nil {
			_ = conn.Close()
		}
	}()
	// return client
	return NewClient(conn, opt)
}

// NewClient returns a new Client.
func NewClient(conn net.Conn, opt *Option) (*Client, error) {
	// send options
	f := codec.NewCodecFuncMap[opt.CodecType]
	if f == nil {
		err := errors.New("invalid codec type")
		_ = conn.Close()
		return nil, err
	}
	// send options
	if err := json.NewEncoder(conn).Encode(opt); err != nil {
		log.Println("rpc client: options error:", err)
		_ = conn.Close()
		return nil, err
	}
	return newClientCodec(f(conn)), nil
}

// newClientCodec returns a ClientCodec with a given codec
func newClientCodec(cc codec.Codec) *Client {
	client := &Client{
		cc:      cc,
		seq:     1,
		pending: make(map[uint64]*Call),
	}
	// start a goroutine to receive response from server
	go client.receive()
	return client
}

// receive reads and handles response from server
func (client *Client) receive() {
	var err error
	for err == nil {
		var h codec.Header
		if err = client.cc.ReadHeader(&h); err != nil {
			break
		}
		call := client.removeCall(h.Seq)
		switch {
		case call == nil:
			// it usually means that Write partially failed, we will close the connection
			err = client.cc.ReadBody(nil)
		case h.Error != "":
			call.Error = errors.New(h.Error)
			err = client.cc.ReadBody(nil)
			call.done()
		default:
			err = client.cc.ReadBody(call.Reply)
			if err != nil {
				call.Error = errors.New("reading body" + err.Error())
			}
			call.done()
		}
	}
	// error occurs, so terminateCalls pending calls
	client.terminateCalls(err)
}

func (client *Client) send(call *Call) {
	// make sure that the client is available
	client.sending.Lock()
	defer client.sending.Unlock()
	// register this call
	seq, err := client.registerCall(call)
	if err != nil {
		call.Error = err
		call.done()
		return
	}
	// prepare request header
	client.header.ServiceMethod = call.ServiceMethod
	client.header.Seq = seq
	client.header.Error = ""

	// encode and send the request
	if err := client.cc.Write(&client.header, call.Args); err != nil {
		// send failed, so terminateCalls this call
		call := client.removeCall(seq)
		// call may be nil, it usually means that Write partially failed, we will close the connection
		if call != nil {
			call.Error = err
			call.done()
		}
	}

}

// Go invokes the function asynchronously.
// It returns the Call structure representing the invocation.
func (client *Client) Go(serviceMethod string, args, reply interface{}, done chan *Call) *Call {
	if done == nil {
		done = make(chan *Call, 10)
	} else if cap(done) == 0 {
		log.Panic("rpc client: done channel is unbuffered")
	}
	call := &Call{
		ServiceMethod: serviceMethod,
		Args:          args,
		Reply:         reply,
		Done:          done,
	}
	client.send(call)
	return call
}

// Call invokes the named function, waits for it to complete, and returns its error status.
func (client *Client) Call(serviceMethod string, args, reply interface{}) error {
	call := <-client.Go(serviceMethod, args, reply, make(chan *Call, 1)).Done
	return call.Error
}
