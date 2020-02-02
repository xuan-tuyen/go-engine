package htmlgen

import (
	"container/list"
	"github.com/esrrhs/go-engine/src/common"
	"github.com/esrrhs/go-engine/src/loggo"
	"html/template"
	"os"
	"time"
)

type HtmlGen struct {
	name       string
	path       string
	lastest    list.List
	lastestmax int
	lastesttpl string
}

func New(name string, path string, maxlastest int, maxday int, mainpagetpl string) *HtmlGen {
	loggo.Info("Ini start %s", path)
	os.MkdirAll(path, os.ModePerm)
	hg := &HtmlGen{}
	hg.name = name
	hg.path = path
	hg.lastestmax = maxlastest

	if len(mainpagetpl) > 0 {
		hg.lastesttpl = mainpagetpl
	} else {
		hg.lastesttpl = common.GetSrcDir() + "/htmlgen/" + "mainpage.tpl"
		if _, err := os.Stat(hg.lastesttpl); os.IsNotExist(err) {
			panic("no main page tpl at " + hg.lastesttpl)
		}
	}

	return hg
}

func (hg *HtmlGen) AddHtml(html string) error {
	b := time.Now()
	now := time.Now()
	hg.addLatest(html)
	err := hg.saveLatest(now)
	if err != nil {
		return err
	}
	loggo.Info("AddHtml %s %s ", html, time.Now().Sub(b).String())
	return nil
}

func (hg *HtmlGen) addLatest(s string) {
	hg.lastest.PushFront(s)
	if hg.lastest.Len() > hg.lastestmax {
		var last *list.Element
		for e := hg.lastest.Front(); e != nil; e = e.Next() {
			last = e
		}
		if last != nil {
			hg.lastest.Remove(last)
		}
	}
}

type mainpageLastest struct {
	Name string
}

type mainpage struct {
	Name    string
	Lastest []mainpageLastest
}

func noescape(str string) template.HTML {
	return template.HTML(str)
}

func (hg *HtmlGen) savefile(data interface{}, des string, src string) error {

	file, err := os.Create(des)
	if err != nil {
		loggo.Error("os create %s", err)
		return err
	}
	defer file.Close()

	t := template.New("text")
	if err != nil {
		loggo.Error("template New %s", err)
		return err
	}

	t = t.Funcs(template.FuncMap{"noescape": noescape})

	srcfile, err := os.Open(src)
	if err != nil {
		loggo.Error("os Open %s", err)
		return err
	}
	defer srcfile.Close()

	var buffer [1024 * 1024]byte
	n, rerr := srcfile.Read(buffer[0:])
	if rerr != nil {
		loggo.Error("srcfile Read %s", err)
		return err
	}

	t, err = t.Parse(string(buffer[0:n]))
	if err != nil {
		loggo.Error("template Parse %s", err)
		return err
	}

	err = t.Execute(file, data)
	if err != nil {
		loggo.Error("template Execute %s", err)
		return err
	}

	return nil
}

func (hg *HtmlGen) saveLatest(now time.Time) error {
	mp := &mainpage{}
	mp.Name = hg.name
	for e := hg.lastest.Front(); e != nil; e = e.Next() {
		t := mainpageLastest{}
		t.Name = e.Value.(string)
		mp.Lastest = append(mp.Lastest, t)
	}

	des := hg.path + "/" + "htmlgen.html"

	src := hg.lastesttpl

	return hg.savefile(mp, des, src)
}
