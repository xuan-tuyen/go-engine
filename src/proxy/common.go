package proxy

import (
	"encoding/binary"
	"errors"
	"github.com/esrrhs/go-engine/src/common"
	"github.com/esrrhs/go-engine/src/conn"
	"github.com/esrrhs/go-engine/src/group"
	"github.com/esrrhs/go-engine/src/loggo"
	"github.com/golang/protobuf/proto"
	"io"
	"strconv"
	"sync"
	"time"
)

type Config struct {
	MaxMsgSize         int // 消息最大长度
	MainBuffer         int // 主通道buffer最大长度
	ConnBuffer         int // 每个conn buffer最大长度
	EstablishedTimeout int // 主通道登录超时
	PingInter          int // 主通道ping间隔
	PingTimeoutInter   int // 主通道ping超时间隔
	ConnTimeout        int // 每个conn的不活跃超时时间
	ConnectTimeout     int // 每个conn的连接超时
	Proto              string
	Key                string
	Encrypt            string
	Compress           int
	ShowPing           bool
}

func DefaultConfig() *Config {
	return &Config{
		MaxMsgSize:         1024 * 1024,
		MainBuffer:         1024 * 1024,
		ConnBuffer:         1024,
		EstablishedTimeout: 10,
		PingInter:          1,
		PingTimeoutInter:   5,
		ConnTimeout:        300,
		ConnectTimeout:     10,
		Proto:              "tcp",
		Key:                "123456",
		Encrypt:            "",
		Compress:           0,
		ShowPing:           false,
	}
}

type ProxyConn struct {
	conn        conn.Conn
	established bool
	sendch      *common.Channel // *ProxyFrame
	recvch      *common.Channel // *ProxyFrame
	actived     int
	pinged      int
	id          string
	needclose   bool
}

func checkProxyFame(f *ProxyFrame) error {
	switch f.Type {
	case FRAME_TYPE_LOGIN:
		if f.LoginFrame == nil {
			return errors.New("LoginFrame nil")
		}
	case FRAME_TYPE_LOGINRSP:
		if f.LoginRspFrame == nil {
			return errors.New("LoginRspFrame nil")
		}
	case FRAME_TYPE_DATA:
		if f.DataFrame == nil {
			return errors.New("DataFrame nil")
		}
	case FRAME_TYPE_PING:
		if f.PingFrame == nil {
			return errors.New("PingFrame nil")
		}
	case FRAME_TYPE_PONG:
		if f.PongFrame == nil {
			return errors.New("PongFrame nil")
		}
	case FRAME_TYPE_OPEN:
		if f.OpenFrame == nil {
			return errors.New("OpenFrame nil")
		}
	case FRAME_TYPE_OPENRSP:
		if f.OpenRspFrame == nil {
			return errors.New("OpenRspFrame nil")
		}
	case FRAME_TYPE_CLOSE:
		if f.CloseFrame == nil {
			return errors.New("CloseFrame nil")
		}
	default:
		return errors.New("Type error")
	}

	return nil
}

func MarshalSrpFrame(f *ProxyFrame, compress int, encrpyt string) ([]byte, error) {

	err := checkProxyFame(f)
	if err != nil {
		return nil, err
	}

	if f.Type == FRAME_TYPE_DATA && compress > 0 && len(f.DataFrame.Data) > compress && !f.DataFrame.Compress {
		newb := common.CompressData(f.DataFrame.Data)
		if len(newb) < len(f.DataFrame.Data) {
			f.DataFrame.Data = newb
			f.DataFrame.Compress = true
		}
	}

	if f.Type == FRAME_TYPE_DATA && encrpyt != "" {
		newb, err := common.Rc4(encrpyt, f.DataFrame.Data)
		if err != nil {
			return nil, err
		}
		f.DataFrame.Data = newb
	}

	mb, err := proto.Marshal(f)
	if err != nil {
		return nil, err
	}
	return mb, err
}

