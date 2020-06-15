package proxy

import (
	"errors"
	"github.com/esrrhs/go-engine/src/common"
	"github.com/esrrhs/go-engine/src/conn"
	"github.com/esrrhs/go-engine/src/group"
	"github.com/esrrhs/go-engine/src/loggo"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

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

	wg         *group.Group
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

	wg := group.NewGroup("Clent"+" "+fromaddr+" "+toaddr, nil, nil)

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

	wg.Go("Client connect"+" "+fromaddr+" "+toaddr, func() error {
		atomic.AddInt32(&gStateThreadNum.ThreadNum, 1)
		defer atomic.AddInt32(&gStateThreadNum.ThreadNum, -1)
		return c.connect(conn)
	})

	wg.Go("Client state"+" "+fromaddr+" "+toaddr, func() error {
		return showState(wg)
	})

	return c, nil
}

func (c *Client) Close() {
	c.wg.Stop()
	c.wg.Wait()
}

func (c *Client) connect(conn conn.Conn) error {

	for !c.wg.IsExit() {
		select {
		case <-c.wg.Done():
			return nil
		case <-time.After(time.Second):
			if c.serverconn == nil {
				targetconn, err := conn.Dial(c.server)
				if err != nil {
					loggo.Error("connect Dial fail: %s %s", c.server, err.Error())
					continue
				}
				c.serverconn = &ServerConn{ProxyConn: ProxyConn{conn: targetconn}}
				c.wg.Go("Client useServer"+" "+targetconn.Info(), func() error {
					atomic.AddInt32(&gStateThreadNum.ThreadNum, 1)
					defer atomic.AddInt32(&gStateThreadNum.ThreadNum, -1)
					return c.useServer(c.serverconn)
				})
			}
		}
	}
	return nil
}

func (c *Client) useServer(serverconn *ServerConn) error {

	loggo.Info("useServer %s", serverconn.conn.Info())

	sendch := common.NewChannel(c.config.MainBuffer)
	recvch := common.NewChannel(c.config.MainBuffer)

	serverconn.sendch = sendch
	serverconn.recvch = recvch

	wg := group.NewGroup("Client useServer"+" "+serverconn.conn.Info(), c.wg, func() {
		loggo.Info("group start exit %s", serverconn.conn.Info())
		serverconn.conn.Close()
		sendch.Close()
		recvch.Close()
		if serverconn.output != nil {
			serverconn.output.Close()
		}
		if serverconn.input != nil {
			serverconn.input.Close()
		}
		loggo.Info("group end exit %s", serverconn.conn.Info())
	})

	c.login(sendch)

	wg.Go("Client recvFrom"+" "+serverconn.conn.Info(), func() error {
		atomic.AddInt32(&gStateThreadNum.ThreadNum, 1)
		defer atomic.AddInt32(&gStateThreadNum.ThreadNum, -1)
		return recvFrom(wg, recvch, serverconn.conn, c.config.MaxMsgSize, c.config.Encrypt)
	})

	wg.Go("Client sendTo"+" "+serverconn.conn.Info(), func() error {
		atomic.AddInt32(&gStateThreadNum.ThreadNum, 1)
		defer atomic.AddInt32(&gStateThreadNum.ThreadNum, -1)
		return sendTo(wg, sendch, serverconn.conn, c.config.Compress, c.config.MaxMsgSize, c.config.Encrypt)
	})

	wg.Go("Client checkPingActive"+" "+serverconn.conn.Info(), func() error {
		atomic.AddInt32(&gStateThreadNum.ThreadNum, 1)
		defer atomic.AddInt32(&gStateThreadNum.ThreadNum, -1)
		return checkPingActive(wg, sendch, recvch, &serverconn.ProxyConn, c.config.EstablishedTimeout, c.config.PingInter, c.config.PingTimeoutInter, c.config.ShowPing)
	})

	wg.Go("Client checkNeedClose"+" "+serverconn.conn.Info(), func() error {
		atomic.AddInt32(&gStateThreadNum.ThreadNum, 1)
		defer atomic.AddInt32(&gStateThreadNum.ThreadNum, -1)
		return checkNeedClose(wg, &serverconn.ProxyConn)
	})

	wg.Go("Client process"+" "+serverconn.conn.Info(), func() error {
		atomic.AddInt32(&gStateThreadNum.ThreadNum, 1)
		defer atomic.AddInt32(&gStateThreadNum.ThreadNum, -1)
		return c.process(wg, sendch, recvch, serverconn)
	})

	wg.Wait()
	c.serverconn = nil
	loggo.Info("useServer close %s %s", c.server, serverconn.conn.Info())

	return nil
}

