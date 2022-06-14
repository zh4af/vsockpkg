package main

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/golang/protobuf/proto"
	"github.com/zh4af/vsockpkg/vsockclient"
	"github.com/zh4af/vsockpkg/vsockserver/serversample/pb"
)

var (
	cid       int
	conns     int
	concurent int
	total     int
	turn      int
	logfile   string

	client *vsockclient.VsockClient
)

func init() {
	const usage = "clientsample [-cid cid] [-conns conns] [-c concurent] [-t total] [-turn turn] [-f logfile]"
	flag.IntVar(&cid, "cid", 3, usage)
	flag.IntVar(&conns, "conns", 10, usage)
	flag.IntVar(&concurent, "c", 2, usage)
	flag.IntVar(&total, "t", 1e4, usage)
	flag.IntVar(&turn, "turn", 10, usage)
	flag.StringVar(&logfile, "f", "console", usage)

	logrus.SetLevel(logrus.DebugLevel)
	logrus.SetFormatter(&logrus.TextFormatter{
		TimestampFormat: "2006-01-02 15:04:05.000000000",
		FullTimestamp:   true,
	})
}

func main() {
	flag.Parse()
	if logfile != "console" {
		file, err := os.OpenFile(logfile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0660)
		if nil != err {
			panic(err)
		}
		logrus.SetOutput(file)
	}

	// client = vsockclient.NewVsockClient(uint32(cid), 3000, nil)
	sayHi()
	select {}
}

func sayHi() error {
	var req pb.HelloReq
	req.Hi = "Hi"

	var clients = make([]*vsockclient.VsockClient, conns)
	for i := 0; i < conns; i++ {
		clients[i] = vsockclient.NewVsockClient(uint32(cid), 3000, nil)
	}
	time.Sleep(time.Millisecond * 10 * time.Duration(conns))
	rand.Seed(time.Now().UnixNano())

	for t := 0; t < turn; t++ {
		startTime := time.Now()
		var timeout = time.NewTimer(time.Second)
		logrus.Debugf("start: %v", startTime)
		var retCnt int
		var retLock sync.Mutex
		var done = make(chan struct{})
		go func() {
			wg := sync.WaitGroup{}
			for i := 0; i < concurent; i++ {
				wg.Add(1)
				go func() {
					defer wg.Done()

					for n := 0; n < total/concurent; n++ {
						retMsg, err := clients[rand.Intn(conns)].Request(&req, uint16(pb.MessageType_MSG_HELLO))
						// retMsg, err := client.Request(&req, uint16(pb.MessageType_MSG_HELLO))
						if err != nil {
							return
						}
						defer retMsg.Clear()

						var rsp pb.HelloRsp
						err = proto.Unmarshal(retMsg.Body().Bytes(), &rsp)
						if err != nil {
							return
						}
						// if n%1000 == 0 {
						// 	logrus.Debugf("client reieve rsp: %s", rsp.Hello)
						// }
						retLock.Lock()
						retCnt++
						retLock.Unlock()
					}
				}()
			}
			wg.Wait()
			done <- struct{}{}
		}()

		select {
		case <-done:
			logrus.Debugf("end: %v, %v, %v", retCnt, time.Now(), time.Now().Sub(startTime))
		case <-timeout.C:
			logrus.Debugf("end: %v, %v, %v", retCnt, time.Now(), time.Now().Sub(startTime))
		}

		fmt.Println("")
		time.Sleep(time.Second)
	}
	return nil
}
