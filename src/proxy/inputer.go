package proxy

import (
	"github.com/esrrhs/go-engine/src/common"
	"github.com/esrrhs/go-engine/src/conn"
	"github.com/esrrhs/go-engine/src/group"
	"github.com/esrrhs/go-engine/src/loggo"
	"sync"
	"time"
)

type Inputer struct {
	clienttype CLIENT_TYPE
	config     *Config
	proto      string
	addr       string
	father     *ProxyConn
	fwg        *group.Group

	listenconn conn.Conn
	sonny      sync.Map
}

func NewInputer(wg *group.Group, proto string, addr string, clienttype CLIENT_TYPE, config *Config, father *ProxyConn) (*Inputer, error) {
	conn, err := conn.NewConn(proto)
	if conn == nil {
		return nil, err
	}

	listenconn, err := conn.Listen(addr)
	if err != nil {
		return nil, err
	}

	input := &Inputer{
		clienttype: clienttype,
		config:     config,
		proto:      proto,
		addr:       addr,
		father:     father,
		fwg:        wg,
		listenconn: listenconn,
	}

	wg.Go("Inputer listen", func() error {
		return input.listen()
	})

	loggo.Info("NewInputer ok %s", addr)

	return input, nil
}

func (i *Inputer) Close() {
	i.listenconn.Close()
}

func (i *Inputer) processDataFrame(f *ProxyFrame) {
	id := f.DataFrame.Id
	v, ok := i.sonny.Load(id)
	if !ok {
		loggo.Info("Inputer processDataFrame no sonnny %s %d", id, len(f.DataFrame.Data))
		return
	}
	sonny := v.(*ProxyConn)
	sonny.sendch.Write(f)
	sonny.actived++
	loggo.Debug("Inputer processDataFrame %s %d", f.DataFrame.Id, len(f.DataFrame.Data))
}

func (i *Inputer) processCloseFrame(f *ProxyFrame) {
	id := f.CloseFrame.Id
	v, ok := i.sonny.Load(id)
	if !ok {
		loggo.Info("Inputer processCloseFrame no sonnny %s", f.CloseFrame.Id)
		return
	}

	sonny := v.(*ProxyConn)
	sonny.sendch.Write(f)
}

func (i *Inputer) processOpenRspFrame(f *ProxyFrame) {
	id := f.OpenRspFrame.Id
	v, ok := i.sonny.Load(id)
	if !ok {
		loggo.Error("Inputer processOpenRspFrame no sonnny %s", id)
		return
	}
	sonny := v.(*ProxyConn)
	if f.OpenRspFrame.Ret {
		sonny.established = true
		loggo.Info("Inputer processOpenRspFrame ok %s %s", id, sonny.conn.Info())
	} else {
		sonny.needclose = true
		loggo.Info("Inputer processOpenRspFrame fail %s %s", id, sonny.conn.Info())
	}
}

func (i *Inputer) listen() error {

	loggo.Info("Inputer start listen %s", i.addr)

	for {
		select {
		case <-i.fwg.Done():
			return nil
		case <-time.After(time.Second):
			conn, err := i.listenconn.Accept()
			if err != nil {
				continue
			}
			proxyconn := &ProxyConn{conn: conn}
			i.fwg.Go("Inputer processProxyConn", func() error {
				return i.processProxyConn(proxyconn)
			})
		}
	}
}

func (i *Inputer) processProxyConn(proxyConn *ProxyConn) error {

	proxyConn.id = common.UniqueId()

	loggo.Info("Inputer processProxyConn start %s %s", proxyConn.id, proxyConn.conn.Info())

	_, loaded := i.sonny.LoadOrStore(proxyConn.id, proxyConn)
	if loaded {
		loggo.Error("Inputer processProxyConn LoadOrStore fail %s", proxyConn.id)
		proxyConn.conn.Close()
		return nil
	}

	sendch := common.NewChannel(i.config.ConnBuffer)
	recvch := common.NewChannel(i.config.ConnBuffer)

	proxyConn.sendch = sendch
	proxyConn.recvch = recvch

	wg := group.NewGroup(i.fwg, func() {
		proxyConn.conn.Close()
		sendch.Close()
		recvch.Close()
	})

	i.openConn(proxyConn)

	wg.Go("Inputer recvFromSonny", func() error {
		return recvFromSonny(wg, recvch, proxyConn.conn, i.config.MaxMsgSize)
	})

	wg.Go("Inputer sendToSonny", func() error {
		return sendToSonny(wg, sendch, proxyConn.conn)
	})

	wg.Go("Inputer checkSonnyActive", func() error {
		return checkSonnyActive(wg, proxyConn, i.config.EstablishedTimeout, i.config.ConnTimeout)
	})

	wg.Go("Inputer checkNeedClose", func() error {
		return checkNeedClose(wg, proxyConn)
	})

	wg.Go("Inputer copySonnyRecv", func() error {
		return copySonnyRecv(wg, recvch, proxyConn, i.father)
	})

	wg.Wait()
	i.sonny.Delete(proxyConn.id)

	closeRemoteConn(proxyConn, i.father)

	loggo.Info("Inputer processProxyConn end %s %s", proxyConn.id, proxyConn.conn.Info())

	return nil
}

func (i *Inputer) openConn(proxyConn *ProxyConn) {
	f := &ProxyFrame{}
	f.Type = FRAME_TYPE_OPEN
	f.OpenFrame = &OpenConnFrame{}
	f.OpenFrame.Id = proxyConn.id

	i.father.sendch.Write(f)
	loggo.Info("Inputer openConn %s", proxyConn.id)
}
