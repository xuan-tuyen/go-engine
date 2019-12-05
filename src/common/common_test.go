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

	dd := MAKEINT64(12345, 7890)
	fmt.Println(dd)
	fmt.Println(HIINT32(dd))
	fmt.Println(LOINT32(dd))
	ddd := MAKEINT32(12345, 7890)
	fmt.Println(ddd)
	fmt.Println(HIINT16(ddd))
	fmt.Println(LOINT16(ddd))
}
