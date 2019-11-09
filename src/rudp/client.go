package rudp

import (
	"errors"
	"github.com/esrrhs/go-engine/src/frame"
	"github.com/esrrhs/go-engine/src/loggo"
	"github.com/golang/protobuf/proto"
	"net"
	"time"
)

func Dail(targetAddr string, timeoutms int, cc *ConnConfig) (*Conn, error) {
	if cc == nil {
		cc = &ConnConfig{}
	}
	cc.check()

	conn := &Conn{}

	startConnectTime := time.Now()
	c, err := net.DialTimeout("udp", targetAddr, time.Millisecond*time.Duration(timeoutms/2))
	if err != nil {
		loggo.Debug("Error listening for udp packets: %s %s", targetAddr, err.Error())
		return nil, err
	}
	targetConn := c.(*net.UDPConn)
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
			err := proto.Unmarshal(bytes[0:n], f)
			if err != nil {
				loggo.Error("Unmarshal tcp Error %s", err)
				return nil, err
			}
			fm.OnRecvFrame(f)
		}

		// timeout
		now := time.Now()
		diffclose := now.Sub(startConnectTime)
		if diffclose > time.Millisecond*time.Duration(timeoutms) {
			loggo.Debug("can not connect remote rudp %s", targetAddr)
			return nil, errors.New("can not connect remote rudp " + targetAddr)
		}

		time.Sleep(time.Millisecond * 10)
	}

	go conn.update()

	return conn, nil
}
