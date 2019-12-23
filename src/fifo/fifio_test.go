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

	fmt.Println(f.Read())
	fmt.Println(f.Read())
	fmt.Println(f.Read())
	fmt.Println(f.Read())
}
