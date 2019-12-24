package threadpool

import (
	"fmt"
	"testing"
	"time"
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

func Test2(t *testing.T) {
	tp := NewThreadPool(2, 1, func(i interface{}) {
		v := i.(int)
		fmt.Println(v)
		time.Sleep(time.Second)
	})
	tp.AddJob(0, 0)
	tp.AddJob(1, 1)
	tp.AddJob(2, 2)
	tp.AddJob(3, 3)
	tp.AddJob(4, 4)
	tp.AddJob(5, 5)
	time.Sleep(time.Second * 2)
	tp.Stop()
	fmt.Println("Stop")
}
