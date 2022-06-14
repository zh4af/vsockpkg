package vsockserver

import (
	"encoding/binary"
	"io"
	"net"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/golang/protobuf/proto"
	"github.com/google/uuid"
	"github.com/zh4af/vsockpkg/msgbuf"
	"github.com/zh4af/vsockpkg/utils"
)

type ConnHandler struct {
	connId     string
	conn       net.Conn
	remoteAddr string

	closed chan struct{}

	readMsg  chan *msgbuf.MsgBody
	writeMsg chan *msgbuf.MsgBody

	log *logrus.Entry
}

func NewConnHandler(conn net.Conn) *ConnHandler {
	handler := &ConnHandler{
		connId:     uuid.New().String(),
		conn:       conn,
		remoteAddr: conn.RemoteAddr().String(),
		closed:     make(chan struct{}),
		readMsg:    make(chan *msgbuf.MsgBody, 100000),
		writeMsg:   make(chan *msgbuf.MsgBody, 100000),
	}
	handler.log = logrus.WithFields(logrus.Fields{
		"connId":     handler.connId,
		"remoteAddr": handler.remoteAddr,
	})
	return handler
}

func (handler *ConnHandler) Close() {
	select {
	case _, ok := <-handler.closed:
		if !ok { // chan closed
			return
		}
	default:
		close(handler.closed) // 作为是否closed的标志一定要首先close
		// handler.conn.Close() // 在外层的tcpserver关闭

		if len(handler.writeMsg) > 0 {
			for v := range handler.writeMsg {
				if v != nil {
					v.Clear()
				}
				if len(handler.writeMsg) <= 0 {
					break
				}
			}
		}
		close(handler.readMsg)
		close(handler.writeMsg)
		logrus.WithFields(logrus.Fields{
			"connId":     handler.connId,
			"remoteAddr": handler.remoteAddr,
		}).Debugf("ConnHandler exist release resource")
	}
}

func (handler *ConnHandler) ServerLoop() {
	go handler.writeLoop()
	go handler.readLoop()
	go handler.onMessage()

	var err error
	pingTicker := time.NewTicker(5 * time.Second)
	defer pingTicker.Stop()
	for {
		select {
		case _, ok := <-handler.closed:
			if !ok { // chan closed
				return
			}
		case <-pingTicker.C:
			if err = handler.sendPing(); err != nil {
				handler.log.WithError(err).Error("send ping err, tcp conn disconnected")
				handler.Close()
				return
			}
		}
	}
}

func (handler *ConnHandler) sendPack(body []byte, msgType uint16, msgId uint64) error {
	var msg = msgbuf.NewMsgBody(msgType, msgId, body)
	msg.Pack()

	select {
	case _, ok := <-handler.closed:
		if !ok { // chan closed
			return nil
		}
	default:
		handler.writeMsg <- msg
	}
	return nil
}

func (handler *ConnHandler) sendPing() error {
	return handler.sendPack(nil, MSG_TYPE_RESERVE_PING, utils.GenerateRandomUint64())
}

func (handler *ConnHandler) writeLoop() {
	for {
		select {
		case _, ok := <-handler.closed:
			if !ok {
				return
			}
		case msg, ok := <-handler.writeMsg:
			if !ok { // chan closed
				return
			}
			func() {
				defer msg.Clear()
				writeLen, err := handler.conn.Write(msg.Buffer().Bytes())
				if err != nil || writeLen != msg.Buffer().Len() {
					handler.log.WithFields(logrus.Fields{
						"err":      err,
						"writeLen": writeLen,
						"msgLen":   msg.Buffer().Len(),
					}).Error("write msg to tcp conn err")

					handler.Close()
					return
				}
			}()
		}
	}
}

func (handler *ConnHandler) readLoop() {
	defer handler.Close()

	var nRead, n int
	var bodyLen uint16
	var err error

	for {
		n = 0
		nRead = 0
		var msg = msgbuf.NewMsgBody(0, 0, nil) // write all msg info into msg.buffer
		msgbuf.BufferGrowNByte(msg.Buffer(), msgbuf.Msg_Header_Bytes)
		for {
			n, err = handler.conn.Read(msg.Buffer().Bytes()[nRead:msgbuf.Msg_Header_Bytes])
			if err != nil && err != io.EOF {
				handler.log.WithError(err).Error("read length from remote tcp err")
				return
			}
			nRead += n
			if err != nil || nRead < msgbuf.Msg_Header_Bytes {
				continue
			}
			bodyLen = binary.BigEndian.Uint16(msg.Buffer().Bytes()[msgbuf.Msg_Header_MsgType_Bytes : msgbuf.Msg_Header_MsgType_Bytes+msgbuf.Msg_Header_MsgLen_Bytes])
			break
		}

		if bodyLen > 0 {
			msgbuf.BufferGrowNByte(msg.Buffer(), int(bodyLen))
			n = 0
			nRead = 0
			for {
				n, err = handler.conn.Read(msg.Buffer().Bytes()[msgbuf.Msg_Header_Bytes+nRead:])
				if err != nil && err != io.EOF {
					handler.log.WithFields(logrus.Fields{
						"err":     err,
						"readLen": n,
					}).Error("read body from remote tcp err")
					return
				}
				nRead += n
				if err != nil || nRead < int(bodyLen) {
					continue
				}
				break
			}
		}

		msg.UnPack()
		// handler.onMessage(msg)
		select {
		case _, ok := <-handler.closed:
			if !ok {
				return
			}
		case handler.readMsg <- msg:
		}
	}
}

func (handler *ConnHandler) onMessage() {
	for {
		select {
		case _, ok := <-handler.closed:
			if !ok {
				return
			}
		case msg := <-handler.readMsg:
			if msg != nil {
				switch msg.MsgType() {
				case MSG_TYPE_RESERVE_PING:
					handler.onPing(msg)
				case MSG_TYPE_RESERVE_PONG:
					handler.onPong(msg)
				default:
					handler.onHandle(msg)
				}
				msg.Clear()
			}
		}
	}
}

func (handler *ConnHandler) onPing(msg *msgbuf.MsgBody) {
	handler.sendPack(nil, MSG_TYPE_RESERVE_PONG, msg.MsgId())
	return
}

func (handler *ConnHandler) onPong(msg *msgbuf.MsgBody) {
	// handler.log.Debug("onPong")
	return
}

func (handler *ConnHandler) onHandle(msg *msgbuf.MsgBody) {
	var bodyRet []byte
	f, err := GetServHandler(msg.MsgType())
	if f == nil && err != nil {
		logrus.WithFields(logrus.Fields{"err": err, "msgType": msg.MsgType()}).Error("onHandle err")
		bodyRet = []byte(err.Error())
		handler.sendPack(bodyRet, msg.MsgType(), msg.MsgId())
		return
	}
	retMsg, err := f(msg)
	if err != nil {
		handler.log.WithError(err).Error("handler err")
		bodyRet = []byte(err.Error())
	} else {
		bodyRet, err = proto.Marshal(retMsg)
		if err != nil {
			handler.log.WithError(err).Error("marshal message err")
			bodyRet = []byte(err.Error())
		}
	}

	handler.sendPack(bodyRet, msg.MsgType(), msg.MsgId())
}
