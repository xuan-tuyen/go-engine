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
	MaxPacketSize    int
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
		MaxPacketSize:    10240,
		CutSize:          500,
		MaxId:            1000000,
		BufferSize:       10240,
		MaxWin:           10000,
		ResendTimems:     200,
		Compress:         0,
		Stat:             0,
		HBTimeoutms:      3000,
		ConnectTimeoutMs: 10000,
		CloseTimeoutMs:   60000,
	}
}

type rudpConn struct {
	info          string
	config        *RudpConfig
	dialer        *rudpConnDialer
	listenersonny *rudpConnListenerSonny
	listener      *rudpConnListener
	cancel        context.CancelFunc
	isclose       bool
}

type rudpConnDialer struct {
	conn *net.UDPConn
	fm   *frame.FrameMgr
	wg   *group.Group
}

type rudpConnListenerSonny struct {
	dstaddr    *net.UDPAddr
	fatherconn *net.UDPConn
	fm         *frame.FrameMgr
	wg         *group.Group
}

type rudpConnListener struct {
	listenerconn *net.UDPConn
	wg           *group.Group
	sonny        sync.Map
	accept       *common.Channel
}

func (c *rudpConn) Name() string {
	return "rudp"
}

func (c *rudpConn) Read(p []byte) (n int, err error) {
	c.checkConfig()

	if c.isclose {
		return 0, errors.New("read closed conn")
	}

	var fm *frame.FrameMgr
	if c.dialer != nil {
		fm = c.dialer.fm
	} else if c.listener != nil {
		return 0, errors.New("listener can not be read")
	} else if c.listenersonny != nil {
		fm = c.listenersonny.fm
	} else {
		return 0, errors.New("empty conn")
	}

	for !c.isclose {
		if fm.GetRecvBufferSize() <= 0 {
			time.Sleep(time.Millisecond * 10)
			continue
		}

		size := copy(p, fm.GetRecvReadLineBuffer())
		fm.SkipRecvBuffer(size)
		return size, nil
	}

	return 0, errors.New("read closed conn")
}

func (c *rudpConn) Write(p []byte) (n int, err error) {
	c.checkConfig()

	if c.isclose {
		return 0, errors.New("write closed conn")
	}

	var fm *frame.FrameMgr
	if c.dialer != nil {
		fm = c.dialer.fm
	} else if c.listener != nil {
		return 0, errors.New("listener can not be write")
	} else if c.listenersonny != nil {
		fm = c.listenersonny.fm
	} else {
		return 0, errors.New("empty conn")
	}

	for !c.isclose {
		size := len(p)
		if size > fm.GetSendBufferLeft() {
			size = fm.GetSendBufferLeft()
		}

		if size <= 0 {
			time.Sleep(time.Millisecond * 10)
			continue
		}

		fm.WriteSendBuffer(p[0:size])
		return size, nil
	}

	return 0, errors.New("write closed conn")
}

func (c *rudpConn) Close() error {
	c.checkConfig()

	if c.isclose {
		return nil
	}

	loggo.Debug("start Close %s", c.Info())

	if c.cancel != nil {
		c.cancel()
	}
	if c.dialer != nil {
		if c.dialer.wg != nil {
			loggo.Debug("start Close dialer %s", c.Info())
			c.dialer.wg.Stop()
			c.dialer.wg.Wait()
		}
	} else if c.listener != nil {
		if c.listener.wg != nil {
			loggo.Debug("start Close listener %s", c.Info())
			c.listener.wg.Stop()
			c.listener.sonny.Range(func(key, value interface{}) bool {
				u := value.(*rudpConn)
				u.Close()
				return true
			})
			c.listener.wg.Wait()
		}
	} else if c.listenersonny != nil {
		if c.listenersonny.wg != nil {
			loggo.Debug("start Close listenersonny %s", c.Info())
			c.listenersonny.wg.Stop()
			c.listenersonny.wg.Wait()
		}
	}
	c.isclose = true

	loggo.Debug("Close ok %s", c.Info())

	return nil
}

func (c *rudpConn) Info() string {
	c.checkConfig()

	if c.info != "" {
		return c.info
	}
	if c.dialer != nil {
		c.info = c.dialer.conn.LocalAddr().String() + "<--rudp-->" + c.dialer.conn.RemoteAddr().String()
	} else if c.listener != nil {
		c.info = "rudp--" + c.listener.listenerconn.LocalAddr().String()
	} else if c.listenersonny != nil {
		c.info = c.listenersonny.fatherconn.LocalAddr().String() + "<--rudp-->" + c.listenersonny.dstaddr.String()
	} else {
		c.info = "empty rudp conn"
	}
	return c.info
}

