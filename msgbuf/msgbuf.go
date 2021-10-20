package msgbuf

import (
	"bytes"
	"encoding/binary"
	"sync"

	"github.com/zh4af/vsockpkg/utils"
)

const (
	Msg_Header_Bytes         = 16
	Msg_Header_MsgType_Bytes = 2
	Msg_Header_MsgLen_Bytes  = 2
	Msg_Header_MsgId_Bytes   = 8
	Msg_Header_Reserve_Bytes = 16 - 2 - 2 - 8
)

type bufferPool struct {
	*sync.Pool
}

var BufPool = newBufferPool(512)

func newBufferPool(size int) *bufferPool {
	return &bufferPool{
		&sync.Pool{New: func() interface{} {
			return bytes.NewBuffer(make([]byte, 0, size))
		}},
	}
}

func (bp *bufferPool) Get() *bytes.Buffer {
	return (bp.Pool.Get()).(*bytes.Buffer)
}

func (bp *bufferPool) Put(b *bytes.Buffer) {
	b.Reset()
	bp.Pool.Put(b)
}

type MsgBody struct {
	buffer  *bytes.Buffer // full bytes of msg. sync.Pool
	msgType uint16
	msgId   uint64
	body    *bytes.Buffer // body of msg
}

func NewMsgBody(msgType uint16, msgId uint64, body []byte) *MsgBody {
	msg := &MsgBody{}
	msg.buffer = BufPool.Get()
	msg.msgType = msgType
	msg.msgId = msgId
	if len(body) > 0 {
		msg.body = BufPool.Get()
		msg.body.Write(body)
	}
	return msg
}

func (msg *MsgBody) Buffer() *bytes.Buffer {
	if msg == nil || msg.buffer == nil {
		return nil
	}
	return msg.buffer
}

func (msg *MsgBody) Body() *bytes.Buffer {
	if msg == nil || msg.body == nil {
		return nil
	}
	return msg.body
}

func (msg *MsgBody) MsgType() uint16 {
	if msg == nil {
		return 0
	}
	return msg.msgType
}

func (msg *MsgBody) MsgId() uint64 {
	if msg == nil {
		return 0
	}
	return msg.msgId
}

func (msg *MsgBody) Clear() {
	if msg.buffer != nil {
		BufPool.Put(msg.buffer)
		msg.buffer = nil
	}
	if msg.body != nil {
		BufPool.Put(msg.body)
		msg.body = nil
	}
}

func (msg *MsgBody) Pack() {
	if msg.buffer == nil {
		return
	}
	bodyLen := 0
	if msg.body != nil {
		bodyLen = msg.body.Len()
	}
	// pack header
	msg.buffer.Write(utils.Uint16ToBytes(msg.msgType))     // msgType, 2 bytes
	msg.buffer.Write(utils.Uint16ToBytes(uint16(bodyLen))) // bodyLen, 2 bytes
	msg.buffer.Write(utils.Uint64ToBytes(msg.msgId))       // msgId, 8 bytes
	for i := 0; i < Msg_Header_Reserve_Bytes; i++ {        // header reserve bytes
		msg.buffer.WriteByte(byte(0))
	}
	// pack body
	if bodyLen > 0 {
		msg.buffer.Write(msg.body.Bytes())
	}

	// clear msg.body
	if msg.body != nil {
		BufPool.Put(msg.body)
		msg.body = nil
	}
}

func (msg *MsgBody) UnPack() {
	// unpack header
	n := 0
	msg.msgType = binary.BigEndian.Uint16(msg.buffer.Bytes()[:Msg_Header_MsgType_Bytes])
	n += Msg_Header_MsgType_Bytes
	// bodyLen := binary.BigEndian.Uint16(msg.buffer.Bytes()[n : n+Msg_Header_MsgLen_Bytes])
	n += Msg_Header_MsgLen_Bytes
	msg.msgId = binary.BigEndian.Uint64(msg.buffer.Bytes()[n : n+Msg_Header_MsgId_Bytes])

	// unpack body
	msg.body = BufPool.Get()
	msg.body.Write(msg.buffer.Bytes()[Msg_Header_Bytes:])

	// clear msg.buffer
	if msg.buffer != nil {
		BufPool.Put(msg.buffer)
		msg.buffer = nil
	}
}

func BufferGrowNByte(buf *bytes.Buffer, n int) {
	if buf == nil {
		return
	}
	for i := 0; i < n; i++ {
		buf.WriteByte(byte(0))
	}
}
