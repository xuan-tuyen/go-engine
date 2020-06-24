package conn

import (
	"fmt"
	"testing"
	"time"
)

func Test0001(t *testing.T) {
	c, err := NewConn("tcp")
	if err != nil {
		fmt.Println(err)
		return
	}

	cc, err := c.Listen(":58080")
	if err != nil {
		fmt.Println(err)
		return
	}

	go func() {
		cc.Accept()
		fmt.Println("accept done")
	}()

	time.Sleep(time.Second)

	cc.Close()

	time.Sleep(time.Second)
}

func Test0002(t *testing.T) {
	c, err := NewConn("tcp")
	if err != nil {
		fmt.Println(err)
		return
	}

	go func() {
		_, err := c.Dial("9.9.9.9:58080")
		fmt.Println(err)
	}()

	time.Sleep(time.Second)

	c.Close()

	time.Sleep(time.Second)
}

func Test0003(t *testing.T) {
	c, err := NewConn("tcp")
	if err != nil {
		fmt.Println(err)
		return
	}

	cc, err := c.Listen(":58080")
	if err != nil {
		fmt.Println(err)
		return
	}

	go func() {
		cc.Accept()
		fmt.Println("accept done")
	}()

	ccc, err := c.Dial(":58080")
	if err != nil {
		fmt.Println(err)
		return
	}

	go func() {
		buf := make([]byte, 100)
		_, err := ccc.Read(buf)
		if err != nil {
			fmt.Println(err)
			return
		}
	}()

	time.Sleep(time.Second)

	cc.Close()
	ccc.Close()

	time.Sleep(time.Second)
}
