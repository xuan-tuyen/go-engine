package evilnet

import (
	"github.com/esrrhs/go-engine/src/common"
	"math"
)

const (
	MSG_MAX_SIZE         int    = math.MaxInt16
	CONN_MSG_BUFFER_SIZE int    = 100 * 1024
	CONN_MSG_LIST_SIZE   int    = 100
	MSG_RC4_KEY          string = "deadbeef"
)

func encrypt(data []byte) (bool, []byte) {
	newdata, err := common.Rc4(MSG_RC4_KEY, data)
	if err != nil {
		return false, nil
	}
	return true, newdata
}

func decrypt(data []byte) (bool, []byte) {
	newdata, err := common.Rc4(MSG_RC4_KEY, data)
	if err != nil {
		return false, nil
	}
	return true, newdata
}
