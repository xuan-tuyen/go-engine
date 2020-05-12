package chartjs

import (
	"fmt"
	"testing"
)

func Test0001(t *testing.T) {
	ld := NewLineData("test", Red, Red, false, 3)
	ld.Add("a", 1)
	ld.Add("b", 11)
	ld.Add("c", 111)
	ld.Add("d", 1111)
	fmt.Println(ld.Export())
}
