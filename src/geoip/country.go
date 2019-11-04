package geoip

import (
	"github.com/oschwald/geoip2-golang"
	"net"
)

var gdb *geoip2.Reader

func Ini(file string) {

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
