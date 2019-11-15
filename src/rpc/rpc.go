package rpc

import (
	"errors"
	"github.com/esrrhs/go-engine/src/common"
	"sync"
	"time"
)

type Rpc struct {
	callMap sync.Map
}

func NewRpc() *Rpc {
	r := &Rpc{}
	return r
}

func (r *Rpc) NewCall(timeoutms int) *RpcCall {
	c := &RpcCall{
		fr:        r,
		timeoutms: timeoutms,
		id:        common.Guid(),
		retc:      make(chan int, 1),
	}
	r.callMap.Store(c.id, c)
	return c
}

func (r *Rpc) PutRet(id string, ret ...interface{}) {
	v, ok := r.callMap.Load(id)
	if !ok {
		return
	}
	rc := v.(*RpcCall)
	rc.result = ret
	rc.retc <- 1
}

type RpcCall struct {
	fr        *Rpc
	timeoutms int
	id        string
	result    []interface{}
	retc      chan int
}

func (r *RpcCall) Id() string {
	return r.id
}

func (r *RpcCall) Call(f func()) ([]interface{}, error) {
	f()

	select {
	case _ = <-r.retc:
		r.fr.callMap.Delete(r.id)
		return r.result, nil
	case <-time.After(time.Duration(r.timeoutms) * time.Millisecond):
		r.fr.callMap.Delete(r.id)
		return nil, errors.New("time out")
	}
}
