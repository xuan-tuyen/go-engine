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
	"sync/atomic"
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
	Username           string
	Password           string
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
		Username:           "",
		Password:           "",
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
			if loggo.IsDebug() {
				loggo.Debug("MarshalSrpFrame Compress from %d %d", len(f.DataFrame.Data), len(newb))
			}
			f.DataFrame.Data = newb
			f.DataFrame.Compress = true
		}
	}

	if f.Type == FRAME_TYPE_DATA && encrpyt != "" {
		newb, err := common.Rc4(encrpyt, f.DataFrame.Data)
		if err != nil {
			return nil, err
		}
		if loggo.IsDebug() {
			loggo.Debug("MarshalSrpFrame Rc4 from %s %s", common.GetCrc32(f.DataFrame.Data), common.GetCrc32(newb))
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
		if loggo.IsDebug() {
			loggo.Debug("UnmarshalSrpFrame Rc4 from %s %s", common.GetCrc32(f.DataFrame.Data), common.GetCrc32(newb))
		}
		f.DataFrame.Data = newb
	}

	if f.Type == FRAME_TYPE_DATA && f.DataFrame.Compress {
		newb, err := common.DeCompressData(f.DataFrame.Data)
		if err != nil {
			return nil, err
		}
		if loggo.IsDebug() {
			loggo.Debug("UnmarshalSrpFrame Compress from %d %d", len(f.DataFrame.Data), len(newb))
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
			if loggo.IsDebug() {
				loggo.Debug("recvFrom start ReadFull len %s", conn.Info())
			}
			_, err := io.ReadFull(conn, bs)
			if err != nil {
				loggo.Info("recvFrom ReadFull fail: %s %s", conn.Info(), err.Error())
				return err
			}

			msglen := binary.LittleEndian.Uint32(bs)
			if msglen > uint32(maxmsgsize) || msglen <= 0 {
				loggo.Error("recvFrom len fail: %s %d", conn.Info(), msglen)
				return errors.New("msg len fail " + strconv.Itoa(int(msglen)))
			}

			if loggo.IsDebug() {
				loggo.Debug("recvFrom start ReadFull body %s %d", conn.Info(), msglen)
			}
			_, err = io.ReadFull(conn, ds[0:msglen])
			if err != nil {
				loggo.Info("recvFrom ReadFull fail: %s %s", conn.Info(), err.Error())
				return err
			}

			f, err := UnmarshalSrpFrame(ds[0:msglen], encrypt)
			if err != nil {
				loggo.Error("recvFrom UnmarshalSrpFrame fail: %s %s", conn.Info(), err.Error())
				return err
			}

			if loggo.IsDebug() {
				loggo.Debug("recvFrom start Write %s", conn.Info())
			}
			recvch.Write(f)

			if f.Type != FRAME_TYPE_PING && f.Type != FRAME_TYPE_PONG && loggo.IsDebug() {
				loggo.Debug("recvFrom %s %s", conn.Info(), f.Type.String())
				if f.Type == FRAME_TYPE_DATA {
					if common.GetCrc32(f.DataFrame.Data) != f.DataFrame.Crc {
						loggo.Error("recvFrom crc error %s %s %s %p", conn.Info(), common.GetCrc32(f.DataFrame.Data), f.DataFrame.Crc, f)
						return errors.New("conn crc error")
					}
				}
			}

			atomic.AddInt32(&gState.MainRecvNum, 1)
			atomic.AddInt32(&gState.MainRecvSize, int32(msglen)+4)
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
			if ff == nil {
				return nil
			}
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

			if loggo.IsDebug() {
				loggo.Debug("sendTo start Write len %s", conn.Info())
			}
			binary.LittleEndian.PutUint32(bs, msglen)
			_, err = conn.Write(bs)
			if err != nil {
				loggo.Info("sendTo Write fail: %s %s", conn.Info(), err.Error())
				return err
			}

			if loggo.IsDebug() {
				loggo.Debug("sendTo start Write body %s %d", conn.Info(), msglen)
			}
			n, err := conn.Write(mb)
			if err != nil {
				loggo.Info("sendTo Write fail: %s %s", conn.Info(), err.Error())
				return err
			}

			if n != len(mb) {
				loggo.Error("sendTo Write len fail: %s %d %d", conn.Info(), n, len(mb))
				return errors.New("len error")
			}

			if f.Type != FRAME_TYPE_PING && f.Type != FRAME_TYPE_PONG && loggo.IsDebug() {
				loggo.Debug("sendTo %s %s", conn.Info(), f.Type.String())
				if f.Type == FRAME_TYPE_DATA {
					if common.GetCrc32(f.DataFrame.Data) != f.DataFrame.Crc {
						loggo.Error("sendTo crc error %s %s %s %p", conn.Info(), common.GetCrc32(f.DataFrame.Data), f.DataFrame.Crc, f)
						return errors.New("conn crc error")
					}
				}
			}

			atomic.AddInt32(&gState.MainSendNum, 1)
			atomic.AddInt32(&gState.MainSendSize, int32(msglen)+4)
		}
	}
}

