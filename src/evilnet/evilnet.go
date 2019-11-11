package evilnet

import (
	"github.com/esrrhs/go-engine/src/common"
	"github.com/esrrhs/go-engine/src/loggo"
	"github.com/esrrhs/go-engine/src/rudp"
	"github.com/golang/protobuf/proto"
	"strconv"
	"time"
)

type EvilNetConfig struct {
	Name string

	Listenport int
	Fatheraddr string

	RegFatherInterSec int

	Rudpconfig rudp.ConnConfig
}

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
	exit   bool
	config *EvilNetConfig

	uuid    string
	localip string

	father *rudp.Conn
	regkey string
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
}

func (ev *EvilNet) Run() {
	loggo.Info("start run")

	go ev.updateFather()
	go ev.updateSon()
}

func (ev *EvilNet) updateFather() {

	regtime := common.GetNowUpdateInSecond()

	for !ev.exit {

		needReg := false
		now := common.GetNowUpdateInSecond()

		if ev.father == nil || !ev.father.IsConnected() {

			loggo.Info("start connect father %s", ev.father)

			conn, err := rudp.Dail(ev.config.Fatheraddr, &ev.config.Rudpconfig)
			if err != nil {
				loggo.Error("connect father fail %s", err)
				time.Sleep(time.Second)
				continue
			}
			ev.father = conn
			needReg = true

			loggo.Info("connect father ok %s", ev.father)
		}

		if now.Sub(regtime) > time.Second*time.Duration(ev.config.RegFatherInterSec) {
			needReg = true
		}

		if needReg {
			regtime = now
			ev.regFather()
		}
	}

}

func (ev *EvilNet) regFather() {

	evm := EvilNetMsg{}
	evm.Type = int32(EvilNetMsg_REQREG)
	evm.ReqRegMsg = &EvilNetReqRegMsg{}
	evm.ReqRegMsg.Name = ev.config.Name
	evm.ReqRegMsg.Key = ev.regkey
	evm.ReqRegMsg.Localaddr = ev.localip + ":" + strconv.Itoa(ev.config.Listenport)

	loggo.Info("reg to father %s %s", evm.ReqRegMsg.Localaddr, ev.father)

	mb, _ := proto.Marshal(&evm)
	ev.father.Write(mb)
}

func (ev *EvilNet) updateSon() {

	for !ev.exit {
		// TODO
	}
}
