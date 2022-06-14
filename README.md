server
=======

```golang
    // initialize and start vsock server 
    if err = vsockserver.Start(server_port); err != nil {
        logrus.WithError(err).Fatal("vsockserver.Start failed")
    }

    // register mesage and handler to vsock server
    /*  mesage shoule be declared in some file, each msg has one msg_type and one handler(type: func(*msgbuf.MsgBody) (proto.Message, error))
        MessageType_MSG_KMS_CREDENTIALS = 1, handler1
        MessageType_MSG_NEW_DATA_KEY = 2, handler2
        MessageType_MSG_SET_DATA_KEY = 3, handler3
        MessageType_MSG_XXXXXXXX = 4, handler4
    */
    vsockserver.AddServHandler(uint16(MessageType_MSG_KMS_CREDENTIALS), handler1)
    vsockserver.AddServHandler(uint16(MessageType_MSG_NEW_DATA_KEY), handler2)
    vsockserver.AddServHandler(uint16(MessageType_MSG_SET_DATA_KEY), handler3)
    vsockserver.AddServHandler(uint16(MessageType_MSG_XXXXXXXX), handler4)

```
there is a server sample code in path https://github.com/zh4af/vsockpkg/tree/master/vsockserver/serversample


client
======
init vsock client
------
```golang
    // invoke vsockclient.NewVsockClient once when start service in main
    /*
      enclave_cid: vm socket server cid (run in enclave)
      port: vm socke port to connect
      onConnected: func which be invoked when connection established
    */
    client := vsockclient.NewVsockClient(enclave_cid, port, onConnected)
```
request and response through vsock client
-----------------------------------------
```golang
    var req proto.Message
    // initilized req....
    retMsg, err := client.WaitRequest(&req, uint16(MessageType_MSG_XXXXXXXX))
    if err != nil {
        return nil, err
    }
    defer retMsg.Clear()

    var rsp proto.Message
    err = proto.Unmarshal(retMsg.Body().Bytes(), &rsp)
    if err != nil {
        return nil, err
    }

```
there is a client sample code in path https://github.com/zh4af/vsockpkg/tree/master/vsockclient/clientsample