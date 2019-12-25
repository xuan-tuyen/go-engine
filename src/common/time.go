package common

import (
	"sync"
	"time"
)

var timeinited bool
var gnowsecond time.Time
var gtimelock sync.Mutex

func GetNowUpdateInSecond() time.Time {
	checkTimeInit()
	return gnowsecond
}

func checkTimeInit() {
	if !timeinited {
		defer gtimelock.Unlock()
		gtimelock.Lock()
		if !timeinited {
			timeinited = true
			gnowsecond = time.Now()
			timeInit()
		}
	}
}

func timeInit() {
	go updateNowInSecond()
}

func updateNowInSecond() {
	defer CrashLog()

	for {
		gnowsecond = time.Now()
		time.Sleep(time.Second)
	}
}
