package common

import (
	"time"
)

var gnowsecond time.Time

func init() {
	gnowsecond = time.Now()
	go updateNowInSecond()
}

func GetNowUpdateInSecond() time.Time {
	return gnowsecond
}

func updateNowInSecond() {
	defer CrashLog()

	for {
		gnowsecond = time.Now()
		time.Sleep(time.Second)
	}
}
