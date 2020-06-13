package conn

import (
	"context"
	"net"
	"time"
)

type tcpConn struct {
	conn     *net.TCPConn
	listener *net.TCPListener
	cancel   context.CancelFunc
}

func (c *tcpConn) Name() string {
	return "tcp"
}

func (c *tcpConn) Read(p []byte) (n int, err error) {
	return c.conn.Read(p)
}

func (c *tcpConn) Write(p []byte) (n int, err error) {
	return c.conn.Write(p)
}

func (c *tcpConn) Close() error {
	if c.conn != nil {
		return c.conn.Close()
	} else if c.listener != nil {
		return c.listener.Close()
	}
	if c.cancel != nil {
		c.cancel()
	}
	return nil
}

func (c *tcpConn) Info() string {
	if c.conn != nil {
		return c.conn.LocalAddr().String() + "<--->" + c.conn.RemoteAddr().String()
	} else {
		return c.listener.Addr().String()
	}
}

func (c *tcpConn) Dial(dst string) (Conn, error) {
	addr, err := net.ResolveTCPAddr("tcp", dst)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	c.cancel = cancel
	var d net.Dialer
	conn, err := d.DialContext(ctx, "tcp", addr.String())
	if err != nil {
		return nil, err
	}
	c.cancel = nil
	return &tcpConn{conn: conn.(*net.TCPConn)}, nil
}

func (c *tcpConn) Listen(dst string) (Conn, error) {
	addr, err := net.ResolveTCPAddr("tcp", dst)
	if err != nil {
		return nil, err
	}
	listener, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return nil, err
	}
	return &tcpConn{listener: listener}, nil
}

func (c *tcpConn) Accept() (Conn, error) {
	c.listener.SetDeadline(time.Now().Add(time.Second))
	conn, err := c.listener.Accept()
	if err != nil {
		return nil, err
	}
	return &tcpConn{conn: conn.(*net.TCPConn)}, nil
}
