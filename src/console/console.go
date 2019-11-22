package console

import (
	"bufio"
	"fmt"
	"github.com/esrrhs/go-engine/src/common"
	"github.com/esrrhs/go-engine/src/loggo"
	"github.com/esrrhs/go-engine/src/synclist"
	"github.com/esrrhs/go-engine/src/termcolor"
	"os"
	"sync"
	"time"
)

type Console struct {
	exit           bool
	workResultLock sync.WaitGroup
	readbuffer     chan byte
	read           *synclist.List
	write          *synclist.List
	pretext        string

	in ConsoleInput
}

func NewConsole(pretext string) *Console {
	ret := &Console{}
	ret.readbuffer = make(chan byte, 1024)
	ret.read = synclist.NewList()
	ret.write = synclist.NewList()
	ret.pretext = pretext

	err := ret.in.Init()
	if err != nil {
		loggo.Error("NewConsole fail %s", err)
		return nil
	}

	go ret.updateRead()
	go ret.run()
	return ret
}

func (cc *Console) Get() string {
	ret := cc.read.Pop()
	if ret == nil {
		return ""
	}
	return ret.(string)
}

func (cc *Console) Put(str string) {
	cc.write.Push(str)
}

func (cc *Console) Stop() {
	cc.exit = true
	cc.workResultLock.Wait()
	cc.in.Close()
}

func (cc *Console) updateRead() {
	defer common.CrashLog()

	cc.workResultLock.Add(1)
	defer cc.workResultLock.Done()

	reader := bufio.NewReader(os.Stdin)

	for !cc.exit {
		b, err := reader.ReadByte()
		if err != nil {
			time.Sleep(time.Millisecond)
			continue
		}
		cc.readbuffer <- b
	}
}

func (cc *Console) run() {
	defer common.CrashLog()

	cc.workResultLock.Add(1)
	defer cc.workResultLock.Done()

	cur := ""
	lastreturn := true
	for !cc.exit {
		read := ""
		select {
		case c := <-cc.readbuffer:
			read = string(c)
		case <-time.After(time.Duration(100) * time.Millisecond):

		}

		needreprintpre := false
		curreturn := false

		if len(read) > 0 {
			if read == "\n" {
				cc.read.Push(cur)
				cur = ""
				needreprintpre = true
				curreturn = true
			} else {
				cur = cur + read
			}
		}

		for {
			write := cc.write.Pop()
			if write != nil {
				str := write.(string)
				if !lastreturn {
					fmt.Println()
				}
				fmt.Println(termcolor.FgString(str, 180, 224, 135))
				needreprintpre = true
				curreturn = true
			} else {
				break
			}
		}

		if needreprintpre {
			fmt.Print(termcolor.FgString(cc.pretext+cur, 225, 186, 134))
		}

		lastreturn = curreturn
	}
}