const (
	MAX_INDEX = 1024
)

func recvFromSonny(wg *group.Group, recvch *common.Channel, conn conn.Conn, maxmsgsize int) error {

	ds := make([]byte, maxmsgsize)

	index := int32(0)
	for {
		select {
		case <-wg.Done():
			return nil
		default:
			msglen, err := conn.Read(ds)
			if err != nil {
				loggo.Info("recvFromSonny Read fail: %s %s", conn.Info(), err.Error())
				return err
			}

			if msglen <= 0 {
				loggo.Error("recvFromSonny len error: %s %d", conn.Info(), msglen)
				return errors.New("len error " + strconv.Itoa(msglen))
			}

			f := &ProxyFrame{}
			f.Type = FRAME_TYPE_DATA
			f.DataFrame = &DataFrame{}
			f.DataFrame.Data = make([]byte, msglen)
			copy(f.DataFrame.Data, ds[0:msglen])
			f.DataFrame.Compress = false
			f.DataFrame.Crc = common.GetCrc32(f.DataFrame.Data)
			index++
			f.DataFrame.Index = index % MAX_INDEX

			recvch.Write(f)

			if loggo.IsDebug() {
				loggo.Debug("recvFromSonny %s %d %s %d %p", conn.Info(), msglen, f.DataFrame.Crc, f.DataFrame.Index, f)
			}

			atomic.AddInt32(&gState.RecvNum, 1)
			atomic.AddInt32(&gState.RecvSize, int32(len(f.DataFrame.Data)))
		}
	}
}

func sendToSonny(wg *group.Group, sendch *common.Channel, conn conn.Conn) error {

	index := int32(0)
	for {
		select {
		case <-wg.Done():
			return nil
		case ff := <-sendch.Ch():
			if ff == nil {
				return nil
			}
			f := ff.(*ProxyFrame)
			if f.Type == FRAME_TYPE_CLOSE {
				loggo.Info("sendToSonny close by remote: %s", conn.Info())
				return errors.New("close by remote")
			}
			if f.DataFrame.Compress {
				loggo.Error("sendToSonny Compress error: %s", conn.Info())
				return errors.New("msg compress error")
			}

			if len(f.DataFrame.Data) <= 0 {
				loggo.Error("sendToSonny len error: %s %d", conn.Info(), len(f.DataFrame.Data))
				return errors.New("len error " + strconv.Itoa(len(f.DataFrame.Data)))
			}

			if f.DataFrame.Crc != common.GetCrc32(f.DataFrame.Data) {
				loggo.Error("sendToSonny crc error: %s %d %s %s", conn.Info(), len(f.DataFrame.Data), f.DataFrame.Crc, common.GetCrc32(f.DataFrame.Data))
				return errors.New("crc error")
			}

			index++
			index = index % MAX_INDEX
			if f.DataFrame.Index != index {
				loggo.Error("sendToSonny index error: %s %d %d %d", conn.Info(), len(f.DataFrame.Data), f.DataFrame.Index, index)
				return errors.New("index error")
			}

			n, err := conn.Write(f.DataFrame.Data)
			if err != nil {
				loggo.Info("sendToSonny Write fail: %s %s", conn.Info(), err.Error())
				return err
			}

			if n != len(f.DataFrame.Data) {
				loggo.Error("sendToSonny Write len fail: %s %d %d", conn.Info(), n, len(f.DataFrame.Data))
				return errors.New("len error")
			}

			if loggo.IsDebug() {
				loggo.Debug("sendToSonny %s %d %s %d", conn.Info(), len(f.DataFrame.Data), f.DataFrame.Crc, f.DataFrame.Index)
			}

			atomic.AddInt32(&gState.SendNum, 1)
			atomic.AddInt32(&gState.SendSize, int32(len(f.DataFrame.Data)))
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
				return errors.New("established timeout")
			}
		} else {
			break
		}
	}

	n = 0
	for {
		select {
		case <-wg.Done():
			return nil
		case <-time.After(time.Second):
			n++
			if n > timeout {
				if proxyconn.actived == 0 {
					loggo.Error("checkSonnyActive timeout %s", proxyconn.conn.Info())
					return errors.New("conn timeout")
				}
				proxyconn.actived = 0
			}
		}
	}
}

