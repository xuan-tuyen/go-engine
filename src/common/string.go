package common

import (
	"strconv"
)

func IntArrayToString(a []int, delim string) string {
	var ret string
	for _, s := range a {
		ret += strconv.Itoa(s) + delim
	}
	return ret
}

func Int32ArrayToString(a []int32, delim string) string {
	var ret string
	for _, s := range a {
		ret += strconv.Itoa((int)(s)) + delim
	}
	return ret
}

func Int64ArrayToString(a []int64, delim string) string {
	var ret string
	for _, s := range a {
		ret += strconv.Itoa((int)(s)) + delim
	}
	return ret
}
