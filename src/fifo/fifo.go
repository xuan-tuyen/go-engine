package fifo

import (
	"database/sql"
	"errors"
	"github.com/esrrhs/go-engine/src/loggo"
	_ "github.com/mattn/go-sqlite3"
	"sync"
)

type FiFo struct {
	name          string
	db            *sql.DB
	insertJobStmt *sql.Stmt
	getJobStmt    *sql.Stmt
	deleteJobStmt *sql.Stmt
	sizeDoneStmt  *sql.Stmt
	lock          sync.Mutex
}

func NewFIFO(name string) *FiFo {
	f := &FiFo{name: name}

	gdb, err := sql.Open("sqlite3", "./fifo_"+name+".db")
	if err != nil {
		loggo.Error("open sqlite3 Job fail %v", err)
		return nil
	}

	f.db = gdb

	gdb.Exec("CREATE TABLE  IF NOT EXISTS [data_info](" +
		"[id] INTEGER PRIMARY KEY AUTOINCREMENT," +
		"[data] TEXT NOT NULL);")

	stmt, err := gdb.Prepare("insert into data_info(data) values(?)")
	if err != nil {
		loggo.Error("Prepare sqlite3 fail %v", err)
		return nil
	}
	f.insertJobStmt = stmt

	stmt, err = gdb.Prepare("select id,data from data_info limit 0,1")
	if err != nil {
		loggo.Error("Prepare sqlite3 fail %v", err)
		return nil
	}
	f.getJobStmt = stmt

	stmt, err = gdb.Prepare("delete from data_info where id = ?")
	if err != nil {
		loggo.Error("Prepare sqlite3 fail %v", err)
		return nil
	}
	f.deleteJobStmt = stmt

	stmt, err = gdb.Prepare("select count(*) from data_info")
	if err != nil {
		loggo.Error("Prepare sqlite3 fail %v", err)
		return nil
	}
	f.sizeDoneStmt = stmt

	return f
}

func (f *FiFo) Write(data string) error {
	f.lock.Lock()
	defer f.lock.Unlock()
	_, err := f.insertJobStmt.Exec(data)
	if err != nil {
		loggo.Error("Write fail %v", err)
		return err
	}
	//loggo.Info("Write ok %s", data)
	return nil
}

func (f *FiFo) Read() (string, error) {
	id, data, err := f.read()
	if err != nil {
		return "", err
	}

	f.lock.Lock()
	defer f.lock.Unlock()

	_, err = f.deleteJobStmt.Exec(id)
	if err != nil {
		loggo.Error("Read delete fail %v", err)
		return "", err
	}

	//loggo.Info("Read ok %d %s", id, data)

	return data, nil
}

func (f *FiFo) read() (int, string, error) {
	f.lock.Lock()
	defer f.lock.Unlock()

	rows, err := f.getJobStmt.Query()
	if err != nil {
		loggo.Error("Read Query fail %v", err)
		return 0, "", err
	}
	defer rows.Close()

	for rows.Next() {

		var id int
		var data string
		err = rows.Scan(&id, &data)
		if err != nil {
			loggo.Error("Read Scan fail %v", err)
			return 0, "", err
		}
		return id, data, nil
	}

	return 0, "", errors.New("no data")
}

func (f *FiFo) GetSize() int {
	f.lock.Lock()
	defer f.lock.Unlock()
	var ret int
	err := f.sizeDoneStmt.QueryRow().Scan(&ret)
	if err != nil {
		loggo.Error("GetSize fail %v", err)
		return 0
	}
	return ret
}
