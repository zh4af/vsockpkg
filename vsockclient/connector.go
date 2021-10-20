package vsockclient

import (
	"encoding/binary"
	"errors"
	"io"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/mdlayher/vsock"
	"github.com/zh4af/vsockpkg/msgbuf"
	"github.com/zh4af/vsockpkg/utils"
)

var (
	MSG_TYPE_RESERVE_PING uint16 = 10001
	MSG_TYPE_RESERVE_PONG uint16 = 10002
)

var (
	Err_Vsock_Client_Exit      = errors.New("vsock client exit")
	Err_Vsock_Client_Dial      = errors.New("vsock client dial failed")
	Err_Vsock_Client_Not_Ready = errors.New("vsock client not ready")
)

type VsockClient struct {
	// vm socket server cid (run in enclave)
	enclave_cid uint32
	// vm socke port to connect
	port uint32

	conn     *vsock.Conn
	closed   chan struct{}
	writeMsg chan *msgbuf.MsgBody
	log      *logrus.Entry

	ready bool

	onConnected func() error
}

var DefaultVsockClient *VsockClient

/*
  enclave_cid: vm socket server cid (run in enclave)
  port: vm socke port to connect
  onConnected: func which be invoked when connection established
*/
func NewVsockClient(enclave_cid, port uint32, onConnected func() error) *VsockClient {
	client := &VsockClient{
		enclave_cid: enclave_cid,
		port:        port,
		closed:      make(chan struct{}),
		writeMsg:    make(chan *msgbuf.MsgBody, 100),
		onConnected: onConnected,
	}
	client.log = logrus.WithFields(logrus.Fields{"port": port})

	DefaultVsockClient = client
	go client.Start()
	return client
}

func (cli *VsockClient) ReInit() {
	cli.closed = make(chan struct{})
	cli.writeMsg = make(chan *msgbuf.MsgBody, 100)
}

func (cli *VsockClient) Start() {
	go func() {
		var err error
		for {
			err = cli.connectAndServe() // reconnect if failed
			switch err {
			case Err_Vsock_Client_Dial:
				time.Sleep(time.Second * 5)
			case Err_Vsock_Client_Exit:
				time.Sleep(time.Second)
			default:
			}
			cli.ReInit()
		}
	}()
}

func (cli *VsockClient) connectAndServe() error {
	conn, err := vsock.Dial(cli.enclave_cid, cli.port)
	if err != nil {
		cli.log.WithError(err).Error("vsock dial failed")
		return Err_Vsock_Client_Dial
	}

	cli.conn = conn
	cli.serverLoop() // serve loop until exception
	cli.log.WithError(Err_Vsock_Client_Exit).Error("client connectAndServe exit")
	return Err_Vsock_Client_Exit
}

func (cli *VsockClient) Close() error {
	var err error
	cli.ready = false
	err = cli.conn.Close()
	select {
	case _, ok := <-cli.closed:
		if !ok { // chan closed
			return nil
		}
	default:
		close(cli.closed)
		if len(cli.writeMsg) > 0 {
			for v := range cli.writeMsg {
				if v != nil {
					v.Clear()
				}
				if len(cli.writeMsg) <= 0 {
					break
				}
			}
		}
		close(cli.writeMsg)
	}
	return err
}

func (cli *VsockClient) serverLoop() {
	go cli.writeLoop()
	go cli.readLoop()

	var err error

	cli.ready = true

	// set credentials when reconnect to vsock server
	err = cli.onConnected()
	if err != nil {
		return
	}

	pingTicker := time.NewTicker(5 * time.Second)
	defer pingTicker.Stop()
	for {
		select {
		case _, ok := <-cli.closed:
			if !ok { // chan closed
				return
			}
		case <-pingTicker.C:
			if err = cli.sendPing(); err != nil {
				cli.log.WithError(err).Error("send ping err, tcp conn disconnected")
				cli.Close()
				return
			}
		}
	}
}

func (cli *VsockClient) sendPack(body []byte, msgType uint16, msgId uint64) error {
	var msg = msgbuf.NewMsgBody(msgType, msgId, body)
	msg.Pack()

	select {
	case _, ok := <-cli.closed:
		if !ok { // chan closed
			return Err_Vsock_Client_Exit
		}
	default:
		cli.writeMsg <- msg
	}
	return nil
}

func (cli *VsockClient) sendPing() error {
	return cli.sendPack(nil, MSG_TYPE_RESERVE_PING, utils.GenerateRandomUint64())
}

func (cli *VsockClient) writeLoop() {
	for {
		select {
		case _, ok := <-cli.closed:
			if !ok {
				return
			}
		case msg, ok := <-cli.writeMsg:
			if !ok { // chan closed
				return
			}
			func() {
				defer msg.Clear()
				writeLen, err := cli.conn.Write(msg.Buffer().Bytes())
				if err != nil || writeLen != msg.Buffer().Len() {
					cli.log.WithFields(logrus.Fields{
						"err":      err,
						"writeLen": writeLen,
						"msgLen":   msg.Buffer().Len(),
					}).Error("write msg to tcp conn err")

					cli.Close()
					return
				}
			}()
		}
	}
}

func (cli *VsockClient) readLoop() {
	defer cli.Close()

	var nRead, n int
	var bodyLen uint16
	var err error

	for {
		n = 0
		nRead = 0
		var msg = msgbuf.NewMsgBody(0, 0, nil) // write all msg info into msg.buffer
		msgbuf.BufferGrowNByte(msg.Buffer(), msgbuf.Msg_Header_Bytes)
		for {
			n, err = cli.conn.Read(msg.Buffer().Bytes()[nRead:msgbuf.Msg_Header_Bytes])
			if err != nil && err != io.EOF {
				cli.log.WithError(err).Error("read header from remote tcp err")
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
				n, err = cli.conn.Read(msg.Buffer().Bytes()[msgbuf.Msg_Header_Bytes+nRead:])
				if err != nil && err != io.EOF {
					cli.log.WithFields(logrus.Fields{
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
		cli.onMessage(msg)
	}
}

func (cli *VsockClient) onMessage(msg *msgbuf.MsgBody) {
	defer func() {
		utils.RecoverStack()
	}()

	switch msg.MsgType() {
	case MSG_TYPE_RESERVE_PING:
		cli.onPing(msg)
	case MSG_TYPE_RESERVE_PONG:
		cli.onPong(msg)
	default:
		CallbackRequest(msg) // the msg should Clear by request side after handle msg
	}
}

func (cli *VsockClient) onPing(msg *msgbuf.MsgBody) {
	defer msg.Clear()
	cli.sendPack(nil, MSG_TYPE_RESERVE_PONG, msg.MsgId())
	// cli.log.Debug("onPing")
	return
}

func (cli *VsockClient) onPong(msg *msgbuf.MsgBody) {
	defer msg.Clear()
	// cli.log.Debug("onPong")
	return
}