func (c *rudpConn) Dial(dst string) (Conn, error) {
	c.checkConfig()

	addr, err := net.ResolveUDPAddr("udp", dst)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	c.cancel = cancel
	var d net.Dialer
	conn, err := d.DialContext(ctx, "udp", addr.String())
	if err != nil {
		return nil, err
	}
	c.cancel = nil

	id := common.Guid()
	fm := frame.NewFrameMgr(c.config.CutSize, c.config.MaxId, c.config.BufferSize, c.config.MaxWin, c.config.ResendTimems, c.config.Compress, c.config.Stat)
	fm.SetDebugid(id)

	dialer := &rudpConnDialer{conn: conn.(*net.UDPConn), fm: fm}

	u := &rudpConn{config: c.config, dialer: dialer}

	loggo.Debug("start connect remote rudp %s", u.Info())

	u.dialer.fm.Connect()

	startConnectTime := time.Now()
	buf := make([]byte, c.config.MaxPacketSize)
	for {
		if u.dialer.fm.IsConnected() {
			break
		}

		u.dialer.fm.Update()

		// send udp
		sendlist := u.dialer.fm.GetSendList()
		for e := sendlist.Front(); e != nil; e = e.Next() {
			f := e.Value.(*frame.Frame)
			mb, _ := u.dialer.fm.MarshalFrame(f)
			u.dialer.conn.SetReadDeadline(time.Now().Add(time.Millisecond * 100))
			u.dialer.conn.Write(mb)
		}

		// recv udp
		u.dialer.conn.SetReadDeadline(time.Now().Add(time.Millisecond * 100))
		n, _ := u.dialer.conn.Read(buf)
		if n > 0 {
			f := &frame.Frame{}
			err := proto.Unmarshal(buf[0:n], f)
			if err == nil {
				u.dialer.fm.OnRecvFrame(f)
			} else {
				loggo.Error("%s %s Unmarshal fail %s", c.Info(), u.Info(), err)
				break
			}
		}

		if c.isclose {
			loggo.Debug("can not connect remote rudp %s", u.Info())
			break
		}

		// timeout
		now := time.Now()
		diffclose := now.Sub(startConnectTime)
		if diffclose > time.Millisecond*time.Duration(c.config.ConnectTimeoutMs) {
			loggo.Debug("can not connect remote rudp %s", u.Info())
			break
		}

		time.Sleep(time.Millisecond * 10)
	}

	if c.isclose {
		u.Close()
		return nil, errors.New("closed conn")
	}

	if u.isclose {
		return nil, errors.New("closed conn")
	}

	if !u.dialer.fm.IsConnected() {
		return nil, errors.New("connect timeout")
	}

	loggo.Debug("connect remote ok rudp %s", u.Info())

	wg := group.NewGroup("rudpConn serveListenerSonny"+" "+u.Info(), nil, nil)

	u.dialer.wg = wg

	wg.Go("rudpConn updateDialerSonny"+" "+u.Info(), func() error {
		return u.updateDialerSonny()
	})

	return u, nil
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

	wg := group.NewGroup("rudpConn Listen"+" "+dst, nil, nil)

	listener := &rudpConnListener{
		listenerconn: listenerconn,
		wg:           wg,
		accept:       ch,
	}

	u := &rudpConn{config: c.config, listener: listener}
	wg.Go("rudpConn loopListenerRecv"+" "+dst, func() error {
		return u.loopListenerRecv()
	})

	return u, nil
}

