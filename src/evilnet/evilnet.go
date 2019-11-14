package evilnet

import (
	"github.com/esrrhs/go-engine/src/common"
	"github.com/esrrhs/go-engine/src/loggo"
	"github.com/esrrhs/go-engine/src/msgmgr"
	"github.com/esrrhs/go-engine/src/rudp"
	"github.com/golang/protobuf/proto"
	"strconv"
	"sync"
	"time"
)

type EvilNetConfig struct {
	Name string

	ListenSonPort    int
	ListenFatherPort int
	Fatheraddr       string

	RegFatherInterSec int

	Key        string
	FatherKey  string
	ConnectKey string

	Rudpconfig rudp.ConnConfig
}

func (evc *EvilNetConfig) Check() {
	if len(evc.Name) <= 0 {
		evc.Name = common.RandStr(6)
	}
	if len(evc.ConnectKey) <= 0 {
		evc.ConnectKey = common.RandStr(16)
	}
	if evc.RegFatherInterSec <= 0 {
		evc.RegFatherInterSec = 10
	}
	evc.Rudpconfig.Check()
}

type EvilNet struct {
	exit           bool
	config         *EvilNetConfig
	workResultLock sync.WaitGroup

	uuid    string
	localip string

	fa     *rudp.Conn
	father *rudp.Conn

	fathername string
	globalname string
	globaladdr string

	son        *rudp.Conn
	sonConnMap sync.Map
	sonid      int

	plugin map[string]Plugin
}

type EvilNetSon struct {
	conn      *rudp.Conn
	localaddr string
	sonkey    string
	name      string
}

func NewEvilNet(plugins []Plugin, config *EvilNetConfig) *EvilNet {
	if config == nil {
		config = &EvilNetConfig{}
	}
	config.Check()

	uuid := common.UniqueId()

	ip, err := common.GetOutboundIP()
	if err != nil {
		loggo.Error("get local ip fail")
		return nil
	}

	ret := &EvilNet{
		config:  config,
		uuid:    uuid,
		localip: ip.String(),
	}

	ret.plugin = make(map[string]Plugin)
	for _, p := range plugins {
		ret.plugin[p.Name()] = p
	}

	return ret
}

func (ev *EvilNet) Stop() {
	ev.exit = true
	ev.workResultLock.Wait()
}

func (ev *EvilNet) Run() error {

	ev.globalname = ev.config.Name

	if ev.config.ListenSonPort > 0 {
		addr := ev.localip + ":" + strconv.Itoa(ev.config.ListenSonPort)
		loggo.Info("start run at %s", addr)

		conn, err := rudp.Listen(addr, &ev.config.Rudpconfig)
		if err != nil {
			return err
		}
		ev.son = conn

		go ev.updateSon()
	}

	if len(ev.config.Fatheraddr) > 0 && ev.config.ListenFatherPort > 0 {

		addr := ev.localip + ":" + strconv.Itoa(ev.config.ListenFatherPort)
		loggo.Info("start run at %s", addr)

		conn, err := rudp.Listen(addr, &ev.config.Rudpconfig)
		if err != nil {
			return err
		}
		ev.fa = conn

		go ev.updateFather()
	}

	return nil
}

func (ev *EvilNet) updateFather() {

	defer common.CrashLog()

	ev.workResultLock.Add(1)
	defer ev.workResultLock.Done()

	regtime := common.GetNowUpdateInSecond()

	bytes := make([]byte, 2000)

	for !ev.exit {

		needReg := false
		now := common.GetNowUpdateInSecond()

		// connect
		if ev.father == nil || !ev.father.IsConnected() {

			loggo.Info("start connect father %s", ev.config.Fatheraddr)

			conn, err := ev.fa.Dail(ev.config.Fatheraddr)
			if err != nil {
				loggo.Error("connect father fail %s", err)
				time.Sleep(time.Second)
				continue
			}
			ev.father = conn
			ef := encrypt
			df := decrypt
			ev.father.SetUserData(msgmgr.NewMsgMgr(MSG_MAX_SIZE, CONN_MSG_BUFFER_SIZE, CONN_MSG_LIST_SIZE, &ef, &df))
			needReg = true

			loggo.Info("connect father ok %s", ev.config.Fatheraddr)
		}

		// reg inter
		if now.Sub(regtime) > time.Second*time.Duration(ev.config.RegFatherInterSec) {
			needReg = true
		}

		// reg
		if needReg {
			regtime = now
			ev.regFather()
		}

		mm := ev.father.UserData().(*msgmgr.MsgMgr)

		// send msg
		packbuffer := mm.GetPackBuffer()
		n, _ := ev.father.Write(packbuffer)
		if n > 0 {
			mm.SkipPackBuffer(n)
		}

		// recv msg
		n, _ = ev.father.Read(bytes)
		if n > 0 {
			mm.WriteUnPackBuffer(bytes[0:n])
		}

		mm.Update()

		// process
		rl := mm.RecvList()
		if rl != nil {
			for e := rl.Front(); e != nil; e = e.Next() {
				rb := e.Value.([]byte)
				enm := &EvilNetMsg{}
				err := proto.Unmarshal(rb, enm)
				if err == nil {
					ev.processFather(enm)
				}
			}
		}
	}

}

func (ev *EvilNet) updateSon() {

	defer common.CrashLog()

	ev.workResultLock.Add(1)
	defer ev.workResultLock.Done()

	for !ev.exit {
		conn := ev.son.Accept(1000)
		if conn != nil {
			go ev.updateSonConn(conn)
		}
	}
}

func (ev *EvilNet) updateSonConn(conn *rudp.Conn) {

	defer common.CrashLog()

	ev.workResultLock.Add(1)
	defer ev.workResultLock.Done()

	ef := encrypt
	df := decrypt
	conn.SetUserData(msgmgr.NewMsgMgr(MSG_MAX_SIZE, CONN_MSG_BUFFER_SIZE, CONN_MSG_LIST_SIZE, &ef, &df))

	bytes := make([]byte, 2000)

	for !ev.exit {
		if !conn.IsConnected() {
			break
		}

		mm := conn.UserData().(*msgmgr.MsgMgr)

		// send msg
		packbuffer := mm.GetPackBuffer()
		n, err := conn.Write(packbuffer)
		if err != nil {
			break
		}
		if n > 0 {
			mm.SkipPackBuffer(n)
		}

		// recv msg
		n, err = conn.Read(bytes)
		if err != nil {
			break
		}
		if n > 0 {
			mm.WriteUnPackBuffer(bytes[0:n])
		}

		mm.Update()

		// process
		rl := mm.RecvList()
		if rl != nil {
			for e := rl.Front(); e != nil; e = e.Next() {
				rb := e.Value.([]byte)
				enm := &EvilNetMsg{}
				err := proto.Unmarshal(rb, enm)
				if err == nil {
					ev.processSon(conn, enm)
				}
			}
		}
	}

	conn.Close(false)
}

func (ev *EvilNet) addSonConn(name string, conn *EvilNetSon) {
	ev.sonConnMap.Store(name, conn)
}

func (ev *EvilNet) getSonConn(name string) *EvilNetSon {
	ret, ok := ev.sonConnMap.Load(name)
	if !ok {
		return nil
	}
	return ret.(*EvilNetSon)
}

func (ev *EvilNet) deleteSonConn(name string) {
	ev.sonConnMap.Delete(name)
}

func (ev *EvilNet) updatePeerServer(plugin Plugin) {

}
