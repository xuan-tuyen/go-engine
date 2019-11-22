package console

import (
	"bufio"
	"fmt"
	"github.com/esrrhs/go-engine/src/common"
	"github.com/esrrhs/go-engine/src/loggo"
	"github.com/esrrhs/go-engine/src/synclist"
	"github.com/esrrhs/go-engine/src/termcolor"
	"os"
	"strings"
	"sync"
	"time"
)

type Console struct {
	exit           bool
	workResultLock sync.WaitGroup
	readbuffer     chan string
	read           *synclist.List
	write          *synclist.List
	pretext        string
	in             *ConsoleInput
	eb             *EditBox
	normalinput    bool
}

func NewConsole(pretext string, normalinput bool, historyMaxLen int) *Console {
	ret := &Console{}
	ret.readbuffer = make(chan string, 16)
	ret.read = synclist.NewList()
	ret.write = synclist.NewList()
	ret.pretext = pretext

	if normalinput {
		go ret.updateRead()
		go ret.run_normal()
	} else {
		ret.in = NewConsoleInput()
		ret.eb = NewEditBox(historyMaxLen)
		err := ret.in.Init()
		if err != nil {
			loggo.Error("NewConsole fail %s", err)
			return nil
		}
		go ret.run()
	}

	return ret
}

func (cc *Console) Stop() {
	cc.exit = true
	cc.workResultLock.Wait()
	if cc.in != nil {
		cc.in.Stop()
	}
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

func (cc *Console) updateRead() {
	defer common.CrashLog()

	cc.workResultLock.Add(1)
	defer cc.workResultLock.Done()

	reader := bufio.NewReader(os.Stdin)

	for !cc.exit {
		s, err := reader.ReadString('\n')
		if err != nil {
			time.Sleep(time.Millisecond)
			continue
		}
		s = strings.TrimRight(s, "\r")
		s = strings.TrimRight(s, "\n")
		cc.readbuffer <- s
	}
}

func (cc *Console) run_normal() {
	defer common.CrashLog()

	cc.workResultLock.Add(1)
	defer cc.workResultLock.Done()

	for !cc.exit {
		isneedprintpre := false

		read := ""
		select {
		case c := <-cc.readbuffer:
			read = c
		case <-time.After(time.Duration(100) * time.Millisecond):

		}

		if len(read) > 0 {
			cc.read.Push(read)
			isneedprintpre = true
		}

		for {
			write := cc.write.Pop()
			if write != nil {
				str := write.(string)
				fmt.Println(termcolor.FgString(str, 180, 224, 135))
				isneedprintpre = true
			} else {
				break
			}
		}

		if isneedprintpre {
			fmt.Println(termcolor.FgString(cc.pretext+read, 225, 186, 134))
		}
	}
}

func (cc *Console) run() {
	defer common.CrashLog()

	cc.workResultLock.Add(1)
	defer cc.workResultLock.Done()

	for !cc.exit {
		isneedprintpre := false

		for {
			e := cc.in.PollEvent()
			if e == nil {
				break
			}
			isneedprintpre = true
			cc.eb.Input(e)
			str := cc.eb.GetEnterText()
			if len(str) > 0 {
				cc.read.Push(str)
			}
		}

		for {
			write := cc.write.Pop()
			if write != nil {
				str := write.(string)
				fmt.Println(termcolor.FgString(str, 180, 224, 135))
				isneedprintpre = true
			} else {
				break
			}
		}

		if isneedprintpre {
			fmt.Println(termcolor.FgString(cc.pretext, 225, 186, 134) + cc.eb.GetShowText(true))
		}
	}
}
