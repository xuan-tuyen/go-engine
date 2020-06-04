package proxy

import (
	"github.com/esrrhs/go-engine/src/common"
	"github.com/esrrhs/go-engine/src/conn"
	"github.com/esrrhs/go-engine/src/group"
	"github.com/esrrhs/go-engine/src/loggo"
	"sync"
)

type Outputer struct {
	clienttype CLIENT_TYPE
	config     *Config
	proto      string
	father     *ProxyConn
	fwg        *group.Group

	conn  conn.Conn
	sonny sync.Map
}

func NewOutputer(wg *group.Group, proto string, clienttype CLIENT_TYPE, config *Config, father *ProxyConn) (*Outputer, error) {
	conn, err := conn.NewConn(proto)
	if conn == nil {
		return nil, err
	}

	output := &Outputer{
		clienttype: clienttype,
		config:     config,
		conn:       conn,
		proto:      proto,
		father:     father,
		fwg:        wg,
	}

	loggo.Info("NewOutputer ok %s", proto)

	return output, nil
}

func (o *Outputer) Close() {
	o.conn.Close()
}

func (o *Outputer) processDataFrame(f *ProxyFrame) {
	id := f.DataFrame.Id
	v, ok := o.sonny.Load(id)
	if !ok {
		loggo.Info("Outputer processDataFrame no sonnny %s %d", f.DataFrame.Id, len(f.DataFrame.Data))
		return
	}
	sonny := v.(*ProxyConn)
	sonny.sendch.Write(f)
	sonny.actived++
	loggo.Debug("Outputer processDataFrame %s %d", f.DataFrame.Id, len(f.DataFrame.Data))
}

func (o *Outputer) processCloseFrame(f *ProxyFrame) {
	id := f.CloseFrame.Id
	v, ok := o.sonny.Load(id)
	if !ok {
		loggo.Info("Outputer processCloseFrame no sonnny %s", f.CloseFrame.Id)
		return
	}

	sonny := v.(*ProxyConn)
	sonny.sendch.Write(f)
}

func (o *Outputer) processOpenFrame(f *ProxyFrame) {

	id := f.OpenFrame.Id
	addr := f.OpenFrame.Toaddr

	rf := &ProxyFrame{}
	rf.Type = FRAME_TYPE_OPENRSP
	rf.OpenRspFrame = &OpenConnRspFrame{}
	rf.OpenRspFrame.Id = id

	conn, err := o.conn.Dial(addr)
	if err != nil {
		rf.OpenRspFrame.Ret = false
		rf.OpenRspFrame.Msg = "Dial fail " + addr
		o.father.sendch.Write(rf)
		loggo.Error("Outputer processOpenFrame Dial fail %s %s", addr, err.Error())
		return
	}

	proxyconn := &ProxyConn{id: id, conn: conn, established: true}
	_, loaded := o.sonny.LoadOrStore(proxyconn.id, proxyconn)
	if loaded {
		loggo.Error("Outputer processOpenFrame LoadOrStore fail %s %s", addr, id)
		proxyconn.conn.Close()
		return
	}

	sendch := common.NewChannel(o.config.ConnBuffer)
	recvch := common.NewChannel(o.config.ConnBuffer)

	proxyconn.sendch = sendch
	proxyconn.recvch = recvch

	rf.OpenRspFrame.Ret = true
	rf.OpenRspFrame.Msg = "ok"
	o.father.sendch.Write(rf)

	o.fwg.Go("Outputer processProxyConn", func() error {
		return o.processProxyConn(proxyconn)
	})
}

func (o *Outputer) processProxyConn(proxyConn *ProxyConn) error {

	loggo.Info("Outputer processProxyConn start %s %s", proxyConn.id, proxyConn.conn.Info())

	sendch := proxyConn.sendch
	recvch := proxyConn.recvch

	wg := group.NewGroup(o.fwg, func() {
		proxyConn.conn.Close()
		sendch.Close()
		recvch.Close()
	})

	wg.Go("Outputer recvFromSonny", func() error {
		return recvFromSonny(wg, recvch, proxyConn.conn, o.config.MaxMsgSize)
	})

	wg.Go("Outputer sendToSonny", func() error {
		return sendToSonny(wg, sendch, proxyConn.conn)
	})

	wg.Go("Outputer checkSonnyActive", func() error {
		return checkSonnyActive(wg, proxyConn, o.config.EstablishedTimeout, o.config.ConnTimeout)
	})

	wg.Go("Outputer checkNeedClose", func() error {
		return checkNeedClose(wg, proxyConn)
	})

	wg.Go("Outputer copySonnyRecv", func() error {
		return copySonnyRecv(wg, recvch, proxyConn, o.father)
	})

	wg.Wait()
	o.sonny.Delete(proxyConn.id)

	closeRemoteConn(proxyConn, o.father)

	loggo.Info("Outputer processProxyConn end %s %s", proxyConn.id, proxyConn.conn.Info())

	return nil
}
