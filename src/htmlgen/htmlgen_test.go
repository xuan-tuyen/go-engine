package htmlgen

import (
	"github.com/esrrhs/go-engine/src/common"
	"testing"
)

func Test0001(t *testing.T) {
	common.Ini()
	hg := New("test", "./", 10, 10, "./mainpage.tpl", "./subpage.tpl")

	hg.AddHtml("aaa")
	hg.AddHtml("啊啊")
	hg.AddHtml("3阿斯发a")
	hg.AddHtml("asfa")
}
