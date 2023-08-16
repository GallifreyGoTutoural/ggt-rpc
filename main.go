package main

import (
	"encoding/json"
	"github.com/GallifreyGoTutoural/ggt-rpc/codec"
	"log"
	"net"
	"time"
)

func startServer(addr chan string) {
	// :0 main a random port
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		log.Fatal("network error:", err)
	}
	log.Println("start rpc server on", l.Addr())
	addr <- l.Addr().String()
	DefaultServer.Accept(l)
}

func main() {
	addr := make(chan string)
	go startServer(addr)

	// build a connection with server
	conn, _ := net.Dial("tcp", <-addr)
	defer func() { _ = conn.Close() }()

	time.Sleep(time.Second)
	// send options
	_ = json.NewEncoder(conn).Encode(DefaultOption)
	cc := codec.NewGobCodec(conn)
	// send request
	for i := 0; i < 5; i++ {
		h := &codec.Header{
			ServiceMethod: "Foo.Sum",
			Seq:           uint64(i),
		}
		// write request
		_ = cc.Write(h, "ggt-rpc")
		// read response

	}

	//receive response
	time.Sleep(time.Second)
	for i := 0; i < 5; i++ {
		var respHeader codec.Header
		_ = cc.ReadHeader(&respHeader)
		log.Println("replyHeader:", respHeader)
		var reply string
		_ = cc.ReadBody(&reply)
		log.Println("replyBody:", reply)
	}

}
