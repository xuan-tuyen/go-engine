package group

import (
	"errors"
	"fmt"
	"github.com/esrrhs/go-engine/src/common"
	"github.com/esrrhs/go-engine/src/loggo"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type Group struct {
	father   *Group
	son      []*Group
	wg       int32
	errOnce  sync.Once
	err      error
	isexit   bool
	exitfunc func()
	donech   chan int
	sonname  map[string]int
	lock     sync.Mutex
}

func NewGroup(father *Group, exitfunc func()) *Group {
	g := &Group{
		father:   father,
		exitfunc: exitfunc,
	}
	g.donech = make(chan int)
	g.sonname = make(map[string]int)

	if father != nil {
		father.son = append(father.son, g)
	}

	return g
}

func (g *Group) add() {
	atomic.AddInt32(&g.wg, 1)
	if g.father != nil {
		g.father.add()
	}
}

func (g *Group) done() {
	atomic.AddInt32(&g.wg, -1)
	if g.father != nil {
		g.father.done()
	}
}

func (g *Group) exit(err error) {
	g.errOnce.Do(func() {
		g.err = err
		g.isexit = true
		if g.exitfunc != nil {
			g.exitfunc()
		}

		for _, son := range g.son {
			son.exit(err)
		}
	})
}

func (g *Group) runningmap() string {
	g.lock.Lock()
	defer g.lock.Unlock()
	ret := ""
	tmp := make(map[string]int)
	for k, v := range g.sonname {
		if v > 0 {
			tmp[k] = v
		}
	}
	ret += fmt.Sprintf("%v", tmp) + "\n"
	for _, son := range g.son {
		ret += son.runningmap()
	}
	return ret
}

func (g *Group) Done() <-chan int {
	if !g.isexit {
		return g.donech
	} else {
		tmp := make(chan int, 1)
		tmp <- 1
		return tmp
	}
}

func (g *Group) Go(name string, f func() error) {
	g.lock.Lock()
	defer g.lock.Unlock()
	g.add()
	name = strings.Replace(name, " ", "-", -1)
	g.sonname[name]++

	go func() {
		defer common.CrashLog()
		defer g.done()

		if err := f(); err != nil {
			g.exit(err)
		}

		g.lock.Lock()
		defer g.lock.Unlock()
		g.sonname[name]--
	}()
}

func (g *Group) Stop() {
	g.exit(errors.New("stop"))
}

func (g *Group) Wait() error {
	last := int64(0)
	begin := int64(0)
	for g.wg != 0 {
		if g.isexit {
			cur := time.Now().Unix()
			if last == 0 {
				last = cur
				begin = cur
			} else {
				if cur > last {
					last = cur
					loggo.Error("Group Wait too long %s %v", time.Duration((cur-begin)*int64(time.Second)).String(), g.runningmap())
				}
			}
		}
		time.Sleep(time.Millisecond * 10)
	}
	return g.err
}
