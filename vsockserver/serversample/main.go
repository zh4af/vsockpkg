package main

import (
	"errors"

	"github.com/Sirupsen/logrus"
	"github.com/golang/protobuf/proto"
	"github.com/zh4af/vsockpkg/msgbuf"
	"github.com/zh4af/vsockpkg/vsockserver"
	"github.com/zh4af/vsockpkg/vsockserver/serversample/pb"
)

var (
	ERR_MSG_EMPTY = errors.New("message empty")
)

func init() {
	logrus.SetLevel(logrus.DebugLevel)
	logrus.SetFormatter(&logrus.TextFormatter{
		TimestampFormat: "2006-01-02 15:04:05.000000000",
		FullTimestamp:   true,
	})
}

func main() {
	var err error

	if err = vsockserver.Start(3000); err != nil {
		logrus.WithError(err).Fatal("vsockserver.Start failed")
	}

	vsockserver.AddServHandler(uint16(pb.MessageType_MSG_HELLO), Hello)

	select {}
}

func Hello(msg *msgbuf.MsgBody) (proto.Message, error) {
	if msg == nil || msg.Body() == nil {
		return nil, ERR_MSG_EMPTY
	}

	var req pb.HelloReq
	var rsp pb.HelloRsp

	err := proto.Unmarshal(msg.Body().Bytes(), &req)
	if err != nil {
		return nil, err
	}

	// logrus.Debugf("server handle: %s", req.Hi)
	rsp.Hello = "hello"
	return &rsp, nil
}
