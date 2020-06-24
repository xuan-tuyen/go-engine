package conn

import (
	"context"
	"errors"
	"github.com/esrrhs/go-engine/src/common"
	"github.com/esrrhs/go-engine/src/group"
	"net"
	"sync"
)

type udpConn struct {
	info          string
	dialer        *udpConnDialer
	listenersonny *udpConnListenerSonny
	listener      *udpConnListener
	cancel        context.CancelFunc
}

type udpConnDialer struct {
	conn *net.UDPConn
}

type udpConnListenerSonny struct {
	dstaddr    *net.UDPAddr
	fatherconn *net.UDPConn
	recvch     *common.Channel
	isclose    bool
}

type udpConnListener struct {
	listenerconn *net.UDPConn
	wg           *group.Group
	sonny        sync.Map
	accept       *common.Channel
}

const (
	MAX_UDP_PACKET             = 10240
	UDP_RECV_CHAN_LEN          = 128
	UDP_ACCEPT_CHAN_LEN        = 128
	UDP_RECV_CHAN_PUSH_TIMEOUT = 100
)

func (c *udpConn) Name() string {
	return "udp"
}

func (c *udpConn) Read(p []byte) (n int, err error) {
	if c.dialer != nil {
		return c.dialer.conn.Read(p)
	} else if c.listener != nil {
		return 0, errors.New("listener can not be read")
	} else if c.listenersonny != nil {
		b := <-c.listenersonny.recvch.Ch()
		data := b.([]byte)
		if len(data) > len(p) {
			return 0, errors.New("read buffer too small")
		}
		copy(p, data)
		return len(data), nil
	}
	return 0, errors.New("empty conn")
}

func (c *udpConn) Write(p []byte) (n int, err error) {
	if c.dialer != nil {
		return c.dialer.conn.Write(p)
	} else if c.listener != nil {
		return 0, errors.New("listener can not be write")
	} else if c.listenersonny != nil {
		c.listenersonny.fatherconn.WriteToUDP(p, c.listenersonny.dstaddr)
	}
	return 0, errors.New("empty conn")
}

func (c *udpConn) Close() error {
	if c.cancel != nil {
		c.cancel()
	}
	if c.dialer != nil {
		return c.dialer.conn.Close()
	} else if c.listener != nil {
		return c.listener.listenerconn.Close()
	} else if c.listenersonny != nil {
		c.listenersonny.isclose = true
	}
	return nil
}

func (c *udpConn) Info() string {
	if c.info != "" {
		return c.info
	}
	if c.dialer != nil {
		c.info = c.dialer.conn.LocalAddr().String() + "<--udp-->" + c.dialer.conn.RemoteAddr().String()
	} else if c.listener != nil {
		c.info = "udp--" + c.listener.listenerconn.LocalAddr().String()
	} else if c.listenersonny != nil {
		c.info = c.listenersonny.fatherconn.LocalAddr().String() + "<--udp-->" + c.listenersonny.dstaddr.String()
	} else {
		c.info = "empty udp conn"
	}
	return c.info
}

func (c *udpConn) Dial(dst string) (Conn, error) {
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
	dialer := &udpConnDialer{conn: conn.(*net.UDPConn)}
	return &udpConn{dialer: dialer}, nil
}

func (c *udpConn) Listen(dst string) (Conn, error) {
	listenerconn, err := net.ListenPacket("udp", dst)
	if err != nil {
		return nil, err
	}

	wg := group.NewGroup("udpConn Listen"+" "+dst, nil, func() {
		listenerconn.Close()
	})

	listener := &udpConnListener{
		listenerconn: listenerconn.(*net.UDPConn),
		wg:           wg,
		accept:       common.NewChannel(UDP_ACCEPT_CHAN_LEN),
	}

	u := &udpConn{listener: listener}
	wg.Go("udpConn Listen loopRecv"+" "+dst, func() error {
		return u.loopRecv()
	})

	return u, nil
}

func (c *udpConn) Accept() (Conn, error) {
	if c.listener.wg == nil {
		return nil, errors.New("not listen")
	}
	for !c.listener.wg.IsExit() {
		s := <-c.listener.accept.Ch()
		if s == nil {
			break
		}
		sonny := s.(*udpConn)
		_, ok := c.listener.sonny.Load(sonny.listenersonny.dstaddr.String())
		if !ok {
			continue
		}
		return sonny, nil
	}
	return nil, errors.New("listener close")
}

func (c *udpConn) loopRecv() error {
	buf := make([]byte, MAX_UDP_PACKET)
	for !c.listener.wg.IsExit() {
		n, srcaddr, err := c.listener.listenerconn.ReadFromUDP(buf)
		if err != nil {
			return err
		}

		data := make([]byte, n)
		copy(data, buf[0:n])
		srcaddrstr := srcaddr.String()

		v, ok := c.listener.sonny.Load(srcaddrstr)
		if !ok {
			sonny := &udpConnListenerSonny{
				dstaddr:    srcaddr,
				fatherconn: c.listener.listenerconn,
				recvch:     common.NewChannel(UDP_RECV_CHAN_LEN),
			}

			u := &udpConn{listenersonny: sonny}
			u.listenersonny.recvch.WriteTimeout(data, UDP_RECV_CHAN_PUSH_TIMEOUT)
			c.listener.sonny.Store(srcaddrstr, u)

			c.listener.accept.WriteTimeout(sonny, UDP_RECV_CHAN_PUSH_TIMEOUT)
		} else {
			u := v.(*udpConn)
			u.listenersonny.recvch.WriteTimeout(data, UDP_RECV_CHAN_PUSH_TIMEOUT)
		}
	}
	return nil
}
