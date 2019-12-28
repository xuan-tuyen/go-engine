package fifo

import (
	"fmt"
	"testing"
)

func Test0001(t *testing.T) {
	f, _ := NewFIFO("aa")
	f.Write("aa")
	f.Write("bb")
	f.Write("cc")

	fmt.Println(f.GetSize())

	for f.GetSize() > 0 {
		datas, _ := f.Read(10)
		for _, d := range datas {
			fmt.Println(d)
		}
	}
}
