package spider

import (
	"fmt"
	"github.com/go-sql-driver/mysql"
	"testing"
)

func Test0001(t *testing.T) {

	dbconfig := mysql.NewConfig()
	dbconfig.User = "root"
	dbconfig.Passwd = "123123"
	dbconfig.Addr = "192.168.0.106:4406"
	dbconfig.Net = "tcp"

	f := LoadJob(dbconfig.FormatDSN(), 10, "http://www.baidu.com")
	if f == nil {
		return
	}
	InsertSpiderJob(f, "aaa", 1)
	InsertSpiderJob(f, "aaaa", 1)
	InsertSpiderJob(f, "aaaaa", 1)
	fmt.Println(HasJob(f, "aaa"))
	fmt.Println(HasJob(f, "aaba"))
	u, d := PopSpiderJob(f, 1)
	fmt.Println(u)
	fmt.Println(d)
}