func UnmarshalSrpFrame(b []byte, encrpyt string) (*ProxyFrame, error) {

	f := &ProxyFrame{}
	err := proto.Unmarshal(b, f)
	if err != nil {
		return nil, err
	}

	err = checkProxyFame(f)
	if err != nil {
		return nil, err
	}

	if f.Type == FRAME_TYPE_DATA && encrpyt != "" {
		newb, err := common.Rc4(encrpyt, f.DataFrame.Data)
		if err != nil {
			return nil, err
		}
		f.DataFrame.Data = newb
	}

	if f.Type == FRAME_TYPE_DATA && f.DataFrame.Compress {
		newb, err := common.DeCompressData(f.DataFrame.Data)
		if err != nil {
			return nil, err
		}
		f.DataFrame.Data = newb
		f.DataFrame.Compress = false
	}

	return f, nil
}

func recvFrom(wg *group.Group, recvch *common.Channel, conn conn.Conn, maxmsgsize int, encrypt string) error {

	bs := make([]byte, 4)
	ds := make([]byte, maxmsgsize)

	for {
		select {
		case <-wg.Done():
			return nil
		default:
			_, err := io.ReadFull(conn, bs)
			if err != nil {
				loggo.Error("recvFrom ReadFull fail: %s %s", conn.Info(), err.Error())
				return err
			}

			msglen := binary.LittleEndian.Uint32(bs)
			if msglen > uint32(maxmsgsize) || msglen <= 0 {
				loggo.Error("recvFrom len fail: %s %d", conn.Info(), msglen)
				return errors.New("msg len fail " + strconv.Itoa(int(msglen)))
			}

			_, err = io.ReadFull(conn, ds[0:msglen])
			if err != nil {
				loggo.Error("recvFrom ReadFull fail: %s %s", conn.Info(), err.Error())
				return err
			}

			f, err := UnmarshalSrpFrame(ds[0:msglen], encrypt)
			if err != nil {
				loggo.Error("recvFrom UnmarshalSrpFrame fail: %s %s", conn.Info(), err.Error())
				return err
			}

			recvch.Write(f)

			if f.Type != FRAME_TYPE_PING && f.Type != FRAME_TYPE_PONG && loggo.IsDebug() {
				loggo.Debug("recvFrom %s %s", conn.Info(), f.Type.String())
			}
		}
	}
}

func sendTo(wg *group.Group, sendch *common.Channel, conn conn.Conn, compress int, maxmsgsize int, encrypt string) error {

	bs := make([]byte, 4)

	for {
		select {
		case <-wg.Done():
			return nil
		case ff := <-sendch.Ch():
			f := ff.(*ProxyFrame)
			mb, err := MarshalSrpFrame(f, compress, encrypt)
			if err != nil {
				loggo.Error("sendTo MarshalSrpFrame fail: %s %s", conn.Info(), err.Error())
				return err
			}

			msglen := uint32(len(mb))
			if msglen > uint32(maxmsgsize) || msglen <= 0 {
				loggo.Error("sendTo len fail: %s %d", conn.Info(), msglen)
				return errors.New("msg len fail " + strconv.Itoa(int(msglen)))
			}

			binary.LittleEndian.PutUint32(bs, msglen)
			_, err = conn.Write(bs)
			if err != nil {
				loggo.Error("sendTo Write fail: %s %s", conn.Info(), err.Error())
				return err
			}

			_, err = conn.Write(mb)
			if err != nil {
				loggo.Error("sendTo Write fail: %s %s", conn.Info(), err.Error())
				return err
			}

			if f.Type != FRAME_TYPE_PING && f.Type != FRAME_TYPE_PONG && loggo.IsDebug() {
				loggo.Debug("sendTo %s %s", conn.Info(), f.Type.String())
			}
		}
	}
}

