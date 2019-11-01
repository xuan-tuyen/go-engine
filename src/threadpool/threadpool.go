package threadpool

import (
	"github.com/esrrhs/go-engine/src/common"
	"sync"
	"time"
)

type ThreadPool struct {
	exit           bool
	workResultLock sync.WaitGroup
	max            int
	exef           func(interface{})
	ca             []chan interface{}
}

func NewThreadPool(max int, buffer int, exef func(interface{})) *ThreadPool {
	ca := make([]chan interface{}, max)
	for index, _ := range ca {
		ca[index] = make(chan interface{}, buffer)
	}

	tp := &ThreadPool{exit: false, max: max, exef: exef, ca: ca}

	for index, _ := range ca {
		go tp.run(index)
	}

	return tp
}

func (tp *ThreadPool) AddJob(hash int, v interface{}) {
	tp.ca[common.AbsInt(hash)%len(tp.ca)] <- v
}

func (tp *ThreadPool) Stop() {
	tp.exit = true
	tp.workResultLock.Wait()
}

func (tp *ThreadPool) run(index int) {
	tp.workResultLock.Add(1)
	defer tp.workResultLock.Done()

	interval := time.NewTicker(time.Millisecond * 100)
	for !tp.exit {
		select {
		case <-interval.C:
		case v := <-tp.ca[index]:
			tp.exef(v)
		}
	}
	interval.Stop()
}
