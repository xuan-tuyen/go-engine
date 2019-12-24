package threadpool

import (
	"github.com/esrrhs/go-engine/src/common"
	"sync"
	"time"
)

type ThreadPool struct {
	workResultLock sync.WaitGroup
	max            int
	exef           func(interface{})
	ca             []chan interface{}
	control        chan int
}

func NewThreadPool(max int, buffer int, exef func(interface{})) *ThreadPool {
	ca := make([]chan interface{}, max)
	control := make(chan int, max)
	for index, _ := range ca {
		ca[index] = make(chan interface{}, buffer)
	}

	tp := &ThreadPool{max: max, exef: exef, ca: ca, control: control}

	for index, _ := range ca {
		go tp.run(index)
	}

	return tp
}

func (tp *ThreadPool) AddJob(hash int, v interface{}) {
	tp.ca[common.AbsInt(hash)%len(tp.ca)] <- v
}

func (tp *ThreadPool) AddJobTimeout(hash int, v interface{}, timeoutms int) bool {
	select {
	case tp.ca[common.AbsInt(hash)%len(tp.ca)] <- v:
		return true
	case <-time.After(time.Duration(timeoutms) * time.Millisecond):
		return false
	}
}

func (tp *ThreadPool) Stop() {
	for i := 0; i < tp.max; i++ {
		tp.control <- i
	}
	tp.workResultLock.Wait()
}

func (tp *ThreadPool) run(index int) {
	defer common.CrashLog()

	tp.workResultLock.Add(1)
	defer tp.workResultLock.Done()

	for {
		select {
		case <-tp.control:
			return
		case v := <-tp.ca[index]:
			tp.exef(v)
		}
	}
}
