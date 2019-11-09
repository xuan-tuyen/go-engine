package rudp

import (
	"fmt"
	"testing"
)

func Test0001(t *testing.T) {

	_, err := Dail("192.168.1.1:8888", 1000, nil)
	if err != nil {
		fmt.Println(err)
	}
	fmt.Println("done ")

}
