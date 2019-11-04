package geoip

import (
	"fmt"
	"github.com/esrrhs/go-engine/src/common"
	"testing"
)

func TestNew(t *testing.T) {
	common.Ini()
	Load("")

	fmt.Println(GetCountryIsoCode("39.106.101.133"))
}
