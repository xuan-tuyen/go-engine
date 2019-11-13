package evilnet

import (
	"github.com/esrrhs/go-engine/src/loggo"
	"github.com/esrrhs/go-engine/src/msgmgr"
	"github.com/esrrhs/go-engine/src/rudp"
	"github.com/golang/protobuf/proto"
)

func (ev *EvilNet) processFather(enm *EvilNetMsg) {
	loggo.Info("process father msg %d", enm.Type)

	if enm.Type == int32(EvilNetMsg_RSPREG) && enm.RspRegMsg != nil {
		ev.processFatherRspReg(enm)
	}
}

func (ev *EvilNet) processFatherRspReg(enm *EvilNetMsg) {
	loggo.Info("process father rsp reg msg %s", enm.RspRegMsg.String())

}

func (ev *EvilNet) process(enm *EvilNetMsg) {
	loggo.Info("process msg %d", enm.Type)

}

func (ev *EvilNet) regFather() {

	evm := EvilNetMsg{}
	evm.Type = int32(EvilNetMsg_REQREG)
	evm.ReqRegMsg = &EvilNetReqRegMsg{}
	evm.ReqRegMsg.Name = ev.config.Name
	evm.ReqRegMsg.Key = ev.regkey
	evm.ReqRegMsg.Localaddr = ev.father.LocalAddr()

	loggo.Info("reg to father %s %s", evm.ReqRegMsg.Localaddr, ev.config.Fatheraddr)

	mb, _ := proto.Marshal(&evm)

	mm := ev.father.UserData().(*msgmgr.MsgMgr)
	mm.Send(mb)
}

func (ev *EvilNet) processSon(conn *rudp.Conn, enm *EvilNetMsg) {
	loggo.Info("process son msg %d %s", enm.Type, conn.RemoteAddr())

	if enm.Type == int32(EvilNetMsg_REQREG) && enm.ReqRegMsg != nil {
		ev.processFatherReqReg(conn, enm)
	}
}

func (ev *EvilNet) processFatherReqReg(conn *rudp.Conn, enm *EvilNetMsg) {
	loggo.Info("process father req reg msg %s %s", conn.RemoteAddr(), enm.ReqRegMsg.String())

}
