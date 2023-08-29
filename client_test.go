package main

import (
	"context"
	"net"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestClient_dialTimeout(t *testing.T) {
	t.Parallel()
	l, _ := net.Listen("tcp", ":0")
	defer l.Close()
	f := func(conn net.Conn, opt *Option) (client *Client, err error) {
		_ = conn.Close()
		time.Sleep(2 * time.Second)
		return nil, nil
	}
	t.Run("timeout", func(t *testing.T) {
		_, err := dialTimeout(f, "tcp", l.Addr().String(), &Option{ConnectionTimeoutSec: time.Second})
		_assert(err != nil && err.Error() == "rpc client: connect timeout", "dialTimeout() error:%v", err)
	})
	t.Run("no timeout", func(t *testing.T) {
		_, err := dialTimeout(f, "tcp", l.Addr().String(), &Option{ConnectionTimeoutSec: 0})
		_assert(err == nil, "dialTimeout() error:%v", err)
	})
}

type Bar int

func (b Bar) Timeout(argv int, reply *int) error {
	time.Sleep(time.Second * 2)
	return nil
}

func starServer(addr chan string) {
	var b Bar
	_ = Register(&b)
	l, _ := net.Listen("tcp", ":0")
	addr <- l.Addr().String()
	Accept(l)
}

func TestClient_Call(t *testing.T) {
	t.Parallel()
	addrCh := make(chan string)
	go starServer(addrCh)
	time.Sleep(time.Second)
	// client handle timeout
	t.Run("client handle timeout", func(t *testing.T) {
		client, _ := Dial("tcp", <-addrCh)
		var reply int
		ctx, _ := context.WithTimeout(context.Background(), time.Second)
		err := client.Call(ctx, "Bar.Timeout", 1, &reply)
		_assert(err != nil && strings.Contains(err.Error(), ctx.Err().Error()), "client.Call() error:%v", err)
	})
	// server handle timeout
	t.Run("server handle timeout", func(t *testing.T) {
		client, _ := Dial("tcp", <-addrCh, &Option{
			HandleTimeoutSec: time.Second,
		})
		var reply int
		err := client.Call(context.Background(), "Bar.Timeout", 1, &reply)
		_assert(err != nil && strings.Contains(err.Error(), "timeout"), "client.Call() error:%v", err)
	})
}

func TestXDial(t *testing.T) {
	if runtime.GOOS == "linux" {
		ch := make(chan struct{})
		addr := "tmp/ggtrpc.sock"
		go func() {
			_ = os.Remove(addr)
			l, err := net.Listen("unix", addr)
			if err != nil {
				t.Fatal(err)
			}
			ch <- struct{}{}
			Accept(l)
		}()
		<-ch
		_, err := XDial("unix@" + addr)
		_assert(err == nil, "XDial() error:%v", err)

	}
	if runtime.GOOS == "windows" {
		addrCh := make(chan string)
		go starServer(addrCh)
		_, err := XDial("tcp@" + <-addrCh)
		_assert(err == nil, "XDial() error:%v", err)
	}
}
