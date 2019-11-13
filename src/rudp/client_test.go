package rudp

import (
	"fmt"
	"testing"
	"time"
)

func Test0001(t *testing.T) {

	lis, err := Listen("127.0.0.1:9999", nil)
	if err != nil {
		fmt.Println(err)
		return
	}

	conn, err := Dail("127.0.0.1:9999", nil)
	if err != nil {
		fmt.Println(err)
		return
	}

	c := lis.Accept(100)
	fmt.Println(c)

	c.Write([]byte("123123123123"))
	time.Sleep(time.Second)

	tmp := make([]byte, 19)
	n, err := conn.Read(tmp)
	fmt.Println(n)
	fmt.Println(string(tmp))

	conn.Close(false)
	lis.Close(false)
	fmt.Println("done ")

}
