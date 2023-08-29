package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"sync"
	"time"
)

func startServer(addr chan string) {
	var foo Foo
	// register a service
	if err := Register(&foo); err != nil {
		log.Fatal("register error:", err)
	}
	// pick a free port
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		log.Fatal("network error:", err)
	}
	HandleHTTP()
	log.Println("start rpc server on", l.Addr())
	// notify port
	addr <- l.Addr().String()
	// accept connection
	//Accept(l)
	_ = http.Serve(l, nil)
}

func call(addrCh chan string) {
	client, _ := DialHttp("tcp", <-addrCh)
	defer func() { _ = client.Close() }()

	time.Sleep(time.Second)
	var wg sync.WaitGroup
	// send request & receive response
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			args := &Args{Num1: i, Num2: i * i}
			var reply int

			if err := client.Call(context.Background(), "Foo.Sum", args, &reply); err != nil {
				log.Fatal("call Foo.Sum error:", err)
			}

			log.Printf("%d + %d = %d", args.Num1, args.Num2, reply)
		}(i)
	}
	wg.Wait()
}

func main() {
	log.SetFlags(0)
	addr := make(chan string)
	go call(addr)
	startServer(addr)

}
