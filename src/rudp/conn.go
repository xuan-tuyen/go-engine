package rudp

import (
	"errors"
	"github.com/esrrhs/go-engine/src/common"
	"github.com/esrrhs/go-engine/src/frame"
	"github.com/esrrhs/go-engine/src/loggo"
	"github.com/golang/protobuf/proto"
	"net"
	"time"
)

const (
	RUDP_MAX_SIZE int = 500
	RUDP_MAX_ID   int = 100000
)

type ConnConfig struct {
	BufferSize   int
	MaxWin       int
	ResendTimems int
	Compress     int
	Stat         int
	Timeout      int
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
}

type Conn struct {
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
}

func (conn *Conn) Close() {
	conn.exit = true

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

func (conn *Conn) update() {

	loggo.Info("start rudp conn %s->%s", conn.localAddr, conn.remoteAddr)

	conn.activeRecvTime = common.GetNowUpdateInSecond()
	conn.activeSendTime = common.GetNowUpdateInSecond()
	conn.rudpActiveRecvTime = common.GetNowUpdateInSecond()
	conn.rudpActiveSendTime = common.GetNowUpdateInSecond()

	bytes := make([]byte, 2000)

	for !conn.exit {
		now := common.GetNowUpdateInSecond()
		sleep := true

		conn.fm.Update()

		// send udp
		sendlist := conn.fm.GetSendList()
		if sendlist.Len() > 0 {
			sleep = false
			conn.activeSendTime = now
			for e := sendlist.Front(); e != nil; e = e.Next() {
				f := e.Value.(*frame.Frame)
				mb, _ := conn.fm.MarshalFrame(f)
				conn.conn.SetReadDeadline(time.Now().Add(time.Millisecond * 100))
				conn.conn.Write(mb)
			}
		}

		// recv udp
		conn.conn.SetReadDeadline(time.Now().Add(time.Millisecond * 100))
		n, _ := conn.conn.Read(bytes)
		if n > 0 {
			conn.activeRecvTime = now
			f := &frame.Frame{}
			proto.Unmarshal(bytes[0:n], f)
			conn.fm.OnRecvFrame(f)
		}

		// timeout
		diffrecv := now.Sub(conn.activeRecvTime)
		diffsend := now.Sub(conn.activeSendTime)
		tcpdiffrecv := now.Sub(conn.rudpActiveRecvTime)
		tcpdiffsend := now.Sub(conn.rudpActiveSendTime)
		if diffrecv > time.Second*(time.Duration(conn.config.Timeout)) || diffsend > time.Second*(time.Duration(conn.config.Timeout)) ||
			tcpdiffrecv > time.Second*(time.Duration(conn.config.Timeout)) || tcpdiffsend > time.Second*(time.Duration(conn.config.Timeout)) {
			loggo.Debug("close inactive conn %s->%s", conn.localAddr, conn.remoteAddr)
			conn.fm.Close()
			break
		}

		if conn.fm.IsRemoteClosed() {
			loggo.Debug("closed by remote conn %s->%s", conn.localAddr, conn.remoteAddr)
			conn.fm.Close()
			break
		}

		if sleep {
			time.Sleep(time.Millisecond * 10)
		}
	}

	conn.fm.Close()

	startCloseTime := common.GetNowUpdateInSecond()
	for !conn.exit {
		now := common.GetNowUpdateInSecond()

		conn.fm.Update()

		// send udp
		sendlist := conn.fm.GetSendList()
		for e := sendlist.Front(); e != nil; e = e.Next() {
			f := e.Value.(*frame.Frame)
			mb, _ := conn.fm.MarshalFrame(f)
			conn.conn.SetReadDeadline(time.Now().Add(time.Millisecond * 100))
			conn.conn.Write(mb)
		}

		// recv udp
		conn.conn.SetReadDeadline(time.Now().Add(time.Millisecond * 100))
		n, _ := conn.conn.Read(bytes)
		if n > 0 {
			f := &frame.Frame{}
			proto.Unmarshal(bytes[0:n], f)
			conn.fm.OnRecvFrame(f)
		}

		diffclose := now.Sub(startCloseTime)
		if diffclose > time.Second*5 {
			loggo.Info("close conn had timeout %s->%s", conn.localAddr, conn.remoteAddr)
			break
		}

		remoteclosed := conn.fm.IsRemoteClosed()
		if remoteclosed {
			loggo.Info("remote conn had closed %s->%s", conn.localAddr, conn.remoteAddr)
			break
		}

		time.Sleep(time.Millisecond * 100)
	}

	conn.conn.Close()
	conn.exit = true

	loggo.Info("close rudp conn %s->%s", conn.localAddr, conn.remoteAddr)
}
