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
	// send request & receive response
	for i := 0; i < 15; i++ {
		h := &codec.Header{
			ServiceMethod: "Foo.Sum",
			Seq:           uint64(i),
		}
		// write request
		_ = cc.Write(h, "ggt-rpc")
		// read response
		_ = cc.ReadHeader(h)
		var reply string
		_ = cc.ReadBody(&reply)
		log.Println("reply:", reply)
	}

}
