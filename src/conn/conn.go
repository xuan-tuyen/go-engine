package conn

import (
	"errors"
	"io"
	"strings"
)

type Conn interface {
	io.ReadWriteCloser

	Name() string

	Info() string

	Dial(dst string) (Conn, error)

	Listen(dst string) (Conn, error)
	Accept() (Conn, error)
}

func NewConn(proto string) (Conn, error) {
	proto = strings.ToLower(proto)
	if proto == "tcp" {
		return &tcpConn{}, nil
	}
	return nil, errors.New("undefined proto " + proto)
}
