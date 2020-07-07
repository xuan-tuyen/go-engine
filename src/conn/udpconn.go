package conn

import (
	"context"
	"errors"
	"github.com/esrrhs/go-engine/src/common"
	"github.com/esrrhs/go-engine/src/group"
	"github.com/esrrhs/go-engine/src/loggo"
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
		if c.listenersonny.isclose {
			return 0, errors.New("read closed conn")
		}
		b := <-c.listenersonny.recvch.Ch()
		if b == nil {
			return 0, errors.New("read closed conn")
		}
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
		if c.listenersonny.isclose {
			return 0, errors.New("write closed conn")
		}
		return c.listenersonny.fatherconn.WriteToUDP(p, c.listenersonny.dstaddr)
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
		c.listener.wg.Stop()
		c.listener.wg.Wait()
		c.listener.sonny.Range(func(key, value interface{}) bool {
			u := value.(*udpConn)
			u.Close()
			return true
		})
	} else if c.listenersonny != nil {
		c.listenersonny.recvch.Close()
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

	ipaddr, err := net.ResolveUDPAddr("udp", dst)
	if err != nil {
		return nil, err
	}

	listenerconn, err := net.ListenUDP("udp", ipaddr)
	if err != nil {
		return nil, err
	}

	ch := common.NewChannel(UDP_ACCEPT_CHAN_LEN)

	wg := group.NewGroup("udpConn Listen"+" "+dst, nil, func() {
		listenerconn.Close()
		ch.Close()
	})

	listener := &udpConnListener{
		listenerconn: listenerconn,
		wg:           wg,
		accept:       ch,
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
		if sonny.listenersonny.isclose {
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
			if !u.listenersonny.recvch.WriteTimeout(data, UDP_RECV_CHAN_PUSH_TIMEOUT) {
				loggo.Debug("udp conn %s push %d data to %s recv channel timeout", c.Info(), len(data), u.Info())
			}
			c.listener.sonny.Store(srcaddrstr, u)

			if !c.listener.accept.WriteTimeout(u, UDP_RECV_CHAN_PUSH_TIMEOUT) {
				loggo.Debug("udp conn %s push %s to accept channel timeout", c.Info(), u.Info())
			}
		} else {
			u := v.(*udpConn)
			if !u.listenersonny.recvch.WriteTimeout(data, UDP_RECV_CHAN_PUSH_TIMEOUT) {
				loggo.Debug("udp conn %s push %d data to %s recv channel timeout", c.Info(), len(data), u.Info())
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
