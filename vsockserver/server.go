package vsockserver

import (
	"errors"
	"log"
	"net"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/go-stack/stack"
	"github.com/golang/protobuf/proto"
	"github.com/mdlayher/vsock"
	"github.com/zh4af/vsockpkg/msgbuf"
)

var (
	MSG_TYPE_RESERVE_PING uint16 = 10001
	MSG_TYPE_RESERVE_PONG uint16 = 10002
)

type VSockServer struct {
	// vm socker port to listen on.
	port uint32

	l              *vsock.Listener
	conHandlers    map[string]*ConnHandler
	conHandlersMtx sync.RWMutex
	closed         chan struct{}

	servHandlers map[uint16]func(*msgbuf.MsgBody) (proto.Message, error)
}

var enclaveServ VSockServer

func NewVSockServer(port uint32) {
	enclaveServ = VSockServer{
		port:         port,
		conHandlers:  make(map[string]*ConnHandler),
		closed:       make(chan struct{}),
		servHandlers: make(map[uint16]func(*msgbuf.MsgBody) (proto.Message, error), 100),
	}
}

/*
* msgType should be uint16[1~65535] exclude msg types which server has reserved:
* MSG_TYPE_RESERVE_PING = 10001
* MSG_TYPE_RESERVE_PONG = 10002
 */
func AddServHandler(msgType uint16, f func(*msgbuf.MsgBody) (proto.Message, error)) {
	enclaveServ.servHandlers[msgType] = f
}

func GetServHandler(msgType uint16) (func(*msgbuf.MsgBody) (proto.Message, error), error) {
	f := enclaveServ.servHandlers[msgType]
	if f == nil {
		return nil, errors.New("unknown msg type")
	}
	return f, nil
}

func Start(port uint32) error {
	NewVSockServer(port)
	return enclaveServ.ListenAndServe()
}

func (srv *VSockServer) GetHanderNum() int {
	srv.conHandlersMtx.RLock()
	defer srv.conHandlersMtx.RUnlock()

	return len(srv.conHandlers)
}

func (srv *VSockServer) Close() (err error) {
	err = srv.l.Close()
	select {
	case _, ok := <-srv.closed:
		if !ok { // chan closed
			return nil
		}
	default:
		close(srv.closed)
	}

	srv.conHandlersMtx.RLock()
	for _, h := range srv.conHandlers {
		h.Close()
	}
	srv.conHandlersMtx.RUnlock()

	return
}

func (srv *VSockServer) ListenAndServe() error {
	cid, _ := vsock.ContextID()
	logrus.WithField("contextId", cid).Debug("vsock context id")

	l, err := vsock.Listen(srv.port)
	if err != nil {
		return err
	}
	go srv.ServerLoop(l)
	return nil
}

func (srv *VSockServer) ServerLoop(l *vsock.Listener) (err error) {
	srv.l = l
	defer func() {
		srv.Close()
	}()
	logrus.Debug("awaiting connections...")
	for {
		var conn net.Conn
		conn, err = l.Accept()
		if err != nil {
			select {
			case _, ok := <-srv.closed:
				if !ok { // chan closed
					err = nil
					return
				}
			default:
			}
			if ne, ok := err.(net.Error); ok && ne.Temporary() {
				time.Sleep(5 * time.Millisecond)
				continue
			}
			logrus.WithError(err).Error("tcp server accept err")
			return
		}
		go srv.serve(conn)
	}
}

func (srv *VSockServer) serve(conn net.Conn) {
	defer func() {
		conn.Close()

		err := recover()
		if err != nil {
			log.Printf("panic: %v\n", err)
			log.Println("Stack")
			cts := stack.Trace()
			for i := range cts {
				log.Printf("Stack: %+v %n", cts[i], cts[i])
			}
		}
	}()

	logrus.WithFields(logrus.Fields{
		"remoteAddr": conn.RemoteAddr().String(),
	}).Info("recieve conn")
	handler := NewConnHandler(conn)
	srv.conHandlersMtx.Lock()
	srv.conHandlers[handler.connId] = handler
	srv.conHandlersMtx.Unlock()

	handler.ServerLoop() // loop

	// exit
	srv.conHandlersMtx.Lock()
	delete(srv.conHandlers, handler.connId)
	srv.conHandlersMtx.Unlock()
}
