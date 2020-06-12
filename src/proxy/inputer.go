package proxy

import (
	"errors"
	"github.com/esrrhs/go-engine/src/common"
	"github.com/esrrhs/go-engine/src/conn"
	"github.com/esrrhs/go-engine/src/group"
	"github.com/esrrhs/go-engine/src/loggo"
	"github.com/esrrhs/go-engine/src/network"
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

func NewInputer(wg *group.Group, proto string, addr string, clienttype CLIENT_TYPE, config *Config, father *ProxyConn, targetAddr string) (*Inputer, error) {
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

	wg.Go("Inputer listen"+" "+targetAddr, func() error {
		return input.listen(targetAddr)
	})

	loggo.Info("NewInputer ok %s", addr)

	return input, nil
}

func NewSocks5Inputer(wg *group.Group, proto string, addr string, clienttype CLIENT_TYPE, config *Config, father *ProxyConn) (*Inputer, error) {
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

	wg.Go("Inputer listenSocks5"+" "+addr, func() error {
		return input.listenSocks5()
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

func (i *Inputer) listen(targetAddr string) error {

	loggo.Info("Inputer start listen %s %s", i.addr, targetAddr)

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
			i.fwg.Go("Inputer processProxyConn"+" "+targetAddr, func() error {
				return i.processProxyConn(proxyconn, targetAddr)
			})
		}
	}
}

func (i *Inputer) listenSocks5() error {

	loggo.Info("Inputer start listenSocks5 %s", i.addr)

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
			i.fwg.Go("Inputer processSocks5Conn"+" "+conn.Info(), func() error {
				return i.processSocks5Conn(proxyconn)
			})
		}
	}
}

func (i *Inputer) processSocks5Conn(proxyConn *ProxyConn) error {

	wg := group.NewGroup(i.fwg, func() {
		proxyConn.conn.Close()
	})

	targetAddr := ""
	wg.Go("Inputer socks5"+" "+proxyConn.conn.Info(), func() error {
		if proxyConn.conn.Name() != "tcp" {
			loggo.Error("processSocks5Conn no tcp %s %s", proxyConn.conn.Info(), proxyConn.conn.Name())
			return errors.New("socks5 not tcp")
		}

		var err error = nil
		if err = network.Sock5HandshakeBy(proxyConn.conn, i.config.Username, i.config.Password); err != nil {
			loggo.Error("processSocks5Conn Sock5HandshakeBy %s %s", proxyConn.conn.Info(), err)
			return err
		}
		_, addr, err := network.Sock5GetRequest(proxyConn.conn)
		if err != nil {
			loggo.Error("processSocks5Conn Sock5GetRequest %s %s", proxyConn.conn.Info(), err)
			return err
		}
		// Sending connection established message immediately to client.
		// This some round trip time for creating socks connection with the client.
		// But if connection failed, the client will get connection reset error.
		_, err = proxyConn.conn.Write([]byte{0x05, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x08, 0x43})
		if err != nil {
			loggo.Error("processSocks5Conn Write %s %s", proxyConn.conn.Info(), err)
			return err
		}

		targetAddr = addr
		return nil
	})

	err := wg.Wait()
	if err != nil {
		return nil
	}

	loggo.Info("processSocks5Conn ok %s %s", proxyConn.conn.Info(), targetAddr)

	i.fwg.Go("Inputer processProxyConn"+" "+proxyConn.conn.Info(), func() error {
		return i.processProxyConn(proxyConn, targetAddr)
	})

	return nil
}

func (i *Inputer) processProxyConn(proxyConn *ProxyConn, targetAddr string) error {

	proxyConn.id = common.UniqueId()

	loggo.Info("Inputer processProxyConn start %s %s %s", proxyConn.id, proxyConn.conn.Info(), targetAddr)

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

	i.openConn(proxyConn, targetAddr)

	wg.Go("Inputer recvFromSonny"+" "+proxyConn.conn.Info(), func() error {
		return recvFromSonny(wg, recvch, proxyConn.conn, i.config.MaxMsgSize)
	})

	wg.Go("Inputer sendToSonny"+" "+proxyConn.conn.Info(), func() error {
		return sendToSonny(wg, sendch, proxyConn.conn)
	})

	wg.Go("Inputer checkSonnyActive"+" "+proxyConn.conn.Info(), func() error {
		return checkSonnyActive(wg, proxyConn, i.config.EstablishedTimeout, i.config.ConnTimeout)
	})

	wg.Go("Inputer checkNeedClose"+" "+proxyConn.conn.Info(), func() error {
		return checkNeedClose(wg, proxyConn)
	})

	wg.Go("Inputer copySonnyRecv"+" "+proxyConn.conn.Info(), func() error {
		return copySonnyRecv(wg, recvch, proxyConn, i.father)
	})

	wg.Wait()
	i.sonny.Delete(proxyConn.id)

	closeRemoteConn(proxyConn, i.father)

	loggo.Info("Inputer processProxyConn end %s %s %s", proxyConn.id, proxyConn.conn.Info(), targetAddr)

	return nil
}

func (i *Inputer) openConn(proxyConn *ProxyConn, targetAddr string) {
	f := &ProxyFrame{}
	f.Type = FRAME_TYPE_OPEN
	f.OpenFrame = &OpenConnFrame{}
	f.OpenFrame.Id = proxyConn.id
	f.OpenFrame.Toaddr = targetAddr

	i.father.sendch.Write(f)
	loggo.Info("Inputer openConn %s %s", proxyConn.id, targetAddr)
}
