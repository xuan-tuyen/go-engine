package common

import (
	"fmt"
	"strconv"
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

	fmt.Println(IsInt(3.0002))
	fmt.Println(IsInt(3))
	fmt.Println(strconv.FormatFloat(3.1415, 'E', -1, 64))

	aa := []int{1, 2, 3, 4, 5, 6, 7, 8}
	Shuffle(len(aa), func(i, j int) { aa[i], aa[j] = aa[j], aa[i] })
	fmt.Println(aa)

	fmt.Println(RandInt())
	fmt.Println(RandInt31n(10))

	fmt.Println(WrapString("abc", 10))

	ts := StrTable{}
	ts.AddHeader("a")
	ts.AddHeader("b")
	ts.AddHeader("c")
	tsl := StrTableLine{}
	tsl.AddData("1234")
	tsl.AddData("123421412")
	ts.AddLine(tsl)
	tsl = StrTableLine{}
	tsl.AddData("aaa")
	ts.AddLine(tsl)
	fmt.Println(WrapString("abc", 10))
	fmt.Println(ts.String())
}
