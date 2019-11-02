package threadpool

import (
	"fmt"
	"testing"
)

func TestNew(t *testing.T) {
	tp := NewThreadPool(100, 100, func(i interface{}) {
		v := i.(int)
		fmt.Println(v)
	})
	tp.AddJob(1, 1)
	tp.AddJob(2, 2)
	tp.AddJob(101, 101)
	tp.AddJob(3, 3)
	tp.AddJob(4, 4)
	tp.AddJob(201, 201)
	tp.Stop()
	fmt.Println("Stop")
}
