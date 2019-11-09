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

func Dail(targetAddr string, cc *ConnConfig) (*Conn, error) {
	if cc == nil {
		cc = &ConnConfig{}
	}
	cc.check()

	conn := &Conn{}

	startConnectTime := time.Now()
	c, err := net.DialTimeout("udp", targetAddr, time.Millisecond*time.Duration(cc.ConnectTimeoutMs/2))
	if err != nil {
		loggo.Debug("Error listening for udp packets: %s %s", targetAddr, err.Error())
		return nil, err
	}
	targetConn := c.(*net.UDPConn)
	conn.config = *cc
	conn.conn = targetConn
	conn.remoteAddr = targetAddr

	fm := frame.NewFrameMgr(RUDP_MAX_SIZE, RUDP_MAX_ID, cc.BufferSize, cc.MaxWin, cc.ResendTimems, cc.Compress, cc.Stat)
	conn.fm = fm

	fm.Connect()
	bytes := make([]byte, 2000)
	for {
		if fm.IsConnected() {
			break
		}

		fm.Update()

		// send udp
		sendlist := fm.GetSendList()
		for e := sendlist.Front(); e != nil; e = e.Next() {
			f := e.Value.(*frame.Frame)
			mb, _ := fm.MarshalFrame(f)
			conn.conn.SetReadDeadline(time.Now().Add(time.Millisecond * 100))
			conn.conn.Write(mb)
		}

		// recv udp
		conn.conn.SetReadDeadline(time.Now().Add(time.Millisecond * 100))
		n, _ := conn.conn.Read(bytes)
		if n > 0 {
			f := &frame.Frame{}
			proto.Unmarshal(bytes[0:n], f)
			fm.OnRecvFrame(f)
		}

		// timeout
		now := time.Now()
		diffclose := now.Sub(startConnectTime)
		if diffclose > time.Millisecond*time.Duration(cc.ConnectTimeoutMs) {
			loggo.Debug("can not connect remote rudp %s", targetAddr)
			return nil, errors.New("can not connect remote rudp " + targetAddr)
		}

		time.Sleep(time.Millisecond * 10)
	}

	conn.localAddr = conn.conn.LocalAddr().String()

	go conn.updateClient()

	return conn, nil
}

func (conn *Conn) updateClient() {

	conn.workResultLock.Add(1)
	defer conn.workResultLock.Done()

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

	conn.exit = true

	time.Sleep(time.Second)

	loggo.Info("close rudp conn %s->%s", conn.localAddr, conn.remoteAddr)
}