func (c *Client) login(sendch *common.Channel) {
	f := &ProxyFrame{}
	f.Type = FRAME_TYPE_LOGIN
	f.LoginFrame = &LoginFrame{}
	f.LoginFrame.Proxyproto = c.proxyproto
	f.LoginFrame.Clienttype = c.clienttype
	f.LoginFrame.Fromaddr = c.fromaddr
	f.LoginFrame.Toaddr = c.toaddr
	f.LoginFrame.Name = c.name
	f.LoginFrame.Key = c.config.Key

	sendch.Write(f)

	loggo.Info("start login %s %s", c.server, f.LoginFrame.String())
}

func (c *Client) process(wg *group.Group, sendch *common.Channel, recvch *common.Channel, serverconn *ServerConn) error {

	for !wg.IsExit() {
		select {
		case <-wg.Done():
			return nil
		case ff := <-recvch.Ch():
			if ff == nil {
				return nil
			}
			f := ff.(*ProxyFrame)
			switch f.Type {
			case FRAME_TYPE_LOGINRSP:
				c.processLoginRsp(wg, f, sendch, serverconn)

			case FRAME_TYPE_PING:
				processPing(f, sendch, &serverconn.ProxyConn)

			case FRAME_TYPE_PONG:
				processPong(f, sendch, &serverconn.ProxyConn, c.config.ShowPing)

			case FRAME_TYPE_DATA:
				c.processData(f, serverconn)

			case FRAME_TYPE_OPEN:
				c.processOpen(f, serverconn)

			case FRAME_TYPE_OPENRSP:
				c.processOpenRsp(f, serverconn)

			case FRAME_TYPE_CLOSE:
				c.processClose(f, serverconn)
			}
		}
	}
	return nil
}

func (c *Client) processLoginRsp(wg *group.Group, f *ProxyFrame, sendch *common.Channel, serverconn *ServerConn) {
	if !f.LoginRspFrame.Ret {
		serverconn.needclose = true
		loggo.Error("processLoginRsp fail %s %s", c.server, f.LoginRspFrame.Msg)
		return
	}

	loggo.Info("processLoginRsp ok %s", c.server)

	err := c.iniService(wg, serverconn)
	if err != nil {
		loggo.Error("processLoginRsp iniService fail %s %s", c.server, err)
		return
	}

	serverconn.established = true
}

func (c *Client) iniService(wg *group.Group, serverConn *ServerConn) error {
	switch c.clienttype {
	case CLIENT_TYPE_PROXY:
		input, err := NewInputer(wg, c.proxyproto.String(), c.fromaddr, c.clienttype, c.config, &serverConn.ProxyConn, c.toaddr)
		if err != nil {
			return err
		}
		serverConn.input = input
	case CLIENT_TYPE_REVERSE_PROXY:
		output, err := NewOutputer(wg, c.proxyproto.String(), c.clienttype, c.config, &serverConn.ProxyConn)
		if err != nil {
			return err
		}
		serverConn.output = output
	case CLIENT_TYPE_SOCKS5:
		input, err := NewSocks5Inputer(wg, c.proxyproto.String(), c.fromaddr, c.clienttype, c.config, &serverConn.ProxyConn)
		if err != nil {
			return err
		}
		serverConn.input = input
	case CLIENT_TYPE_REVERSE_SOCKS5:
		output, err := NewOutputer(wg, c.proxyproto.String(), c.clienttype, c.config, &serverConn.ProxyConn)
		if err != nil {
			return err
		}
		serverConn.output = output
	default:
		return errors.New("error CLIENT_TYPE " + strconv.Itoa(int(c.clienttype)))
	}
	return nil
}

func (c *Client) processData(f *ProxyFrame, serverconn *ServerConn) {
	if serverconn.input != nil {
		serverconn.input.processDataFrame(f)
	} else if serverconn.output != nil {
		serverconn.output.processDataFrame(f)
	}
}

func (c *Client) processOpen(f *ProxyFrame, serverconn *ServerConn) {
	if serverconn.output != nil {
		serverconn.output.processOpenFrame(f)
	}
}

func (c *Client) processOpenRsp(f *ProxyFrame, serverconn *ServerConn) {
	if serverconn.input != nil {
		serverconn.input.processOpenRspFrame(f)
	}
}

func (c *Client) processClose(f *ProxyFrame, serverconn *ServerConn) {
	if serverconn.input != nil {
		serverconn.input.processCloseFrame(f)
	} else if serverconn.output != nil {
		serverconn.output.processCloseFrame(f)
	}
}
