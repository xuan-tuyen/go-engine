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
	"strings"
	"time"
)

type ServerConnSonny struct {
	conn   *net.TCPConn
	id     string
	sendch chan *ProxyFrame
	recvch chan *ProxyFrame
	active int64
}

type ServerConn struct {
	ProxyConn
	output *Outputer
	input  *Inputer
}

type Client struct {
	config     *Config
	server     string
	name       string
	clienttype CLIENT_TYPE
	proxyproto PROXY_PROTO
	fromaddr   string
	toaddr     string

	wg         *errgroup.Group
	serverconn *ServerConn
}

func NewClient(config *Config, server string, name string, clienttypestr string, proxyprotostr string, fromaddr string, toaddr string) (*Client, error) {

	if config == nil {
		config = DefaultConfig()
	}

	conn, err := conn.NewConn(config.Proto)
	if conn == nil {
		return nil, err
	}

	clienttypestr = strings.ToUpper(clienttypestr)
	clienttype, ok := CLIENT_TYPE_value[clienttypestr]
	if !ok {
		return nil, errors.New("no CLIENT_TYPE " + clienttypestr)
	}

	proxyprotostr = strings.ToUpper(proxyprotostr)
	proxyproto, ok := PROXY_PROTO_value[proxyprotostr]
	if !ok {
		return nil, errors.New("no PROXY_PROTO " + proxyprotostr)
	}

	wg, ctx := errgroup.WithContext(context.Background())

	c := &Client{
		config:     config,
		server:     server,
		name:       name,
		clienttype: CLIENT_TYPE(clienttype),
		proxyproto: PROXY_PROTO(proxyproto),
		fromaddr:   fromaddr,
		toaddr:     toaddr,
		wg:         wg,
	}

	wg.Go(func() error {
		return c.connect(ctx, conn)
	})

	return c, nil
}

func (c *Client) Close() {
	c.wg.Go(func() error {
		return errors.New("close")
	})
	if c.serverconn != nil {
		c.serverconn.conn.Close()
	}
	c.wg.Wait()
}

func (c *Client) connect(ctx context.Context, conn conn.Conn) error {

	defer common.CrashLog()

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-time.After(time.Second):
			if c.serverconn == nil {
				targetconn, err := conn.Dial(c.server)
				if err != nil {
					loggo.Error("connect Dial fail: %s %s", c.server, err.Error())
					continue
				}
				c.serverconn = &ServerConn{ProxyConn: ProxyConn{conn: targetconn}}
				go c.useServer(ctx, c.serverconn)
			}
		}
	}
}

func (c *Client) useServer(fctx context.Context, serverconn *ServerConn) {
	defer common.CrashLog()

	loggo.Info("useServer %s", serverconn.conn.Info())

	wg, ctx := errgroup.WithContext(fctx)

	sendch := make(chan *ProxyFrame, c.config.MainBuffer)
	recvch := make(chan *ProxyFrame, c.config.MainBuffer)

	serverconn.sendch = sendch
	serverconn.recvch = recvch

	c.login(sendch)

	wg.Go(func() error {
		return recvFrom(ctx, recvch, serverconn.conn, c.config.MaxMsgSize, c.config.Encrypt)
	})

	wg.Go(func() error {
		return sendTo(ctx, sendch, serverconn.conn, c.config.Compress, c.config.MaxMsgSize, c.config.Encrypt)
	})

	wg.Go(func() error {
		return checkPingActive(ctx, sendch, recvch, &serverconn.ProxyConn, c.config.EstablishedTimeout, c.config.PingInter, c.config.PingTimeoutInter, c.config.ShowPing)
	})

	wg.Go(func() error {
		return checkNeedClose(ctx, &serverconn.ProxyConn)
	})

	wg.Go(func() error {
		return c.process(ctx, wg, sendch, recvch, serverconn)
	})

	wg.Wait()
	serverconn.conn.Close()
	if serverconn.output != nil {
		serverconn.output.Close()
	}
	if serverconn.input != nil {
		serverconn.input.Close()
	}
	c.serverconn = nil
	loggo.Info("useServer close %s %s", c.server, serverconn.conn.Info())
}

