package fifo

import (
	"bufio"
	"encoding/hex"
	"encoding/json"
	"github.com/esrrhs/go-engine/src/common"
	_ "github.com/mattn/go-sqlite3"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type FiFo struct {
	name       string
	bf         *os.File
	ef         *os.File
	bd         *beginData
	ed         *endData
	folderPath string
}

func NewFIFO(name string) (*FiFo, error) {
	f := &FiFo{name: name}

	folderPath := "fifo_" + name
	f.folderPath = folderPath

	os.Mkdir(folderPath, os.ModePerm)

	bd, err := read_begin(folderPath)
	if err != nil {
		bd = ini_begin(folderPath)
		err = write_begin(bd, folderPath)
		if err != nil {
			return nil, err
		}
	}

	ed, err := read_end(folderPath)
	if err != nil {
		ed = ini_end(folderPath)
		err = write_end(ed, folderPath)
		if err != nil {
			return nil, err
		}
	}

	f.bd = bd
	f.ed = ed

	go f.loopCheck()

	return f, nil
}

func (f *FiFo) Write(data string) error {
	f.checknext_end()

	if f.ef == nil {
		file, err := os.OpenFile(f.folderPath+"/"+f.ed.File,
			os.O_WRONLY|os.O_CREATE, 0666)
		if err != nil {
			return err
		}
		f.ef = file
	}

	// Whence is the point of reference for offset
	// 0 = Beginning of file
	// 1 = Current position
	// 2 = End of file
	var whence int = 0
	_, err := f.ef.Seek(f.ed.Pos, whence)
	if err != nil {
		return err
	}

	ecoded := hex.EncodeToString([]byte(data))

	_, err = f.ef.WriteString(ecoded + "\n")
	if err != nil {
		return err
	}

	curPosition := f.ed.Pos + int64(len(ecoded)) + 1

	f.ed.Pos = curPosition
	f.ed.Index++
	err = write_end(f.ed, f.folderPath)
	if err != nil {
		f.ed.Index--
		return err
	}

	return nil
}

func (f *FiFo) Read() (string, error) {
	ret, err := f.read()
	if err == io.EOF {
		ret, err = f.read()
	}
	return ret, err
}

func (f *FiFo) read() (string, error) {
	if f.bf == nil {
		file, err := os.Open(f.folderPath + "/" + f.bd.File)
		if err != nil {
			return "", err
		}
		f.bf = file
	}

	// Whence is the point of reference for offset
	// 0 = Beginning of file
	// 1 = Current position
	// 2 = End of file
	var whence int = 0
	_, err := f.bf.Seek(f.bd.Pos, whence)
	if err != nil {
		return "", err
	}

	reader := bufio.NewReader(f.bf)
	l, err := reader.ReadString('\n')
	if err != nil {
		f.checknext_begin()
		return "", err
	}
	curPosition := f.bd.Pos + int64(len(l))

	l = strings.TrimRight(l, "\n")

	// If we're just at the EOF, break
	if err != nil {
		f.checknext_begin()
		return "", err
	}

	dst := make([]byte, hex.DecodedLen(len(l)))
	n, err := hex.Decode(dst, []byte(l))
	if err != nil {
		return "", err
	}
	ret := dst[:n]

	f.bd.Pos = curPosition
	f.bd.Index++
	err = write_begin(f.bd, f.folderPath)
	if err != nil {
		f.bd.Index--
		return "", err
	}

	return string(ret), err
}

func (f *FiFo) GetSize() int {
	if f.ed.Index-f.bd.Index >= 0 {
		return int(f.ed.Index - f.bd.Index)
	}
	return int(math.MaxInt64 + f.ed.Index - f.bd.Index)
}

type beginData struct {
	File  string
	Pos   int64
	Index int64
}

func (f *FiFo) checknext_begin() error {
	date := strings.TrimRight(f.bd.File, ".fifo")
	t, err := time.Parse("2006-01-02", date)
	if err != nil {
		return err
	}
	nt := t.Add(time.Hour * 24)
	nts := nt.Format("2006-01-02")
	file, err := os.Open(f.folderPath + "/" + nts + ".fifo")
	if err != nil {
		return err
	}

	f.bd.File = nts + ".fifo"
	f.bd.Pos = 0
	if f.bf != nil {
		f.bf.Close()
	}
	f.bf = file
	return nil
}

func ini_begin(folderPath string) *beginData {
	cur := time.Now().Format("2006-01-02")
	return &beginData{
		File: cur + ".fifo",
		Pos:  0,
	}
}

func read_begin(folderPath string) (*beginData, error) {

	fileName := folderPath + "/" + "begin"
	file, err := os.Open(fileName)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	d := beginData{}
	err = decoder.Decode(&d)
	if err != nil {
		return nil, err
	}
	return &d, nil
}

func write_begin(d *beginData, folderPath string) error {

	str, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return err
	}

	fileName := folderPath + "/" + "begin"
	jsonFile, err := os.OpenFile(fileName,
		os.O_WRONLY|os.O_CREATE, 0666)
	if err != nil {
		return err
	}
	defer jsonFile.Close()

	_, err = jsonFile.Write(str)
	if err != nil {
		return err
	}
	return nil
}

