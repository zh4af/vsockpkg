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

type Requester struct {
	requests map[uint64]chan *msgbuf.MsgBody
	sync.RWMutex
}

func (cli *VsockClient) NewRequest() (uint64, chan *msgbuf.MsgBody) {
	var ch = make(chan *msgbuf.MsgBody)
	var msgId = utils.GenerateRandomUint64()
	cli.requester.Lock()
	cli.requester.requests[msgId] = ch
	cli.requester.Unlock()
	return msgId, ch
}

func (cli *VsockClient) ClearRequest(msgId uint64) {
	cli.requester.Lock()
	if ch := cli.requester.requests[msgId]; ch != nil {
		close(ch)
	}
	delete(cli.requester.requests, msgId)
	cli.requester.Unlock()
}

func (cli *VsockClient) CallbackRequest(msg *msgbuf.MsgBody) {
	if msg == nil {
		return
	}
	cli.requester.RLock()
	defer cli.requester.RUnlock()
	ch := cli.requester.requests[msg.MsgId()]
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

func (cli *VsockClient) Request(msg proto.Message, msgType uint16) (*msgbuf.MsgBody, error) {
	if !cli.ready {
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

	msgId, ch := cli.NewRequest()
	defer cli.ClearRequest(msgId)
	err = cli.sendPack(body, msgType, msgId)
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
