package htmlgen

import (
	"container/list"
	"database/sql"
	"github.com/esrrhs/go-engine/src/common"
	"github.com/esrrhs/go-engine/src/loggo"
	_ "github.com/mattn/go-sqlite3"
	"html/template"
	"os"
	"path/filepath"
	"strings"
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
	db         *sql.DB
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

	db, err := sql.Open("sqlite3", "./htmlgen.db")
	if err != nil {
		panic(err)
	}

	db.Exec("CREATE TABLE  IF NOT EXISTS [gen_info](" +
		"[day] TEXT NOT NULL," +
		"[html] TEXT NOT NULL);")
	hg.db = db
	hg.loadDB()

	hg.deleteHtml()
	go func() {
		for {
			time.Sleep(time.Hour)
			hg.deleteHtml()
		}
	}()

	return hg
}

func (hg *HtmlGen) AddHtml(html string) error {
	now := time.Now()
	hg.addLatest(html)
	mustsave := hg.save(now, html)
	head := hg.calcSubdir(now)
	err := hg.saveLatest(now)
	if err != nil {
		return err
	}
	err = hg.saveSub(now, head, mustsave)
	if err != nil {
		return err
	}
	loggo.Info("AddHtml %s ", html)
	return nil
}

func (hg *HtmlGen) insertDB(now time.Time, s string) {
	cur := now.Format("2006-01-02")
	tx, err := hg.db.Begin()
	if err != nil {
		loggo.Error("Begin sqlite3 fail %v", err)
		return
	}
	stmt, err := tx.Prepare("insert into gen_info(day, html) values(?,?)")
	if err != nil {
		loggo.Error("Prepare sqlite3 fail %v", err)
		return
	}
	defer stmt.Close()
	_, err = stmt.Exec(cur, s)
	if err != nil {
		loggo.Error("insert sqlite3 fail %v", err)
	}
	tx.Commit()
}

func (hg *HtmlGen) loadDB() {
	cur := time.Now().Format("2006-01-02")
	rows, err := hg.db.Query("select html from gen_info where day='" + cur + "'")
	if err != nil {
		loggo.Error("Query sqlite3 fail %v", err)
	}
	defer rows.Close()

	for rows.Next() {

		var html string
		err = rows.Scan(&html)
		if err != nil {
			loggo.Error("Scan sqlite3 fail %v", err)
		}

		hg.cur = append(hg.cur, html)
	}

}

func (hg *HtmlGen) clearDB() {
	hg.db.Exec("delete from meta_info")
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

	if now.Sub(hg.lastsub) < time.Hour && !mustsave {
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
