package common

import (
	"crypto/md5"
	"encoding/hex"
	"github.com/OneOfOne/xxhash"
	"strconv"
)

func GetMd5String(s string) string {
	h := md5.New()
	h.Write([]byte(s))
	return hex.EncodeToString(h.Sum(nil))
}

func GetXXHashString(s string) string {
	h := xxhash.New64()
	h.WriteString(s)
	return strconv.FormatUint(h.Sum64(), 10)
}
