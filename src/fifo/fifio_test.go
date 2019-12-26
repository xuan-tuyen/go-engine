package fifo

import (
	"fmt"
	"testing"
)

func Test0001(t *testing.T) {
	f := NewFIFO("aa")
	f.Write("aa")
	f.Write("bb")
	f.Write("cc")

	fmt.Println(f.GetSize())

	for f.GetSize() > 0 {
		fmt.Println(f.Read())
	}
}
