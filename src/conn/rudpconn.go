package conn

import (
	"context"
	"errors"
	"github.com/esrrhs/go-engine/src/common"
	"github.com/esrrhs/go-engine/src/frame"
	"github.com/esrrhs/go-engine/src/group"
	"github.com/esrrhs/go-engine/src/loggo"
	"github.com/golang/protobuf/proto"
	"net"
	"sync"
	"time"
)

type RudpConfig struct {
	CutSize          int
	MaxId            int
	BufferSize       int
	MaxWin           int
	ResendTimems     int
	Compress         int
	Stat             int
	HBTimeoutms      int
	ConnectTimeoutMs int
	CloseTimeoutMs   int
}

func DefaultRudpConfig() *RudpConfig {
	return &RudpConfig{
		CutSize:          500,
		MaxId:            1000000,
		BufferSize:       10240,
		MaxWin:           10000,
		ResendTimems:     200,
		Compress:         0,
		Stat:             0,
		HBTimeoutms:      3000,
		ConnectTimeoutMs: 1000,
		CloseTimeoutMs:   10000,
	}
}

type rudpConn struct {
	info          string
	config        *RudpConfig
	dialer        *rudpConnDialer
	listenersonny *rudpConnListenerSonny
	listener      *rudpConnListener
	cancel        context.CancelFunc
}

type rudpConnDialer struct {
	conn *net.UDPConn
}

type rudpConnListenerSonny struct {
	dstaddr    *net.UDPAddr
	fatherconn *net.UDPConn
	isclose    bool
	fm         *frame.FrameMgr
	wg         *group.Group
}

type rudpConnListener struct {
	listenerconn *net.UDPConn
	wg           *group.Group
	sonny        sync.Map
	accept       *common.Channel
}

const (
	MAX_RUDP_PACKET      = 10240
	RUDP_RECV_CHAN_LEN   = 128
	RUDP_ACCEPT_CHAN_LEN = 128
)

func (c *rudpConn) Name() string {
	return "rudp"
}

func (c *rudpConn) Read(p []byte) (n int, err error) {
	c.checkConfig()
	// TODO
	return 0, nil
}

func (c *rudpConn) Write(p []byte) (n int, err error) {
	c.checkConfig()
	// TODO
	return 0, nil
}

func (c *rudpConn) Close() error {
	c.checkConfig()

	// TODO
	return nil
}

func (c *rudpConn) Info() string {
	c.checkConfig()

	// TODO
	return ""
}

func (c *rudpConn) Dial(dst string) (Conn, error) {
	c.checkConfig()

	// TODO
	return nil, nil
}

func (c *rudpConn) Listen(dst string) (Conn, error) {
	c.checkConfig()

	ipaddr, err := net.ResolveUDPAddr("udp", dst)
	if err != nil {
		return nil, err
	}

	listenerconn, err := net.ListenUDP("udp", ipaddr)
	if err != nil {
		return nil, err
	}

	ch := common.NewChannel(UDP_ACCEPT_CHAN_LEN)

	wg := group.NewGroup("rudpConn Listen"+" "+dst, nil, func() {
		listenerconn.Close()
		ch.Close()
	})

	listener := &rudpConnListener{
		listenerconn: listenerconn,
		wg:           wg,
		accept:       ch,
	}

	u := &rudpConn{listener: listener}
	wg.Go("rudpConn loopListenerRecv"+" "+dst, func() error {
		return u.loopListenerRecv()
	})

	return u, nil
}

func (c *rudpConn) Accept() (Conn, error) {
	c.checkConfig()

	// TODO
	return nil, nil
}

func (c *rudpConn) checkConfig() {
	if c.config == nil {
		c.config = DefaultRudpConfig()
	}
}

func (c *rudpConn) SetConfig(config *RudpConfig) {
	c.config = config
}

func (c *rudpConn) loopListenerRecv() error {
	buf := make([]byte, MAX_RUDP_PACKET)
	for !c.listener.wg.IsExit() {
		n, srcaddr, err := c.listener.listenerconn.ReadFromUDP(buf)
		if err != nil {
			return err
		}

		srcaddrstr := srcaddr.String()

		v, ok := c.listener.sonny.Load(srcaddrstr)
		if !ok {
			id := common.Guid()
			fm := frame.NewFrameMgr(c.config.CutSize, c.config.MaxId, c.config.BufferSize, c.config.MaxWin, c.config.ResendTimems, c.config.Compress, c.config.Stat)
			fm.SetDebugid(id)

			sonny := &rudpConnListenerSonny{
				dstaddr:    srcaddr,
				fatherconn: c.listener.listenerconn,
				fm:         fm,
			}

			u := &rudpConn{config: c.config, listenersonny: sonny}
			c.listener.sonny.Store(srcaddrstr, u)

			c.listener.wg.Go("rudpConn accept"+" "+u.Info(), func() error {
				return c.accept(u)
			})

		} else {
			u := v.(*rudpConn)

			f := &frame.Frame{}
			err := proto.Unmarshal(buf[0:n], f)
			if err == nil {
				u.listenersonny.fm.OnRecvFrame(f)
			} else {
				loggo.Error("%s %s Unmarshal fail %s", c.Info(), u.Info(), err)
			}
		}

		c.listener.sonny.Range(func(key, value interface{}) bool {
			u := value.(*udpConn)
			if u.listenersonny.isclose {
				c.listener.sonny.Delete(key)
			}
			return true
		})
	}
	return nil
}

