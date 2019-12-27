package fifo

import (
	"fmt"
	"github.com/esrrhs/go-engine/src/common"
	"testing"
	"time"
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

func Test0002(t *testing.T) {

	f := NewFIFO("aa")

	go func() {
		last := time.Now()
		n := 0
		for {
			f.Write(common.RandStr(10))
			n++
			if time.Now().Sub(last) > time.Second {
				fmt.Printf("write %d\n", n)
				n = 0
				last = time.Now()
			}
		}
	}()

	go func() {
		last := time.Now()
		n := 0
		for {
			_, err := f.Read()
			if err == nil {
				n++
			}
			if time.Now().Sub(last) > time.Second {
				fmt.Printf("read %d\n", n)
				n = 0
				last = time.Now()
			}
		}
	}()

	time.Sleep(10 * time.Second)
}
