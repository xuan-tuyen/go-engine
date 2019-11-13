package evilnet

import (
	"github.com/esrrhs/go-engine/src/loggo"
	"github.com/esrrhs/go-engine/src/msgmgr"
	"github.com/golang/protobuf/proto"
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

func (ev *EvilNet) PreConnect(dst string, eproto string) {

	if !ev.father.IsConnected() {
		return
	}

	evm := EvilNetMsg{}
	evm.Type = int32(EvilNetMsg_REQCONN)
	evm.ReqConnMsg = &EvilNetReqConnMsg{}
	evm.ReqConnMsg.Key = ev.config.ConnectKey
	evm.ReqConnMsg.Proto = eproto
	evm.ReqConnMsg.Localaddr = ev.father.LocalAddr()
	evm.ReqConnMsg.Globaladdr = ev.globaladdr

	mb, _ := proto.Marshal(&evm)

	evmr := EvilNetMsg{}
	evmr.Type = int32(EvilNetMsg_ROUTER)
	evmr.RouterMsg = &EvilNetRouterMsg{}
	evmr.RouterMsg.Src = ev.globalname
	evmr.RouterMsg.Dst = dst
	evmr.RouterMsg.Data = mb

	mbr, _ := proto.Marshal(&evmr)

	mm := ev.father.UserData().(*msgmgr.MsgMgr)
	mm.Send(mbr)

	loggo.Info("pre connect to %s by father %s", dst, ev.config.Fatheraddr)
}