func (c *rudpConn) Accept() (Conn, error) {
	c.checkConfig()

	if c.listener.wg == nil {
		return nil, errors.New("not listen")
	}
	for !c.listener.wg.IsExit() {
		s := <-c.listener.accept.Ch()
		if s == nil {
			break
		}
		sonny := s.(*rudpConn)
		_, ok := c.listener.sonny.Load(sonny.listenersonny.dstaddr.String())
		if !ok {
			continue
		}
		if sonny.isclose {
			continue
		}
		return sonny, nil
	}
	return nil, errors.New("listener close")
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
	buf := make([]byte, c.config.MaxPacketSize)
	for !c.listener.wg.IsExit() {
		c.listener.listenerconn.SetReadDeadline(time.Now().Add(time.Millisecond * 100))
		n, srcaddr, err := c.listener.listenerconn.ReadFromUDP(buf)
		if err != nil {
			continue
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
			u := value.(*rudpConn)
			if u.isclose {
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
			u.listenersonny.fatherconn.SetWriteDeadline(time.Now().Add(time.Millisecond * 100))
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

	loggo.Debug("server accept rudp ok %s", u.Info())

	c.listener.accept.Write(u)

	wg := group.NewGroup("rudpConn ListenerSonny"+" "+u.Info(), c.listener.wg, nil)

	u.listenersonny.wg = wg

	wg.Go("rudpConn updateListenerSonny"+" "+u.Info(), func() error {
		return u.updateListenerSonny()
	})

	loggo.Debug("accept rudp finish %s", u.Info())

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
				c.listenersonny.fatherconn.SetWriteDeadline(time.Now().Add(time.Millisecond * 100))
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
	for !c.listenersonny.wg.IsExit() {
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
			c.listenersonny.fatherconn.SetWriteDeadline(time.Now().Add(time.Millisecond * 10))
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

func (c *rudpConn) updateDialerSonny() error {

	loggo.Debug("start rudp conn %s", c.Info())

	bytes := make([]byte, c.config.MaxPacketSize)

	for !c.dialer.wg.IsExit() {
		sleep := true

		c.dialer.fm.Update()

		// send udp
		sendlist := c.dialer.fm.GetSendList()
		if sendlist.Len() > 0 {
			sleep = false
			for e := sendlist.Front(); e != nil; e = e.Next() {
				f := e.Value.(*frame.Frame)
				mb, err := c.dialer.fm.MarshalFrame(f)
				if err != nil {
					loggo.Error("MarshalFrame fail %s", err)
					break
				}
				c.dialer.conn.SetWriteDeadline(time.Now().Add(time.Millisecond * 100))
				c.dialer.conn.Write(mb)
			}
		}

		// recv udp
		c.dialer.conn.SetReadDeadline(time.Now().Add(time.Millisecond * 100))
		n, _ := c.dialer.conn.Read(bytes)
		if n > 0 {
			f := &frame.Frame{}
			err := proto.Unmarshal(bytes[0:n], f)
			if err == nil {
				c.dialer.fm.OnRecvFrame(f)
			} else {
				loggo.Error("Unmarshal fail from %s %s", c.Info(), err)
			}
		}

		// timeout
		if c.dialer.fm.IsHBTimeout(c.config.HBTimeoutms) {
			loggo.Debug("close inactive conn %s", c.Info())
			break
		}

		if c.dialer.fm.IsRemoteClosed() {
			loggo.Debug("closed by remote conn %s", c.Info())
			break
		}

		if sleep {
			time.Sleep(time.Millisecond * 10)
		}
	}

	c.dialer.fm.Close()

	startCloseTime := time.Now()
	for !c.dialer.wg.IsExit() {
		now := time.Now()

		c.dialer.fm.Update()

		// send udp
		sendlist := c.dialer.fm.GetSendList()
		for e := sendlist.Front(); e != nil; e = e.Next() {
			f := e.Value.(*frame.Frame)
			mb, err := c.dialer.fm.MarshalFrame(f)
			if err != nil {
				loggo.Error("MarshalFrame fail %s", err)
				break
			}
			c.dialer.conn.SetWriteDeadline(time.Now().Add(time.Millisecond * 10))
			c.dialer.conn.Write(mb)
		}

		// recv udp
		c.dialer.conn.SetReadDeadline(time.Now().Add(time.Millisecond * 10))
		n, _ := c.dialer.conn.Read(bytes)
		if n > 0 {
			f := &frame.Frame{}
			err := proto.Unmarshal(bytes[0:n], f)
			if err == nil {
				c.dialer.fm.OnRecvFrame(f)
			} else {
				loggo.Error("Unmarshal fail from %s", c.Info())
			}
		}

		diffclose := now.Sub(startCloseTime)
		if diffclose > time.Millisecond*time.Duration(c.config.CloseTimeoutMs) {
			loggo.Info("close conn had timeout %s", c.Info())
			break
		}

		remoteclosed := c.dialer.fm.IsRemoteClosed()
		if remoteclosed {
			loggo.Info("remote conn had closed %s", c.Info())
			break
		}

		time.Sleep(time.Millisecond * 10)
	}

	loggo.Info("close rudp conn %s", c.Info())

	return errors.New("closed")
}