func recvFromSonny(wg *group.Group, recvch *common.Channel, conn conn.Conn, maxmsgsize int) error {

	ds := make([]byte, maxmsgsize)

	for {
		select {
		case <-wg.Done():
			return nil
		default:
			len, err := conn.Read(ds)
			if err != nil {
				loggo.Error("recvFromSonny Read fail: %s %s", conn.Info(), err.Error())
				return err
			}

			if len <= 0 {
				loggo.Error("recvFromSonny len error: %s %d", conn.Info(), len)
				return errors.New("len error " + strconv.Itoa(len))
			}

			f := &ProxyFrame{}
			f.Type = FRAME_TYPE_DATA
			f.DataFrame = &DataFrame{}
			f.DataFrame.Data = ds[0:len]
			f.DataFrame.Compress = false
			f.DataFrame.Crc = common.GetCrc32String(string(f.DataFrame.Data))

			recvch.Write(f)

			if loggo.IsDebug() {
				loggo.Debug("recvFromSonny %s %d %s", conn.Info(), len, f.DataFrame.Crc)
			}
		}
	}
}

func sendToSonny(wg *group.Group, sendch *common.Channel, conn conn.Conn) error {

	for {
		select {
		case <-wg.Done():
			return nil
		case ff := <-sendch.Ch():
			f := ff.(*ProxyFrame)
			if f.DataFrame.Compress {
				loggo.Error("sendToSonny Compress error: %s", conn.Info())
				return errors.New("msg compress error")
			}

			if len(f.DataFrame.Data) <= 0 {
				loggo.Error("sendToSonny len error: %s %d", conn.Info(), len(f.DataFrame.Data))
				return errors.New("len error " + strconv.Itoa(len(f.DataFrame.Data)))
			}

			if f.DataFrame.Crc != common.GetCrc32String(string(f.DataFrame.Data)) {
				loggo.Error("sendToSonny crc error: %s %d %s %s", conn.Info(), len(f.DataFrame.Data), f.DataFrame.Crc, common.GetCrc32String(string(f.DataFrame.Data)))
				return errors.New("crc error")
			}

			_, err := conn.Write(f.DataFrame.Data)
			if err != nil {
				loggo.Error("sendToSonny Write fail: %s %s", conn.Info(), err.Error())
				return err
			}

			if loggo.IsDebug() {
				loggo.Debug("sendToSonny %s %d %s", conn.Info(), len(f.DataFrame.Data), f.DataFrame.Crc)
			}
		}
	}
}

func checkPingActive(wg *group.Group, sendch *common.Channel, recvch *common.Channel, proxyconn *ProxyConn,
	estimeout int, pinginter int, pingintertimeout int, showping bool) error {

	n := 0
	select {
	case <-wg.Done():
		return nil
	case <-time.After(time.Second):
		n++
		if !proxyconn.established {
			if n > estimeout {
				loggo.Error("checkPingActive established timeout %s", proxyconn.conn.Info())
				proxyconn.conn.Close()
				return errors.New("established timeout")
			}
		} else {
			break
		}
	}

	for {
		select {
		case <-wg.Done():
			return nil
		case <-time.After(time.Duration(pinginter) * time.Second):
			if proxyconn.pinged > pingintertimeout {
				loggo.Error("checkPingActive ping pong timeout %s", proxyconn.conn.Info())
				proxyconn.conn.Close()
				return errors.New("ping pong timeout")
			}

			f := &ProxyFrame{}
			f.Type = FRAME_TYPE_PING
			f.PingFrame = &PingFrame{}
			f.PingFrame.Time = time.Now().UnixNano()

			sendch.Write(f)

			proxyconn.pinged++
			if showping {
				loggo.Info("ping %s", proxyconn.conn.Info())
			}
		}
	}
}

func checkNeedClose(wg *group.Group, proxyconn *ProxyConn) error {

	for {
		select {
		case <-wg.Done():
			return nil
		case <-time.After(time.Second):
			if proxyconn.needclose {
				loggo.Error("checkNeedClose needclose %s", proxyconn.conn.Info())
				proxyconn.conn.Close()
				return errors.New("needclose")
			}
		}
	}
}

func processPing(f *ProxyFrame, sendch *common.Channel, proxyconn *ProxyConn) {
	rf := &ProxyFrame{}
	rf.Type = FRAME_TYPE_PONG
	rf.PongFrame = &PongFrame{}
	rf.PongFrame.Time = f.PingFrame.Time
	sendch.Write(rf)
}

