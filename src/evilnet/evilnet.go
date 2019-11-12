package evilnet

import (
	"github.com/esrrhs/go-engine/src/common"
	"github.com/esrrhs/go-engine/src/loggo"
	"github.com/esrrhs/go-engine/src/msgmgr"
	"github.com/esrrhs/go-engine/src/rudp"
	"github.com/golang/protobuf/proto"
	"math"
	"strconv"
	"sync"
	"time"
)

type EvilNetConfig struct {
	Name string

	Listenport int
	Fatheraddr string

	RegFatherInterSec int

	Rudpconfig rudp.ConnConfig
}

const (
	MSG_MAX_SIZE         int = math.MaxInt16
	CONN_MSG_BUFFER_SIZE int = 100 * 1024
	CONN_MSG_LIST_SIZE   int = 100
)

func (evc *EvilNetConfig) Check() {
	if len(evc.Name) <= 0 {
		evc.Name = common.RandStr(6)
	}
	if evc.Listenport <= 0 {
		evc.Listenport = 5566
	}
	if evc.RegFatherInterSec <= 0 {
		evc.RegFatherInterSec = 60
	}
	evc.Rudpconfig.Check()
}

type EvilNet struct {
	exit           bool
	config         *EvilNetConfig
	workResultLock sync.WaitGroup

	uuid    string
	localip string

	father *rudp.Conn
	regkey string

	son *rudp.Conn
}

func NewEvilNet(config *EvilNetConfig) *EvilNet {
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
		regkey:  common.RandStr(16),
	}

	return ret
}

func (ev *EvilNet) Stop() {
	ev.exit = true
	ev.workResultLock.Wait()
}

func (ev *EvilNet) Run() error {
	addr := ev.localip + ":" + strconv.Itoa(ev.config.Listenport)
	loggo.Info("start run at %s", addr)

	conn, err := rudp.Listen(addr, &ev.config.Rudpconfig)
	if err != nil {
		return err
	}
	ev.son = conn

	if len(ev.config.Fatheraddr) > 0 {
		go ev.updateFather()
	}

	if ev.config.Listenport > 0 {
		go ev.updateSon()
	}

	return nil
}

func (ev *EvilNet) updateFather() {

	ev.workResultLock.Add(1)
	defer ev.workResultLock.Done()

	regtime := common.GetNowUpdateInSecond()

	bytes := make([]byte, 2000)

	for !ev.exit {

		needReg := false
		now := common.GetNowUpdateInSecond()

		// connect
		if ev.father == nil || !ev.father.IsConnected() {

			loggo.Info("start connect father %s", ev.father)

			conn, err := rudp.Dail(ev.config.Fatheraddr, &ev.config.Rudpconfig)
			if err != nil {
				loggo.Error("connect father fail %s", err)
				time.Sleep(time.Second)
				continue
			}
			ev.father = conn
			ev.father.SetUserData(msgmgr.NewMsgMgr(MSG_MAX_SIZE, CONN_MSG_BUFFER_SIZE, CONN_MSG_LIST_SIZE))
			needReg = true

			loggo.Info("connect father ok %s", ev.father)
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

func (ev *EvilNet) updateSon() {

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

	ev.workResultLock.Add(1)
	defer ev.workResultLock.Done()

	conn.SetUserData(msgmgr.NewMsgMgr(MSG_MAX_SIZE, CONN_MSG_BUFFER_SIZE, CONN_MSG_LIST_SIZE))

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
		for e := rl.Front(); e != nil; e = e.Next() {
			rb := e.Value.([]byte)
			enm := &EvilNetMsg{}
			err := proto.Unmarshal(rb, enm)
			if err == nil {
				ev.processSon(conn, enm)
			}
		}
	}

	conn.Close()
}
