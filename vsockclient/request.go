package vsockclient

import (
	"errors"
	"sync"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/zh4af/vsockpkg/msgbuf"
	"github.com/zh4af/vsockpkg/utils"
)

var (
	Err_Request_Parameter   = errors.New("vsock client request parameter err")
	Err_Request_Timeout     = errors.New("vsock client request timeout")
	Err_Response_Body_Empty = errors.New("vsock client response empty")
	Default_Request_Timeout = time.Second * 30
)

var (
	allRequest    = make(map[uint64]chan *msgbuf.MsgBody, 100) // each msg with msg_id has one chan to recieve MsgBody
	allRequestMtx sync.RWMutex
)

func NewRequest(msgId uint64) (uint64, chan *msgbuf.MsgBody) {
	var ch = make(chan *msgbuf.MsgBody)
	allRequestMtx.Lock()
	allRequest[msgId] = ch
	allRequestMtx.Unlock()
	return msgId, ch
}

func ClearRequest(msgId uint64) {
	allRequestMtx.Lock()
	if ch := allRequest[msgId]; ch != nil {
		close(ch)
	}
	delete(allRequest, msgId)
	allRequestMtx.Unlock()
}

func CallbackRequest(msg *msgbuf.MsgBody) {
	if msg == nil {
		return
	}
	allRequestMtx.RLock()
	defer allRequestMtx.RUnlock()
	ch := allRequest[msg.MsgId()]
	if ch != nil {
		select {
		case _, ok := <-ch:
			if !ok { // ch closed
				msg.Clear()
				return
			}
		case ch <- msg:
		}
	}
}

func WaitRequest(msg proto.Message, msgType uint16) (*msgbuf.MsgBody, error) {
	if !DefaultVsockClient.ready {
		return nil, Err_Vsock_Client_Not_Ready
	}
	var body []byte
	var err error
	if msg != nil {
		body, err = proto.Marshal(msg)
		if err != nil {
			return nil, Err_Request_Parameter
		}
	}

	msgId, ch := NewRequest(utils.GenerateRandomUint64())
	defer ClearRequest(msgId)
	err = DefaultVsockClient.sendPack(body, msgType, msgId)
	if err != nil {
		return nil, err
	}
	timeout := time.NewTicker(Default_Request_Timeout)
	defer timeout.Stop()
	select {
	case retMsg := <-ch:
		if retMsg.Body() == nil {
			return nil, Err_Response_Body_Empty
		}
		return retMsg, nil
	case <-timeout.C:
		return nil, Err_Request_Timeout
	}
}