func processPong(f *ProxyFrame, sendch *common.Channel, proxyconn *ProxyConn, showping bool) {
	elapse := time.Duration(time.Now().UnixNano() - f.PongFrame.Time)
	proxyconn.pinged = 0
	if showping {
		loggo.Info("pong %s %s", proxyconn.conn.Info(), elapse.String())
	}
}

func checkSonnyActive(wg *group.Group, proxyconn *ProxyConn, estimeout int, timeout int) error {

	n := 0
	select {
	case <-wg.Done():
		return nil
	case <-time.After(time.Second):
		n++
		if !proxyconn.established {
			if n > estimeout {
				loggo.Error("checkSonnyActive established timeout %s", proxyconn.conn.Info())
				proxyconn.conn.Close()
				return errors.New("established timeout")
			}
		} else {
			break
		}
	}

	for {
		select {
		case <-wg.Done():
			return nil
		case <-time.After(time.Duration(timeout) * time.Second):
			if proxyconn.actived == 0 {
				loggo.Error("checkSonnyActive timeout %s", proxyconn.conn.Info())
				proxyconn.conn.Close()
				return errors.New("conn timeout")
			}
			proxyconn.actived = 0
		}
	}
}

func copySonnyRecv(wg *group.Group, recvch *common.Channel, proxyConn *ProxyConn, father *ProxyConn) error {

	for {
		select {
		case <-wg.Done():
			return nil
		case ff := <-recvch.Ch():
			f := ff.(*ProxyFrame)
			if f.Type != FRAME_TYPE_DATA {
				loggo.Error("copySonnyRecv type error %s %d", proxyConn.conn.Info(), f.Type)
				return errors.New("conn type error")
			}
			f.DataFrame.Id = proxyConn.id
			proxyConn.actived++

			father.sendch.Write(f)

			loggo.Debug("copySonnyRecv %s %d", proxyConn.id, len(f.DataFrame.Data))
		}
	}
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

	wg.Go(func() error {
		return input.listen()
	})

	loggo.Info("NewInputer ok %s", addr)

	return input, nil
}

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

func (i *Inputer) Close() {
	i.listenconn.Close()
}

func (i *Inputer) processDataFrame(f *ProxyFrame) {
	id := f.DataFrame.Id
	v, ok := i.sonny.Load(id)
	if !ok {
		loggo.Error("Inputer processDataFrame no sonnny %s %d", id, len(f.DataFrame.Data))
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
	sonny.needclose = true
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
			go i.processProxyConn(proxyconn)
		}
	}
}

