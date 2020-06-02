package proxy

import (
	"context"
	"errors"
	"github.com/esrrhs/go-engine/src/common"
	"github.com/esrrhs/go-engine/src/conn"
	"github.com/esrrhs/go-engine/src/loggo"
	"golang.org/x/sync/errgroup"
	"net"
	"strconv"
	"sync"
	"time"
)

type ClientConnSonny struct {
	conn      *net.TCPConn
	id        string
	sendch    chan *ProxyFrame
	recvch    chan *ProxyFrame
	active    int64
	opened    bool
	needclose bool
}

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
	wg         *errgroup.Group
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

	wg, ctx := errgroup.WithContext(context.Background())

	s := &Server{
		config:     config,
		listenaddr: listenaddr,
		listenConn: listenConn,
		wg:         wg,
	}

	wg.Go(func() error {
		return s.listen(ctx)
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

func (s *Server) listen(ctx context.Context) error {

	defer common.CrashLog()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(time.Second):
			conn, err := s.listenConn.Accept()
			if err != nil {
				continue
			}
			clientconn := &ClientConn{ProxyConn: ProxyConn{conn: conn}}
			go s.serveClient(ctx, clientconn)
		}
	}
}

func (s *Server) serveClient(fctx context.Context, clientconn *ClientConn) {
	defer common.CrashLog()

	loggo.Info("serveClient accept new client %s", clientconn.conn.Info())

	wg, ctx := errgroup.WithContext(fctx)

	sendch := make(chan *ProxyFrame, s.config.MainBuffer)
	recvch := make(chan *ProxyFrame, s.config.MainBuffer)

	clientconn.sendch = sendch
	clientconn.recvch = recvch

	wg.Go(func() error {
		return recvFrom(ctx, recvch, clientconn.conn, s.config.MaxMsgSize, s.config.Encrypt)
	})

	wg.Go(func() error {
		return sendTo(ctx, sendch, clientconn.conn, s.config.Compress, s.config.MaxMsgSize, s.config.Encrypt)
	})

	wg.Go(func() error {
		return checkPingActive(ctx, sendch, recvch, &clientconn.ProxyConn, s.config.EstablishedTimeout, s.config.PingInter, s.config.PingTimeoutInter, s.config.ShowPing)
	})

	wg.Go(func() error {
		return checkNeedClose(ctx, &clientconn.ProxyConn)
	})

	wg.Go(func() error {
		return s.process(ctx, wg, sendch, recvch, clientconn)
	})

	wg.Wait()
	clientconn.conn.Close()
	if clientconn.established {
		s.clients.Delete(clientconn.name)
	}
	if clientconn.input != nil {
		clientconn.input.Close()
	}
	if clientconn.output != nil {
		clientconn.output.Close()
	}
	close(sendch)
	close(recvch)

	loggo.Info("serveClient close client %s", clientconn.conn.Info())
}

func (s *Server) process(ctx context.Context, wg *errgroup.Group, sendch chan<- *ProxyFrame, recvch <-chan *ProxyFrame, clientconn *ClientConn) error {
	defer common.CrashLog()

	for {
		select {
		case <-ctx.Done():
			return nil
		case f := <-recvch:
			switch f.Type {
			case FRAME_TYPE_LOGIN:
				s.processLogin(ctx, wg, f, sendch, clientconn)

			case FRAME_TYPE_PING:
				processPing(ctx, f, sendch, &clientconn.ProxyConn)

			case FRAME_TYPE_PONG:
				processPong(ctx, f, sendch, &clientconn.ProxyConn, s.config.ShowPing)

			case FRAME_TYPE_DATA:
				s.processData(ctx, f, sendch, clientconn)

			case FRAME_TYPE_OPEN:
				s.processOpen(ctx, f, sendch, clientconn)

			case FRAME_TYPE_OPENRSP:
				s.processOpenRsp(ctx, f, sendch, clientconn)
			}
		}
	}

}

func (s *Server) processLogin(ctx context.Context, wg *errgroup.Group, f *ProxyFrame, sendch chan<- *ProxyFrame, clientconn *ClientConn) {
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
		sendch <- rf
		loggo.Error("processLogin fail key error %s %s", clientconn.conn.Info(), f.LoginFrame.String())
		return
	}

	if clientconn.established {
		rf.LoginRspFrame.Ret = false
		rf.LoginRspFrame.Msg = "has established before"
		sendch <- rf
		loggo.Error("processLogin fail has established before %s %s", clientconn.conn.Info(), f.LoginFrame.String())
		return
	}

	_, loaded := s.clients.LoadOrStore(f.LoginFrame.Name, clientconn)
	if loaded {
		rf.LoginRspFrame.Ret = false
		rf.LoginRspFrame.Msg = "other has login before"
		sendch <- rf
		loggo.Error("processLogin fail other has login before %s %s", clientconn.conn.Info(), f.LoginFrame.String())
		return
	}

	err := s.iniService(ctx, wg, f, clientconn)
	if err != nil {
		s.clients.Delete(clientconn.name)
		rf.LoginRspFrame.Ret = false
		rf.LoginRspFrame.Msg = "iniService fail"
		sendch <- rf
		loggo.Error("processLogin iniService fail %s %s", clientconn.conn.Info(), f.LoginFrame.String())
		return
	}

	clientconn.established = true

	rf.LoginRspFrame.Ret = true
	rf.LoginRspFrame.Msg = "ok"
	sendch <- rf

	loggo.Info("processLogin ok %s %s", clientconn.conn.Info(), f.LoginFrame.String())
}

func (s *Server) iniService(ctx context.Context, wg *errgroup.Group, f *ProxyFrame, clientConn *ClientConn) error {
	switch f.LoginFrame.Clienttype {
	case CLIENT_TYPE_PROXY:
		output, err := NewOutputer(ctx, wg, f.LoginFrame.Proxyproto.String(), f.LoginFrame.Toaddr, f.LoginFrame.Clienttype, s.config, &clientConn.ProxyConn)
		if err != nil {
			return err
		}
		clientConn.output = output
	case CLIENT_TYPE_REVERSE_PROXY:
		input, err := NewInputer(ctx, wg, f.LoginFrame.Proxyproto.String(), f.LoginFrame.Fromaddr, f.LoginFrame.Clienttype, s.config, &clientConn.ProxyConn)
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

func (s *Server) processData(ctx context.Context, f *ProxyFrame, sendch chan<- *ProxyFrame, clientconn *ClientConn) {
	if clientconn.input != nil {
		clientconn.input.processDataFrame(f)
	} else if clientconn.output != nil {
		clientconn.output.processDataFrame(f)
	}
}

func (s *Server) processOpenRsp(ctx context.Context, f *ProxyFrame, sendch chan<- *ProxyFrame, clientconn *ClientConn) {
	if clientconn.input != nil {
		clientconn.input.processOpenRspFrame(f)
	}
}

func (c *Server) processOpen(ctx context.Context, f *ProxyFrame, sendch chan<- *ProxyFrame, clientconn *ClientConn) {
	if clientconn.output != nil {
		clientconn.output.processOpenFrame(ctx, f)
	}
}

func (c *Server) processClose(ctx context.Context, f *ProxyFrame, sendch chan<- *ProxyFrame, clientconn *ClientConn) {
	if clientconn.input != nil {
		clientconn.input.processCloseFrame(f)
	} else if clientconn.output != nil {
		clientconn.output.processCloseFrame(f)
	}
}
