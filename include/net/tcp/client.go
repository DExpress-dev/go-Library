package tcp

import (
	"errors"
	"fmt"
	log4plus "github.com/include/log4go"
	"net"
	"sync"
	"time"
)

type TCPClient struct {
	lock      sync.Mutex
	linkers   map[uint64]*Linker
	curHandle uint64
	event     TCPEvent
}

var gTCPClient *TCPClient

func (k *TCPClient) CreateHandler() uint64 {
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

func (k *TCPClient) AddLinker(linker *Linker) error {
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

func (k *TCPClient) deleteLinker(handle uint64) {
	k.lock.Lock()
	defer k.lock.Unlock()
	linker, Ok := k.linkers[handle]
	if Ok {
		_ = linker.conn.Close()
		delete(k.linkers, handle)
	}
}

func (k *TCPClient) findLinker(handle uint64) *Linker {
	k.lock.Lock()
	defer k.lock.Unlock()
	linker, Ok := k.linkers[handle]
	if Ok {
		return linker
	}
	return nil
}

func (k *TCPClient) Send(handle uint64, data []byte) error {
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

func (k *TCPClient) start(listen string) (error, uint64) {
	funName := "start"
	conn, err := net.Dial("tcp", listen)
	if err != nil {
		errString := fmt.Sprintf("%s Dial Failed err=[%s]", funName, err.Error())
		log4plus.Error(errString)
		return errors.New(errString), 0
	}
	tcpAddr := conn.RemoteAddr()
	var remoteIp string = ""
	var remotePort int = -1
	if tcpAddr, ok := tcpAddr.(*net.TCPAddr); ok {
		remoteIp = tcpAddr.IP.String()
		remotePort = tcpAddr.Port
	} else {
		errString := fmt.Sprintf("%s tcpAddr.(*net.TCPAddr) Failed", funName)
		log4plus.Error(errString)
		return errors.New(errString), 0
	}
	handle := k.CreateHandler()
	linker := NewLinker(handle, remoteIp, remotePort, conn)
	linker.Init(gTCPClient)
	log4plus.Info("%s Listen Success handle=[%d] remoteIp=[%s] remotePort=[%d]---->>>>", funName, handle, remoteIp, remotePort)
	return nil, handle
}

func (k *TCPClient) Init(event TCPEvent) error {
	funName := "Init"
	if nil == gTCPServer {
		errString := fmt.Sprintf("%s Init Failed Object is nil", funName)
		log4plus.Error(errString)
		return errors.New(errString)
	}
	gTCPServer.event = event
	return nil
}

func (k *TCPClient) OnSend(handle uint64, remoteIp string, remotePort int, size int) {
	k.event.OnSend(handle, remoteIp, remotePort, size)
}

func (k *TCPClient) OnRead(handle uint64, remoteIp string, remotePort int, data []byte, size int) bool {
	return k.event.OnRead(handle, remoteIp, remotePort, data, size)
}

func (k *TCPClient) OnDisconnect(handle uint64, remoteIp string, remotePort int) {
	k.event.OnDisconnect(handle, remoteIp, remotePort)
}

func (k *TCPClient) OnError(handle uint64, remoteIp string, remotePort int) {
	k.event.OnError(handle, remoteIp, remotePort)
}

func SingtonTCPClient() *TCPClient {
	if nil == gTCPClient {
		gTCPClient = &TCPClient{
			curHandle: 1000,
			linkers:   make(map[uint64]*Linker),
		}
	}
	return gTCPClient
}