func (i *Inputer) processProxyConn(proxyConn *ProxyConn) {

	proxyConn.id = common.UniqueId()

	loggo.Info("Inputer processProxyConn start %s %s", proxyConn.id, proxyConn.conn.Info())

	_, loaded := i.sonny.LoadOrStore(proxyConn.id, proxyConn)
	if loaded {
		loggo.Error("Inputer processProxyConn LoadOrStore fail %s", proxyConn.id)
		proxyConn.conn.Close()
		return
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

	wg.Go(func() error {
		return recvFromSonny(wg, recvch, proxyConn.conn, i.config.MaxMsgSize)
	})

	wg.Go(func() error {
		return sendToSonny(wg, sendch, proxyConn.conn)
	})

	wg.Go(func() error {
		return checkSonnyActive(wg, proxyConn, i.config.EstablishedTimeout, i.config.ConnTimeout)
	})

	wg.Go(func() error {
		return checkNeedClose(wg, proxyConn)
	})

	wg.Go(func() error {
		return copySonnyRecv(wg, recvch, proxyConn, i.father)
	})

	wg.Wait()
	i.sonny.Delete(proxyConn.id)

	closeRemoteConn(proxyConn, i.father)

	loggo.Info("Inputer processProxyConn end %s %s", proxyConn.id, proxyConn.conn.Info())
}

func (i *Inputer) openConn(proxyConn *ProxyConn) {
	f := &ProxyFrame{}
	f.Type = FRAME_TYPE_OPEN
	f.OpenFrame = &OpenConnFrame{}
	f.OpenFrame.Id = proxyConn.id

	i.father.sendch.Write(f)
	loggo.Info("Inputer openConn %s", proxyConn.id)
}

func closeRemoteConn(proxyConn *ProxyConn, father *ProxyConn) {
	f := &ProxyFrame{}
	f.Type = FRAME_TYPE_CLOSE
	f.CloseFrame = &CloseFrame{}
	f.CloseFrame.Id = proxyConn.id

	father.sendch.Write(f)
	loggo.Info("closeConn %s", proxyConn.id)
}

func NewOutputer(wg *group.Group, proto string, addr string, clienttype CLIENT_TYPE, config *Config, father *ProxyConn) (*Outputer, error) {
	conn, err := conn.NewConn(proto)
	if conn == nil {
		return nil, err
	}

	output := &Outputer{
		clienttype: clienttype,
		config:     config,
		conn:       conn,
		proto:      proto,
		addr:       addr,
		father:     father,
		fwg:        wg,
	}

	loggo.Info("NewOutputer ok %s", addr)

	return output, nil
}

type Outputer struct {
	clienttype CLIENT_TYPE
	config     *Config
	proto      string
	addr       string
	father     *ProxyConn
	fwg        *group.Group

	conn  conn.Conn
	sonny sync.Map
}

func (o *Outputer) Close() {
	o.conn.Close()
}

func (o *Outputer) processDataFrame(f *ProxyFrame) {
	id := f.DataFrame.Id
	v, ok := o.sonny.Load(id)
	if !ok {
		loggo.Error("Outputer processDataFrame no sonnny %s %d", f.DataFrame.Id, len(f.DataFrame.Data))
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
	sonny.needclose = true
}

func (o *Outputer) processOpenFrame(f *ProxyFrame) {

	id := f.OpenFrame.Id

	rf := &ProxyFrame{}
	rf.Type = FRAME_TYPE_OPENRSP
	rf.OpenRspFrame = &OpenConnRspFrame{}
	rf.OpenRspFrame.Id = id

	conn, err := o.conn.Dial(o.addr)
	if err != nil {
		rf.OpenRspFrame.Ret = false
		rf.OpenRspFrame.Msg = "Dial fail"
		o.father.sendch.Write(rf)
		loggo.Error("Outputer processOpenFrame Dial fail %s %s", o.addr, err.Error())
		return
	}

	proxyconn := &ProxyConn{id: id, conn: conn, established: true}
	_, loaded := o.sonny.LoadOrStore(proxyconn.id, proxyconn)
	if loaded {
		loggo.Error("Outputer processOpenFrame LoadOrStore fail %s %s", o.addr, id)
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

	go o.processProxyConn(proxyconn)
}

func (o *Outputer) processProxyConn(proxyConn *ProxyConn) {

	loggo.Info("Outputer processProxyConn start %s %s", proxyConn.id, proxyConn.conn.Info())

	sendch := proxyConn.sendch
	recvch := proxyConn.recvch

	wg := group.NewGroup(o.fwg, func() {
		proxyConn.conn.Close()
		sendch.Close()
		recvch.Close()
	})

	wg.Go(func() error {
		return recvFromSonny(wg, recvch, proxyConn.conn, o.config.MaxMsgSize)
	})

	wg.Go(func() error {
		return sendToSonny(wg, sendch, proxyConn.conn)
	})

	wg.Go(func() error {
		return checkSonnyActive(wg, proxyConn, o.config.EstablishedTimeout, o.config.ConnTimeout)
	})

	wg.Go(func() error {
		return checkNeedClose(wg, proxyConn)
	})

	wg.Go(func() error {
		return copySonnyRecv(wg, recvch, proxyConn, o.father)
	})

	wg.Wait()
	o.sonny.Delete(proxyConn.id)

	closeRemoteConn(proxyConn, o.father)

	loggo.Info("Outputer processProxyConn end %s %s", proxyConn.id, proxyConn.conn.Info())
}
