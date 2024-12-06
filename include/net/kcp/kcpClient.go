package kcp

import (
	"errors"
	"fmt"
	log4plus "github.com/include/log4go"
	"github.com/xtaci/kcp-go/v5"
	"net"
	"sync"
	"time"
)

type KCPClient struct {
	lock      sync.Mutex
	linkers   map[uint64]*Linker
	curHandle uint64
	event     KCPEvent
}

var gKCPClient *KCPClient

func (k *KCPClient) CreateHandler() uint64 {
	for {
		k.curHandle++
		if k.curHandle < 1000 {
			k.curHandle = 1000
		}
		linker := k.findLinker(k.curHandle)
		if linker == nil {
			return k.curHandle
		}
	}
}

func (k *KCPClient) AddLinker(linker *Linker) error {
	funName := "AddLinker"
	if linker == nil {
		errString := fmt.Sprintf("%s linker is nil", funName)
		log4plus.Error(errString)
		return errors.New(errString)
	}
	k.lock.Lock()
	defer k.lock.Unlock()
	k.linkers[linker.Handle()] = linker
	return nil
}

func (k *KCPClient) deleteLinker(handle uint64) {
	k.lock.Lock()
	defer k.lock.Unlock()
	linker, Ok := k.linkers[handle]
	if Ok {
		_ = linker.conn.Close()
		delete(k.linkers, handle)
	}
}

func (k *KCPClient) findLinker(handle uint64) *Linker {
	k.lock.Lock()
	defer k.lock.Unlock()
	linker, Ok := k.linkers[handle]
	if Ok {
		return linker
	}
	return nil
}

func (k *KCPClient) Send(handle uint64, data []byte) error {
	funName := "Send"
	now := time.Now().Unix()
	defer func() {
		log4plus.Info("%s handle=[%d] data=[%d] consumption time=%d(s)", funName, handle, len(data), time.Now().Unix()-now)
	}()
	linker := k.findLinker(handle)
	if linker == nil {
		errString := fmt.Sprintf("%s findLinker Failed not found Object from handle=[%d]", funName, handle)
		log4plus.Error(errString)
		return errors.New(errString)
	}
	_ = linker.Send(data)
	return nil
}

func (k *KCPClient) start(listen string) (error, uint64) {
	funName := "start"
	addr, err := net.ResolveUDPAddr("udp", listen)
	if err != nil {
		errString := fmt.Sprintf("%s ResolveUDPAddr Failed err=[%s]", funName, err.Error())
		log4plus.Error(errString)
		return errors.New(errString), 0
	}
	conn, err := kcp.DialWithOptions(addr.String(), nil, 10, 3)
	if err != nil {
		errString := fmt.Sprintf("%s DialWithOptions Failed err=[%s]", funName, err.Error())
		log4plus.Error(errString)
		return errors.New(errString), 0
	}
	remoteIp := addr.IP.String()
	remotePort := addr.Port
	handle := k.CreateHandler()
	linker := NewLinker(handle, remoteIp, remotePort, conn)
	linker.Init(gKCPClient)
	log4plus.Info("%s Listen Success handle=[%d] remoteIp=[%s] remotePort=[%d]---->>>>", funName, handle, remoteIp, remotePort)
	return nil, handle
}

func (k *KCPClient) Init(event KCPEvent) error {
	funName := "Init"
	if nil == gKCPClient {
		errString := fmt.Sprintf("%s Init Failed Object is nil", funName)
		log4plus.Error(errString)
		return errors.New(errString)
	}
	gKCPClient.event = event
	return nil
}

func (k *KCPClient) OnSend(handle uint64, remoteIp string, remotePort int, size int) {
	k.event.OnSend(handle, remoteIp, remotePort, size)
}

func (k *KCPClient) OnRead(handle uint64, remoteIp string, remotePort int, data []byte, size int) bool {
	return k.event.OnRead(handle, remoteIp, remotePort, data, size)
}

func (k *KCPClient) OnDisconnect(handle uint64, remoteIp string, remotePort int) {
	k.event.OnDisconnect(handle, remoteIp, remotePort)
}

func (k *KCPClient) OnError(handle uint64, remoteIp string, remotePort int) {
	k.event.OnError(handle, remoteIp, remotePort)
}

func SingtonKCPClient() *KCPClient {
	if nil == gKCPClient {
		gKCPClient = &KCPClient{
			curHandle: 1000,
			linkers:   make(map[uint64]*Linker),
		}
	}
	return gKCPClient
}
