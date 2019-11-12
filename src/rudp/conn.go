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

func (cc *ConnConfig) Check() {
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
	inited     bool
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

	userdata interface{}
}

func (conn *Conn) RemoteAddr() string {
	return conn.remoteAddr
}

func (conn *Conn) LocalAddr() string {
	return conn.localAddr
}

func (conn *Conn) Close() {
	conn.exit = true
	conn.workResultLock.Wait()
	conn.conn.Close()
}

func (conn *Conn) IsConnected() bool {
	return !conn.exit && conn.inited
}

func (conn *Conn) SetUserData(userdata interface{}) {
	conn.userdata = userdata
}

func (conn *Conn) UserData() interface{} {
	return conn.userdata
}

func (conn *Conn) Write(bytes []byte) (int, error) {

	if conn.exit {
		return 0, errors.New("write on closed conn " + conn.localAddr + "->" + conn.remoteAddr)
	}

	size := len(bytes)
	if size > conn.fm.GetSendBufferLeft() {
		size = conn.fm.GetSendBufferLeft()
	}

	if size <= 0 {
		return 0, nil
	}

	conn.rudpActiveSendTime = common.GetNowUpdateInSecond()
	conn.fm.WriteSendBuffer(bytes[0:size])
	return size, nil
}

func (conn *Conn) Read(bytes []byte) (int, error) {

	if conn.exit {
		return 0, errors.New("read on closed conn " + conn.localAddr + "->" + conn.remoteAddr)
	}

	if conn.fm.GetRecvBufferSize() <= 0 {
		return 0, nil
	}

	conn.rudpActiveRecvTime = common.GetNowUpdateInSecond()
	size := copy(bytes, conn.fm.GetRecvReadLineBuffer())
	conn.fm.SkipRecvBuffer(size)
	return size, nil
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
