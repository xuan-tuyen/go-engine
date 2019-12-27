package common

import (
	"crypto/md5"
	"encoding/hex"
	"strconv"
)

const gcharset = "abcdefghijklmnopqrstuvwxyz" +
	"ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

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

func GetMd5String(s string) string {
	h := md5.New()
	h.Write([]byte(s))
	return hex.EncodeToString(h.Sum(nil))
}

func RandStr(l int) string {
	b := make([]byte, l)
	for i := range b {
		b[i] = gcharset[RandInt31n(len(gcharset))]
	}
	return string(b)
}
