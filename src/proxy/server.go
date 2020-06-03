package proxy

import (
	"errors"
	"github.com/esrrhs/go-engine/src/common"
	"github.com/esrrhs/go-engine/src/conn"
	"github.com/esrrhs/go-engine/src/group"
	"github.com/esrrhs/go-engine/src/loggo"
	"strconv"
	"sync"
	"time"
)

type ClientConn struct {
	ProxyConn

	proxyproto PROXY_PROTO
	clienttype CLIENT_TYPE
	fromaddr   string
	toaddr     string
	name       string

	input  *Inputer
	output *Outputer
}

type Server struct {
	config     *Config
	listenaddr string
	listenConn conn.Conn
	wg         *group.Group
	clients    sync.Map
}

func NewServer(config *Config, listenaddr string) (*Server, error) {

	if config == nil {
		config = DefaultConfig()
	}

	conn, err := conn.NewConn(config.Proto)
	if conn == nil {
		return nil, err
	}

	listenConn, err := conn.Listen(listenaddr)
	if err != nil {
		return nil, err
	}

	wg := group.NewGroup(nil, func() {
		listenConn.Close()
	})

	s := &Server{
		config:     config,
		listenaddr: listenaddr,
		listenConn: listenConn,
		wg:         wg,
	}

	wg.Go(func() error {
		return s.listen()
	})

	return s, nil
}

func (s *Server) Close() {
	s.wg.Go(func() error {
		return errors.New("close")
	})
	s.wg.Wait()
	s.listenConn.Close()
}

func (s *Server) listen() error {

	for {
		select {
		case <-s.wg.Done():
			return nil
		case <-time.After(time.Second):
			conn, err := s.listenConn.Accept()
			if err != nil {
				continue
			}
			clientconn := &ClientConn{ProxyConn: ProxyConn{conn: conn}}
			s.wg.Go(func() error {
				return s.serveClient(clientconn)
			})
		}
	}
}

func (s *Server) serveClient(clientconn *ClientConn) error {

	loggo.Info("serveClient accept new client %s", clientconn.conn.Info())

	sendch := common.NewChannel(s.config.MainBuffer)
	recvch := common.NewChannel(s.config.MainBuffer)

	clientconn.sendch = sendch
	clientconn.recvch = recvch

	wg := group.NewGroup(s.wg, func() {
		clientconn.conn.Close()
		sendch.Close()
		recvch.Close()
	})

	wg.Go(func() error {
		return recvFrom(wg, recvch, clientconn.conn, s.config.MaxMsgSize, s.config.Encrypt)
	})

	wg.Go(func() error {
		return sendTo(wg, sendch, clientconn.conn, s.config.Compress, s.config.MaxMsgSize, s.config.Encrypt)
	})

	wg.Go(func() error {
		return checkPingActive(wg, sendch, recvch, &clientconn.ProxyConn, s.config.EstablishedTimeout, s.config.PingInter, s.config.PingTimeoutInter, s.config.ShowPing)
	})

	wg.Go(func() error {
		return checkNeedClose(wg, &clientconn.ProxyConn)
	})

	wg.Go(func() error {
		return s.process(wg, sendch, recvch, clientconn)
	})

	wg.Wait()
	if clientconn.established {
		s.clients.Delete(clientconn.name)
	}
	if clientconn.input != nil {
		clientconn.input.Close()
	}
	if clientconn.output != nil {
		clientconn.output.Close()
	}

	loggo.Info("serveClient close client %s", clientconn.conn.Info())

	return nil
}

func (s *Server) process(wg *group.Group, sendch *common.Channel, recvch *common.Channel, clientconn *ClientConn) error {

	for {
		select {
		case <-wg.Done():
			return nil
		case ff := <-recvch.Ch():
			f := ff.(*ProxyFrame)
			switch f.Type {
			case FRAME_TYPE_LOGIN:
				s.processLogin(wg, f, sendch, clientconn)

			case FRAME_TYPE_PING:
				processPing(f, sendch, &clientconn.ProxyConn)

			case FRAME_TYPE_PONG:
				processPong(f, sendch, &clientconn.ProxyConn, s.config.ShowPing)

			case FRAME_TYPE_DATA:
				s.processData(f, clientconn)

			case FRAME_TYPE_OPEN:
				s.processOpen(f, clientconn)

			case FRAME_TYPE_OPENRSP:
				s.processOpenRsp(f, clientconn)
			}
		}
	}

}

