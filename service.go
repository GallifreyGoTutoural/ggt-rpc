package main

import (
	"go/ast"
	"log"
	"reflect"
	"sync/atomic"
)

type methodType struct {
	method    reflect.Method
	ArgType   reflect.Type
	ReplyType reflect.Type
	numCalls  uint64
}

func (m *methodType) NumCalls() uint64 {
	return atomic.LoadUint64(&m.numCalls)
}

func (m *methodType) newArgv() reflect.Value {
	var argv reflect.Value
	if m.ArgType.Kind() == reflect.Pointer {
		// if arg is a pointer, new a value and return its address(pointer)
		argv = reflect.New(m.ArgType.Elem())
	} else {
		// if arg is not a pointer, new a value and return it
		argv = reflect.New(m.ArgType).Elem()
	}
	return argv
}

func (m *methodType) newReplyv() reflect.Value {
	// reply must be a pointer type
	replyv := reflect.New(m.ReplyType.Elem())
	// if reply is a map or slice, make it
	switch m.ReplyType.Elem().Kind() {
	case reflect.Map:
		replyv.Elem().Set(reflect.MakeMap(m.ReplyType.Elem()))
	case reflect.Slice:
		replyv.Elem().Set(reflect.MakeSlice(m.ReplyType.Elem(), 0, 0))
	}
	return replyv
}

type service struct {
	name   string                 // name of service
	typ    reflect.Type           // type of service
	rcvr   reflect.Value          // receiver of methods for the service
	method map[string]*methodType // registered methods
}

func newService(rcvr interface{}) *service {
	s := new(service)
	s.rcvr = reflect.ValueOf(rcvr)
	s.name = reflect.Indirect(s.rcvr).Type().Name()
	s.typ = reflect.TypeOf(rcvr)
	s.method = make(map[string]*methodType)
	// register all methods
	s.registerMethods()
	return s
}

func (s *service) registerMethods() {
	s.method = make(map[string]*methodType)
	for i := 0; i < s.typ.NumMethod(); i++ {
		method := s.typ.Method(i)
		mType := method.Type
		mName := method.Name
		// num of in args must be 3 ( 0 is receiver, 1 is args, 2 is *reply(reply must be a pointer))
		// num of out args must be 1 ( 0 is error)
		if mType.NumIn() != 3 || mType.NumOut() != 1 {
			continue
		}
		// out arg must be error
		if mType.Out(0) != reflect.TypeOf((*error)(nil)).Elem() {
			continue
		}
		// get arg and reply type
		argType, replyType := mType.In(1), mType.In(2)
		// arg and reply type must be exported or builtin
		if !isExportedOrBuiltinType(argType) || !isExportedOrBuiltinType(replyType) {
			continue
		}
		s.method[mName] = &methodType{
			method:    method,
			ArgType:   argType,
			ReplyType: replyType,
		}
		log.Printf("rpc server: register %s.%s\n", s.name, mName)
	}
}

func isExportedOrBuiltinType(t reflect.Type) bool {
	return ast.IsExported(t.Name()) || t.PkgPath() == ""
}

func (s *service) call(m *methodType, argv, rplyv reflect.Value) error {
	atomic.AddUint64(&m.numCalls, 1)
	f := m.method.Func
	ret := f.Call([]reflect.Value{s.rcvr, argv, rplyv})
	if errInter := ret[0].Interface(); errInter != nil {
		return errInter.(error)
	}
	return nil
}