func (c *Client) login(sendch chan<- *ProxyFrame) {
	f := &ProxyFrame{}
	f.Type = FRAME_TYPE_LOGIN
	f.LoginFrame = &LoginFrame{}
	f.LoginFrame.Proxyproto = c.proxyproto
	f.LoginFrame.Clienttype = c.clienttype
	f.LoginFrame.Fromaddr = c.fromaddr
	f.LoginFrame.Toaddr = c.toaddr
	f.LoginFrame.Name = c.name
	f.LoginFrame.Key = c.config.Key

	sendch <- f

	loggo.Info("start login %s %s", c.server, f.LoginFrame.String())
}

func (c *Client) process(ctx context.Context, wg *errgroup.Group, sendch chan<- *ProxyFrame, recvch <-chan *ProxyFrame, serverconn *ServerConn) error {
	defer common.CrashLog()

	for {
		select {
		case <-ctx.Done():
			return nil
		case f := <-recvch:
			switch f.Type {
			case FRAME_TYPE_LOGINRSP:
				c.processLoginRsp(ctx, wg, f, sendch, serverconn)

			case FRAME_TYPE_PING:
				processPing(ctx, f, sendch, &serverconn.ProxyConn)

			case FRAME_TYPE_PONG:
				processPong(ctx, f, sendch, &serverconn.ProxyConn, c.config.ShowPing)

			case FRAME_TYPE_DATA:
				c.processData(ctx, f, sendch, serverconn)

			case FRAME_TYPE_OPEN:
				c.processOpen(ctx, f, sendch, serverconn)

			case FRAME_TYPE_OPENRSP:
				c.processOpenRsp(ctx, f, sendch, serverconn)
			}
		}
	}

}

func (c *Client) processLoginRsp(ctx context.Context, wg *errgroup.Group, f *ProxyFrame, sendch chan<- *ProxyFrame, serverconn *ServerConn) {
	if !f.LoginRspFrame.Ret {
		serverconn.needclose = true
		loggo.Error("processLoginRsp fail %s %s", c.server, f.LoginRspFrame.Msg)
		return
	}

	loggo.Info("processLoginRsp ok %s", c.server)

	err := c.iniService(ctx, wg, serverconn)
	if err != nil {
		loggo.Error("processLoginRsp iniService fail %s", c.server)
		return
	}

	serverconn.established = true
}

func (c *Client) iniService(ctx context.Context, wg *errgroup.Group, serverConn *ServerConn) error {
	switch c.clienttype {
	case CLIENT_TYPE_PROXY:
		input, err := NewInputer(ctx, wg, c.proxyproto.String(), c.fromaddr, c.clienttype, c.config, &serverConn.ProxyConn)
		if err != nil {
			return err
		}
		serverConn.input = input
	case CLIENT_TYPE_REVERSE_PROXY:
		output, err := NewOutputer(ctx, wg, c.proxyproto.String(), c.toaddr, c.clienttype, c.config, &serverConn.ProxyConn)
		if err != nil {
			return err
		}
		serverConn.output = output
	case CLIENT_TYPE_SOCKS5:
		// TODO
	case CLIENT_TYPE_REVERSE_SOCKS5:
		// TODO
	default:
		return errors.New("error CLIENT_TYPE " + strconv.Itoa(int(c.clienttype)))
	}
	return nil
}

func (c *Client) processData(ctx context.Context, f *ProxyFrame, sendch chan<- *ProxyFrame, serverconn *ServerConn) {
	if serverconn.output != nil {
		serverconn.output.processDataFrame(f)
	}
}

func (c *Client) processOpen(ctx context.Context, f *ProxyFrame, sendch chan<- *ProxyFrame, serverconn *ServerConn) {
	if serverconn.output != nil {
		serverconn.output.processOpenFrame(ctx, f)
	}
}

func (c *Client) processOpenRsp(ctx context.Context, f *ProxyFrame, sendch chan<- *ProxyFrame, serverconn *ServerConn) {
	if serverconn.input != nil {
		serverconn.input.processOpenRspFrame(f)
	}
}
