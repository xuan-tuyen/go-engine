package evilnet

import (
	"github.com/esrrhs/go-engine/src/loggo"
	"github.com/esrrhs/go-engine/src/msgmgr"
	"github.com/golang/protobuf/proto"
	"strings"
	"time"
)

func (ev *EvilNet) regFather() {

	evm := EvilNetMsg{}
	evm.Type = int32(EvilNetMsg_REQREG)
	evm.ReqRegMsg = &EvilNetReqRegMsg{}
	evm.ReqRegMsg.Name = ev.config.Name
	evm.ReqRegMsg.Key = ev.config.FatherKey
	evm.ReqRegMsg.Sonkey = ev.config.Key
	evm.ReqRegMsg.Localaddr = ev.father.LocalAddr()

	loggo.Info("reg to father %s %s", evm.ReqRegMsg.Localaddr, ev.config.Fatheraddr)

	mb, _ := proto.Marshal(&evm)

	mm := ev.father.UserData().(*msgmgr.MsgMgr)
	mm.Send(mb)
}

func (ev *EvilNet) PreConnect(dst string, eproto string, param []string) {

	evm := EvilNetMsg{}
	evm.Type = int32(EvilNetMsg_REQCONN)
	evm.ReqConnMsg = &EvilNetReqConnMsg{}
	evm.ReqConnMsg.Key = ev.config.ConnectKey
	evm.ReqConnMsg.Proto = eproto
	evm.ReqConnMsg.Localaddr = ev.father.LocalAddr()
	evm.ReqConnMsg.Globaladdr = ev.globaladdr
	evm.ReqConnMsg.Param = param

	evmr := ev.packRouterMsg(ev.globalname, dst, &evm)

	ev.routerMsg(ev.globalname, dst, evmr)

	loggo.Info("pre connect to %s by father %s", dst, ev.config.Fatheraddr)
}

func (ev *EvilNet) packRouterMsg(src string, dst string, enm *EvilNetMsg) *EvilNetMsg {

	mb, _ := proto.Marshal(enm)

	evmr := EvilNetMsg{}
	evmr.Type = int32(EvilNetMsg_ROUTER)
	evmr.RouterMsg = &EvilNetRouterMsg{}
	evmr.RouterMsg.Src = src
	evmr.RouterMsg.Dst = dst
	evmr.RouterMsg.Data = mb

	return &evmr
}

func (ev *EvilNet) routerMsg(src string, dst string, enm *EvilNetMsg) {

	dsts := strings.Split(dst, ".")
	curs := strings.Split(ev.globalname, ".")

	// dst A.B.C.D.E.F, cur A.B
	if strings.Contains(dst, ev.globalname) {
		sonname := dsts[len(curs)]
		son := ev.getSonConn(sonname)
		if son != nil {
			mb, _ := proto.Marshal(enm)
			mm := son.conn.UserData().(*msgmgr.MsgMgr)
			mm.Send(mb)

			loggo.Info("%s router son %s->%s msg to son %s", src, dst, son.name, ev.globalname)
			return
		} else {
			loggo.Error("%s router son %s->%s msg no son %s", src, dst, son.name, ev.globalname)
			return
		}
	}

	// dst A.D.E.F, cur A.B
	// dst A.B.E.F, cur A.B.C
	// dst A.D.E.F, cur A.B.C
	if strings.Contains(dst, ev.globalname) {
		if ev.father != nil && ev.father.IsConnected() {
			mb, _ := proto.Marshal(enm)
			mm := ev.father.UserData().(*msgmgr.MsgMgr)
			mm.Send(mb)

			loggo.Info("router son %s->%s msg to father %s %s", src, dst, ev.fathername, ev.globalname)
			return
		} else {
			loggo.Error("router son %s->%s msg no father %s %s", src, dst, ev.fathername, ev.globalname)
			return
		}
	}

	loggo.Error("router son %s->%s msg ignore %s", src, dst, ev.globalname)
}

func (ev *EvilNet) Ping(dst string) {

	evm := EvilNetMsg{}
	evm.Type = int32(EvilNetMsg_PING)
	evm.PingMsg = &EvilNetPingMsg{}
	evm.PingMsg.Time = time.Now().UnixNano()

	evmr := ev.packRouterMsg(ev.globalname, dst, &evm)

	ev.routerMsg(ev.globalname, dst, evmr)

	loggo.Info("send ping %s", dst)
}
