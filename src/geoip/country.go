package geoip

import (
	"github.com/esrrhs/go-engine/src/common"
	"github.com/oschwald/geoip2-golang"
	"net"
)

var gdb *geoip2.Reader

func Load(file string) {

	if len(file) <= 0 {
		file = common.GetDataDir() + "/geoip/" + "GeoLite2-City.mmdb"
	}

	db, err := geoip2.Open(file)
	if err != nil {
		panic(err)
	}
	gdb = db
}

func GetCountryIsoCode(ipaddr string) string {

	ip := net.ParseIP(ipaddr)
	record, err := gdb.City(ip)
	if err != nil {
		return ""
	}

	return record.Country.IsoCode
}

func GetCountryName(ipaddr string) string {

	ip := net.ParseIP(ipaddr)
	record, err := gdb.City(ip)
	if err != nil {
		return ""
	}

	return record.Country.Names["en"]
}