func (c *rudpConn) accept(u *rudpConn) error {

	loggo.Debug("server begin accept rudp %s", u.Info())

	startConnectTime := time.Now()
	done := false
	for !c.listener.wg.IsExit() {

		if u.listenersonny.fm.IsConnected() {
			done = true
			break
		}

		u.listenersonny.fm.Update()

		// send udp
		sendlist := u.listenersonny.fm.GetSendList()
		for e := sendlist.Front(); e != nil; e = e.Next() {
			f := e.Value.(*frame.Frame)
			mb, err := u.listenersonny.fm.MarshalFrame(f)
			if err != nil {
				loggo.Error("MarshalFrame fail %s", err)
				break
			}
			u.listenersonny.fatherconn.SetReadDeadline(time.Now().Add(time.Millisecond * 100))
			u.listenersonny.fatherconn.WriteToUDP(mb, u.listenersonny.dstaddr)
		}

		now := time.Now()
		diffclose := now.Sub(startConnectTime)
		if diffclose > time.Millisecond*time.Duration(c.config.ConnectTimeoutMs) {
			loggo.Debug("can not connect by remote rudp %s", u.Info())
			break
		}

		time.Sleep(time.Millisecond * 10)
	}

	if !done {
		u.Close()
		return nil
	}

	if c.listener.wg.IsExit() {
		u.Close()
		return nil
	}

	c.listener.accept.Write(u)

	wg := group.NewGroup("rudpConn serveListenerSonny"+" "+c.Info(), c.listener.wg, func() {
		u.Close()
	})

	u.listenersonny.wg = wg

	wg.Go("", func() error {
		return u.updateListenerSonny()
	})

	wg.Wait()

	return nil
}

func (c *rudpConn) updateListenerSonny() error {

	loggo.Debug("start rudp conn %s", c.Info())

	for !c.listenersonny.wg.IsExit() {

		sleep := true

		c.listenersonny.fm.Update()

		// send udp
		sendlist := c.listenersonny.fm.GetSendList()
		if sendlist.Len() > 0 {
			sleep = false
			for e := sendlist.Front(); e != nil; e = e.Next() {
				f := e.Value.(*frame.Frame)
				mb, err := c.listenersonny.fm.MarshalFrame(f)
				if err != nil {
					loggo.Error("MarshalFrame fail %s", err)
					break
				}
				c.listenersonny.fatherconn.SetReadDeadline(time.Now().Add(time.Millisecond * 100))
				c.listenersonny.fatherconn.WriteToUDP(mb, c.listenersonny.dstaddr)
			}
		}

		// timeout
		if c.listenersonny.fm.IsHBTimeout(c.config.HBTimeoutms) {
			loggo.Debug("close inactive conn %s", c.Info())
			break
		}

		if c.listenersonny.fm.IsRemoteClosed() {
			loggo.Debug("closed by remote conn %s", c.Info())
			break
		}

		if sleep {
			time.Sleep(time.Millisecond * 10)
		}
	}

	c.listenersonny.fm.Close()

	startCloseTime := time.Now()
	for {
		now := time.Now()

		c.listenersonny.fm.Update()

		// send udp
		sendlist := c.listenersonny.fm.GetSendList()
		for e := sendlist.Front(); e != nil; e = e.Next() {
			f := e.Value.(*frame.Frame)
			mb, err := c.listenersonny.fm.MarshalFrame(f)
			if err != nil {
				loggo.Error("MarshalFrame fail %s", err)
				break
			}
			c.listenersonny.fatherconn.SetReadDeadline(time.Now().Add(time.Millisecond * 10))
			c.listenersonny.fatherconn.WriteToUDP(mb, c.listenersonny.dstaddr)
		}

		diffclose := now.Sub(startCloseTime)
		if diffclose > time.Millisecond*time.Duration(c.config.CloseTimeoutMs) {
			loggo.Debug("close conn had timeout %s", c.Info())
			break
		}

		remoteclosed := c.listenersonny.fm.IsRemoteClosed()
		if remoteclosed {
			loggo.Debug("remote conn had closed %s", c.Info())
			break
		}

		time.Sleep(time.Millisecond * 10)
	}

	loggo.Debug("close rudp conn %s", c.Info())

	return errors.New("closed")
}
