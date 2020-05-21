package conn

import "io"

type Conn interface {
	io.ReadWriteCloser

	Name() string

	LocalAddr() string
	RemoteAddr() string

	Dial(dst string) (Conn, error)

	Listen(dst string) (Conn, error)
	Accept() (Conn, error)
}
