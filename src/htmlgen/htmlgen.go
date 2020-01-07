package htmlgen

import (
	"bufio"
	"container/list"
	"encoding/hex"
	"github.com/esrrhs/go-engine/src/common"
	"github.com/esrrhs/go-engine/src/loggo"
	"html/template"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

type HtmlGen struct {
	name       string
	path       string
	lastest    list.List
	lastestmax int
	lastesttpl string
	maxday     int
	subpagetpl string
	cur        []string
	lastday    time.Time
	lastsub    time.Time
	lock       sync.Mutex
}

func New(name string, path string, maxlastest int, maxday int, mainpagetpl string, subpagetpl string) *HtmlGen {
	loggo.Info("Ini start %s", path)
	os.MkdirAll(path, os.ModePerm)
	os.MkdirAll(path+"/htmlgen/", os.ModePerm)
	hg := &HtmlGen{}
	hg.name = name
	hg.path = path
	hg.lastestmax = maxlastest
	hg.maxday = maxday
	hg.lastday = time.Now()

	if len(mainpagetpl) > 0 {
		hg.lastesttpl = mainpagetpl
	} else {
		hg.lastesttpl = common.GetSrcDir() + "/htmlgen/" + "mainpage.tpl"
		if _, err := os.Stat(hg.lastesttpl); os.IsNotExist(err) {
			panic("no main page tpl at " + hg.lastesttpl)
		}
	}

	if len(subpagetpl) > 0 {
		hg.subpagetpl = subpagetpl
	} else {
		hg.subpagetpl = common.GetSrcDir() + "/htmlgen/" + "subpage.tpl"
		if _, err := os.Stat(hg.subpagetpl); os.IsNotExist(err) {
			panic("no main page tpl at " + hg.subpagetpl)
		}
	}

	hg.loadDB()

	hg.deleteHtml()
	go func() {
		defer common.CrashLog()
		for {
			time.Sleep(time.Hour)
			hg.deleteHtml()
		}
	}()

	return hg
}

func (hg *HtmlGen) AddHtml(html string) error {
	b := time.Now()
	now := time.Now()
	hg.addLatest(html)
	saveb := time.Now()
	mustsave := hg.save(now, html)
	savee := time.Now()
	head := hg.calcSubdir(now)
	err := hg.saveLatest(now)
	if err != nil {
		return err
	}
	err = hg.saveSub(now, head, mustsave)
	if err != nil {
		return err
	}
	loggo.Info("AddHtml %s %s %s", html, savee.Sub(saveb).String(), time.Now().Sub(b).String())
	return nil
}

func (hg *HtmlGen) insertDB(now time.Time, s string) {
	cur := now.Format("2006-01-02")

	ecoded := hex.EncodeToString([]byte(s))

	hg.lock.Lock()
	defer hg.lock.Unlock()

	file, err := os.OpenFile("htmlgendb/"+cur+".txt", os.O_WRONLY|os.O_APPEND|os.O_CREATE, os.ModePerm)
	if err != nil {
		return
	}
	defer file.Close()

	file.WriteString(ecoded + "\n")
}

func (hg *HtmlGen) loadDB() {
	cur := time.Now().Format("2006-01-02")

	hg.lock.Lock()
	defer hg.lock.Unlock()

	os.MkdirAll("htmlgendb/", os.ModePerm)

	file, err := os.Open("htmlgendb/" + cur + ".txt")
	if err != nil {
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		s := scanner.Text()
		s = strings.TrimRight(s, "\n")

		dst := make([]byte, hex.DecodedLen(len(s)))
		n, err := hex.Decode(dst, []byte(s))
		if err != nil {
			continue
		}
		ret := dst[:n]
		hg.cur = append(hg.cur, string(ret))
	}
}

func (hg *HtmlGen) clearDB() {
	hg.lock.Lock()
	defer hg.lock.Unlock()

	os.RemoveAll("htmlgendb/")
	os.MkdirAll("htmlgendb/", os.ModePerm)
}

func (hg *HtmlGen) calcSubdir(now time.Time) string {
	return now.Format("2006-01-02")
}

func (hg *HtmlGen) save(now time.Time, s string) bool {
	cur := now.Format("2006-01-02")
	last := hg.lastday.Format("2006-01-02")
	mustsave := false
	if cur != last {
		hg.cur = make([]string, 0)
		hg.lastday = now
		mustsave = true
		hg.clearDB()
	}
	hg.cur = append(hg.cur, s)
	hg.insertDB(now, s)
	return mustsave
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

type mainpageSub struct {
	Name string
}

type mainpage struct {
	Name    string
	Lastest []mainpageLastest
	Sub     []mainpageSub
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

	for i := 0; i < hg.maxday; i++ {
		tt := time.Now().Add(-24 * time.Hour * time.Duration(i))
		t := mainpageSub{}
		t.Name = tt.Format("2006-01-02")
		mp.Sub = append(mp.Sub, t)
	}

	des := hg.path + "/" + "htmlgen.html"

	src := hg.lastesttpl

	return hg.savefile(mp, des, src)
}

type subpageData struct {
	Name string
}

type subpage struct {
	Name string
	Data []subpageData
}

func (hg *HtmlGen) saveSub(now time.Time, head string, mustsave bool) error {

	if now.Sub(hg.lastsub) < time.Minute && !mustsave {
		return nil
	}
	hg.lastsub = now

	sp := &subpage{}
	sp.Name = head
	for i := len(hg.cur) - 1; i >= 0; i-- {
		t := subpageData{}
		t.Name = hg.cur[i]
		sp.Data = append(sp.Data, t)
	}

	des := hg.path + "/htmlgen" + "/" + head + ".html"

	src := hg.subpagetpl

	return hg.savefile(sp, des, src)
}

func (hg *HtmlGen) deleteHtml() {
	now := time.Now().Format("2006-01-02")
	nowt, _ := time.Parse("2006-01-02", now)
	nowunix := nowt.Unix()
	filepath.Walk(hg.path+"/htmlgen", func(path string, f os.FileInfo, err error) error {

		if f == nil || f.IsDir() {
			return nil
		}

		if !strings.HasSuffix(f.Name(), ".html") {
			return nil
		}

		date := f.Name()
		date = strings.TrimRight(date, ".html")

		t, e := time.Parse("2006-01-02", date)
		if e != nil {
			loggo.Error("delete Parse file fail %v %v %v", f.Name(), date, err)
			return nil
		}
		tunix := t.Unix()
		if nowunix-tunix > int64(hg.maxday)*24*3600 {
			err := os.Remove(hg.path + "/htmlgen" + "/" + f.Name())
			if e != nil {
				loggo.Error("delete file fail %v %v", f.Name(), err)
				return nil
			}
		}

		return nil
	})
}
