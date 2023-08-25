package main

import (
	"fmt"
	"reflect"
	"testing"
)

func _assert(cond bool, msg string, v ...interface{}) {
	if !cond {
		panic(fmt.Sprintf("assertion failed: "+msg, v...))
	}
}

func TestNewService(t *testing.T) {
	var foo Foo
	s := newService(&foo)
	_assert(s.typ.NumMethod() == 1, "wrong method number, expect 1, but got %d", s.typ.NumMethod())
	mType := s.typ.Method(0).Type
	_assert(mType != nil, "wrong method type, expect not nil, but got nil")
}

func TestMethodType_Call(t *testing.T) {
	var foo Foo
	s := newService(&foo)
	mType := s.method["Sum"]
	argv := mType.newArgv()
	replyv := mType.newReplyv()
	argv.Set(reflect.ValueOf(Args{Num1: 1, Num2: 2}))
	err := s.call(mType, argv, replyv)
	_assert(err == nil, "call Foo.Sum error:%v", err)

}
