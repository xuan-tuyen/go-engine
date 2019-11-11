package common

import (
	"crypto/rand"
	"encoding/base64"
	"hash/fnv"
	"io"
	mrand "math/rand"
	"sync"
	"time"
)

var mathinited bool
var gmathlock sync.Mutex
var gseededRand *mrand.Rand

func MinOfInt(vars ...int) int {
	min := vars[0]

	for _, i := range vars {
		if min > i {
			min = i
		}
	}

	return min
}

func MaxOfInt(vars ...int) int {
	max := vars[0]

	for _, i := range vars {
		if max < i {
			max = i
		}
	}

	return max
}

func MinOfInt64(vars ...int64) int64 {
	min := vars[0]

	for _, i := range vars {
		if min > i {
			min = i
		}
	}

	return min
}

func MaxOfInt64(vars ...int64) int64 {
	max := vars[0]

	for _, i := range vars {
		if max < i {
			max = i
		}
	}

	return max
}

func AbsInt(v int) int {
	if v > 0 {
		return v
	}
	return -v
}

func AbsInt32(v int32) int32 {
	if v > 0 {
		return v
	}
	return -v
}

func AbsInt64(v int64) int64 {
	if v > 0 {
		return v
	}
	return -v
}

func HashString(s string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return h.Sum32()
}

func UniqueId() string {
	b := make([]byte, 48)

	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return ""
	}
	return GetMd5String(base64.URLEncoding.EncodeToString(b))
}

func Int31n(n int) int32 {
	checkMathInit()
	ret := gseededRand.Int31n((int32)(n))
	return int32(ret)
}

func checkMathInit() {
	if !mathinited {
		defer gmathlock.Unlock()
		gmathlock.Lock()
		if !mathinited {
			mathinited = true
			mathInit()
		}
	}
}

func mathInit() {
	gseededRand = mrand.New(mrand.NewSource(time.Now().UnixNano()))
}