func copySonnyRecv(wg *group.Group, recvch *common.Channel, proxyConn *ProxyConn, father *ProxyConn) error {

	for {
		select {
		case <-wg.Done():
			return nil
		case ff := <-recvch.Ch():
			if ff == nil {
				return nil
			}
			f := ff.(*ProxyFrame)
			if f.Type != FRAME_TYPE_DATA {
				loggo.Error("copySonnyRecv type error %s %d", proxyConn.conn.Info(), f.Type)
				return errors.New("conn type error")
			}
			if f.DataFrame.Compress {
				loggo.Error("copySonnyRecv compress error %s %d", proxyConn.conn.Info(), f.Type)
				return errors.New("conn compress error")
			}
			if loggo.IsDebug() {
				if common.GetCrc32(f.DataFrame.Data) != f.DataFrame.Crc {
					loggo.Error("copySonnyRecv crc error %s %s %s", proxyConn.conn.Info(), common.GetCrc32(f.DataFrame.Data), f.DataFrame.Crc)
					return errors.New("conn crc error")
				}
			}
			f.DataFrame.Id = proxyConn.id
			proxyConn.actived++

			father.sendch.Write(f)

			loggo.Debug("copySonnyRecv %s %d %s %p", proxyConn.id, len(f.DataFrame.Data), f.DataFrame.Crc, f)
		}
	}
}

func closeRemoteConn(proxyConn *ProxyConn, father *ProxyConn) {
	f := &ProxyFrame{}
	f.Type = FRAME_TYPE_CLOSE
	f.CloseFrame = &CloseFrame{}
	f.CloseFrame.Id = proxyConn.id

	father.sendch.Write(f)
	loggo.Info("closeConn %s", proxyConn.id)
}

type State struct {
	ThreadNum    int32
	MainRecvNum  int32
	MainSendNum  int32
	MainRecvSize int32
	MainSendSize int32
	RecvNum      int32
	SendNum      int32
	RecvSize     int32
	SendSize     int32
}

var gState State

func showState(wg *group.Group) error {
	n := 0
	for {
		select {
		case <-wg.Done():
			return nil
		case <-time.After(time.Second):
			n++
			if n > 60 {
				MainRecvNum := gState.MainRecvNum
				MainSendNum := gState.MainSendNum
				MainRecvSize := gState.MainRecvSize
				MainSendSize := gState.MainSendSize
				RecvNum := gState.RecvNum
				SendNum := gState.SendNum
				RecvSize := gState.RecvSize
				SendSize := gState.SendSize
				loggo.Info("showState\n%s", common.StructToTable(&gState))
				atomic.AddInt32(&gState.MainRecvNum, -MainRecvNum)
				atomic.AddInt32(&gState.MainSendNum, -MainSendNum)
				atomic.AddInt32(&gState.MainRecvSize, -MainRecvSize)
				atomic.AddInt32(&gState.MainSendSize, -MainSendSize)
				atomic.AddInt32(&gState.RecvNum, -RecvNum)
				atomic.AddInt32(&gState.SendNum, -SendNum)
				atomic.AddInt32(&gState.RecvSize, -RecvSize)
				atomic.AddInt32(&gState.SendSize, -SendSize)
				n = 0
			}
		}
	}
}
