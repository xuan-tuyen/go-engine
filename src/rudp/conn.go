package rudp

import (
	"errors"
	"github.com/esrrhs/go-engine/src/common"
	"github.com/esrrhs/go-engine/src/frame"
	"net"
	"sync"
	"time"
)

const (
	RUDP_MAX_SIZE int = 500
	RUDP_MAX_ID   int = 100000
)

type ConnConfig struct {
	BufferSize       int
	MaxWin           int
	ResendTimems     int
	Compress         int
	Stat             int
	Timeout          int
	Backlog          int
	ConnectTimeoutMs int
}

func (cc *ConnConfig) check() {
	if cc.BufferSize == 0 {
		cc.BufferSize = 1024 * 1024
	}
	if cc.MaxWin == 0 {
		cc.MaxWin = 1000
	}
	if cc.ResendTimems == 0 {
		cc.ResendTimems = 400
	}
	if cc.Timeout == 0 {
		cc.Timeout = 60
	}
	if cc.Backlog == 0 {
		cc.Backlog = 100
	}
	if cc.ConnectTimeoutMs == 0 {
		cc.ConnectTimeoutMs = 1000
	}
}

type Conn struct {
	isListener bool
	config     ConnConfig
	exit       bool
	localAddr  string
	remoteAddr string

	activeRecvTime     time.Time
	activeSendTime     time.Time
	rudpActiveRecvTime time.Time
	rudpActiveSendTime time.Time

	conn *net.UDPConn
	fm   *frame.FrameMgr

	workResultLock sync.WaitGroup

	localAddrToConnMap sync.Map
	waitAccept         chan *Conn
}

func (conn *Conn) Close() {
	conn.exit = true
	conn.workResultLock.Wait()
	conn.conn.Close()
}

func (conn *Conn) Write(bytes []byte) (bool, error) {

	if conn.exit {
		return false, errors.New("write on closed conn " + conn.localAddr + "->" + conn.remoteAddr)
	}

	if len(bytes) > conn.fm.GetSendBufferLeft() {
		return false, nil
	}

	conn.rudpActiveSendTime = common.GetNowUpdateInSecond()
	conn.fm.WriteSendBuffer(bytes)
	return true, nil
}

func (conn *Conn) Read() ([]byte, error) {

	if conn.exit {
		return nil, errors.New("read on closed conn " + conn.localAddr + "->" + conn.remoteAddr)
	}

	if conn.fm.GetRecvBufferSize() <= 0 {
		return nil, nil
	}

	conn.rudpActiveRecvTime = common.GetNowUpdateInSecond()
	return conn.fm.GetRecvReadLineBuffer(), nil
}

func (conn *Conn) addClientConn(addr string, c *Conn) {
	conn.localAddrToConnMap.Store(addr, c)
}

func (conn *Conn) getClientConnByAddr(addr string) *Conn {
	ret, ok := conn.localAddrToConnMap.Load(addr)
	if !ok {
		return nil
	}
	return ret.(*Conn)
}

func (conn *Conn) deleteClientConn(addr string) {
	conn.localAddrToConnMap.Delete(addr)
}