type endData struct {
	File  string
	Pos   int64
	Index int64
}

func (f *FiFo) checknext_end() {
	cur := time.Now().Format("2006-01-02")
	if cur+".fifo" == f.ed.File {
		return
	}
	f.ed.File = cur + ".fifo"
	f.ed.Pos = 0
	if f.ef != nil {
		f.ef.Close()
	}
	f.ef = nil
}

func ini_end(folderPath string) *endData {

	max := "2006-01-02"

	filepath.Walk(folderPath, func(path string, f os.FileInfo, err error) error {

		if f == nil || f.IsDir() {
			return nil
		}

		if !strings.HasSuffix(f.Name(), ".fifo") {
			return nil
		}

		date := f.Name()
		date = strings.TrimRight(date, ".fifo")

		t, e := time.Parse("2006-01-02", date)
		if e != nil {
			return nil
		}
		tunix := t.Unix()

		mt, e := time.Parse("2006-01-02", max)
		if e != nil {
			return nil
		}
		mtunix := mt.Unix()
		if mtunix < tunix {
			mtunix = tunix
		}

		return nil
	})

	if max == "2006-01-02" {
		max = time.Now().Format("2006-01-02")
	}

	pos := int64(0)
	file, err := os.Open(max + ".fifo")
	if err == nil {
		fi, err := file.Stat()
		if err == nil {
			pos = fi.Size()
		}
		file.Close()
	}

	return &endData{
		File: max + ".fifo",
		Pos:  pos,
	}
}
func read_end(folderPath string) (*endData, error) {

	fileName := folderPath + "/" + "end"
	file, err := os.Open(fileName)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	d := endData{}
	err = decoder.Decode(&d)
	if err != nil {
		return nil, err
	}
	return &d, nil
}

func write_end(d *endData, folderPath string) error {

	str, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return err
	}

	fileName := folderPath + "/" + "end"
	jsonFile, err := os.OpenFile(fileName,
		os.O_WRONLY|os.O_CREATE, 0666)
	if err != nil {
		return err
	}
	defer jsonFile.Close()

	_, err = jsonFile.Write(str)
	if err != nil {
		return err
	}
	return nil
}

func (f *FiFo) loopCheck() {
	defer common.CrashLog()
	for {
		f.checkDate()
		time.Sleep(time.Minute)
	}
}

func (f *FiFo) checkDate() {
	now := strings.TrimRight(f.bd.File, ".fifo")
	nowt, _ := time.Parse("2006-01-02", now)
	nowunix := nowt.Unix()
	filepath.Walk(f.folderPath, func(path string, f os.FileInfo, err error) error {

		if f == nil || f.IsDir() {
			return nil
		}

		if !strings.HasSuffix(f.Name(), ".fifo") {
			return nil
		}

		date := f.Name()
		date = strings.TrimRight(date, ".fifo")

		t, e := time.Parse("2006-01-02", date)
		if e != nil {
			return nil
		}
		tunix := t.Unix()
		if nowunix-tunix > 24*3600 {
			os.Remove(path)
		}

		return nil
	})
}
