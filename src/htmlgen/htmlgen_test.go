package htmlgen

import (
	"testing"
)

func Test0001(t *testing.T) {
	hg := New("test", "./", 10, 10, "./mainpage.tpl")
	hg.AddHtml("aa")
	hg.AddHtml("bb")
	//hg.AddHtml("aaa")
	//hg.AddHtml("啊啊")
	//hg.AddHtml("3阿斯发a")
	//hg.AddHtml("asfa")
}