func (s *Server) processLogin(wg *group.Group, f *ProxyFrame, sendch *common.Channel, clientconn *ClientConn) {
	loggo.Info("processLogin from %s %s", clientconn.conn.Info(), f.LoginFrame.String())

	clientconn.proxyproto = f.LoginFrame.Proxyproto
	clientconn.clienttype = f.LoginFrame.Clienttype
	clientconn.fromaddr = f.LoginFrame.Fromaddr
	clientconn.toaddr = f.LoginFrame.Toaddr
	clientconn.name = f.LoginFrame.Name

	rf := &ProxyFrame{}
	rf.Type = FRAME_TYPE_LOGINRSP
	rf.LoginRspFrame = &LoginRspFrame{}

	if f.LoginFrame.Key != s.config.Key {
		rf.LoginRspFrame.Ret = false
		rf.LoginRspFrame.Msg = "key error"
		sendch.Write(rf)
		loggo.Error("processLogin fail key error %s %s", clientconn.conn.Info(), f.LoginFrame.String())
		return
	}

	if clientconn.established {
		rf.LoginRspFrame.Ret = false
		rf.LoginRspFrame.Msg = "has established before"
		sendch.Write(rf)
		loggo.Error("processLogin fail has established before %s %s", clientconn.conn.Info(), f.LoginFrame.String())
		return
	}

	_, loaded := s.clients.LoadOrStore(f.LoginFrame.Name, clientconn)
	if loaded {
		rf.LoginRspFrame.Ret = false
		rf.LoginRspFrame.Msg = "other has login before"
		sendch.Write(rf)
		loggo.Error("processLogin fail other has login before %s %s", clientconn.conn.Info(), f.LoginFrame.String())
		return
	}

	err := s.iniService(wg, f, clientconn)
	if err != nil {
		s.clients.Delete(clientconn.name)
		rf.LoginRspFrame.Ret = false
		rf.LoginRspFrame.Msg = "iniService fail"
		sendch.Write(rf)
		loggo.Error("processLogin iniService fail %s %s", clientconn.conn.Info(), f.LoginFrame.String())
		return
	}

	clientconn.established = true

	rf.LoginRspFrame.Ret = true
	rf.LoginRspFrame.Msg = "ok"
	sendch.Write(rf)

	loggo.Info("processLogin ok %s %s", clientconn.conn.Info(), f.LoginFrame.String())
}

func (s *Server) iniService(wg *group.Group, f *ProxyFrame, clientConn *ClientConn) error {
	switch f.LoginFrame.Clienttype {
	case CLIENT_TYPE_PROXY:
		output, err := NewOutputer(wg, f.LoginFrame.Proxyproto.String(), f.LoginFrame.Toaddr, f.LoginFrame.Clienttype, s.config, &clientConn.ProxyConn)
		if err != nil {
			return err
		}
		clientConn.output = output
	case CLIENT_TYPE_REVERSE_PROXY:
		input, err := NewInputer(wg, f.LoginFrame.Proxyproto.String(), f.LoginFrame.Fromaddr, f.LoginFrame.Clienttype, s.config, &clientConn.ProxyConn)
		if err != nil {
			return err
		}
		clientConn.input = input
	case CLIENT_TYPE_SOCKS5:
		// TODO
	case CLIENT_TYPE_REVERSE_SOCKS5:
		// TODO
	default:
		return errors.New("error CLIENT_TYPE " + strconv.Itoa(int(f.LoginFrame.Clienttype)))
	}
	return nil
}

func (s *Server) processData(f *ProxyFrame, clientconn *ClientConn) {
	if clientconn.input != nil {
		clientconn.input.processDataFrame(f)
	} else if clientconn.output != nil {
		clientconn.output.processDataFrame(f)
	}
}

func (s *Server) processOpenRsp(f *ProxyFrame, clientconn *ClientConn) {
	if clientconn.input != nil {
		clientconn.input.processOpenRspFrame(f)
	}
}

func (c *Server) processOpen(f *ProxyFrame, clientconn *ClientConn) {
	if clientconn.output != nil {
		clientconn.output.processOpenFrame(f)
	}
}

func (c *Server) processClose(f *ProxyFrame, clientconn *ClientConn) {
	if clientconn.input != nil {
		clientconn.input.processCloseFrame(f)
	} else if clientconn.output != nil {
		clientconn.output.processCloseFrame(f)
	}
}
