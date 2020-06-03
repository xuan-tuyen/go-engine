package group

import (
	"errors"
	"github.com/esrrhs/go-engine/src/common"
	"sync"
)

type Group struct {
	father   *Group
	son      []*Group
	wg       sync.WaitGroup
	errOnce  sync.Once
	err      error
	isexit   bool
	exitfunc func()
	donech   chan int
}

func NewGroup(father *Group, exitfunc func()) *Group {
	g := &Group{
		father:   father,
		exitfunc: exitfunc,
	}
	g.donech = make(chan int)

	if father != nil {
		father.son = append(father.son, g)
	}

	return g
}

func (g *Group) add() {
	g.wg.Add(1)
	if g.father != nil {
		g.father.add()
	}
}

func (g *Group) done() {
	g.wg.Done()
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

func (g *Group) Done() <-chan int {
	if !g.isexit {
		return g.donech
	} else {
		tmp := make(chan int, 1)
		tmp <- 1
		return tmp
	}
}

func (g *Group) Go(f func() error) {
	g.add()

	go func() {
		defer common.CrashLog()
		defer g.done()

		if err := f(); err != nil {
			g.exit(err)
		}
	}()
}

func (g *Group) Stop() {
	g.exit(errors.New("stop"))
}

func (g *Group) Wait() error {
	g.wg.Wait()
	return g.err
}
