package common

import (
	"fmt"
	"testing"
)

func Test0001(t *testing.T) {
	a := RandStr(5)
	a1 := RandStr(5)
	fmt.Println(a)
	fmt.Println(a1)

	fmt.Println(GetOutboundIP())

	fmt.Println(GetNowUpdateInSecond())

	d, _ := Rc4("123456", []byte("asdgdsagdsag435t43321dsgesg"))
	fmt.Println(string(d))

	d, _ = Rc4("123456", d)
	fmt.Println(string(d))
}
