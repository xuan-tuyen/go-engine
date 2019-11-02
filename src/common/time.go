package common

import (
	"sync"
	"time"
)

var inited bool
var gnowsecond time.Time
var gnowlock sync.Mutex

func GetNowUpdateInSecond() time.Time {
	if !inited {
		defer gnowlock.Unlock()
		gnowlock.Lock()
		inited = true
		go updateNowInSecond()
	}
	return gnowsecond
}

func updateNowInSecond() {
	for {
		gnowsecond = time.Now()
		time.Sleep(time.Second)
	}
}
