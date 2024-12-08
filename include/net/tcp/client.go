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
	lock           sync.Mutex
	linkers        map[uint64]*Linker
	curHandle      uint64
	event          TCPEvent
	timeOutSecond  int64
	disconnectChan chan uint64
}

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

func (k *TCPClient) AddLinker(linker *Linker) {
	funName := "AddLinker"
	if linker == nil {
		errString := fmt.Sprintf("%s linker is nil", funName)
		log4plus.Error(errString)
		return
	}
	k.lock.Lock()
	defer k.lock.Unlock()
	k.linkers[linker.Handle()] = linker
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

func (k *TCPClient) Send(handle uint64, data []byte) (error, int) {
	funName := "Send"
	linker := k.findLinker(handle)
	if linker == nil {
		errString := fmt.Sprintf("%s findLinker Failed not found Object from handle=[%d]", funName, handle)
		log4plus.Error(errString)
		return errors.New(errString), 0
	}
	err, nRet := linker.Send(data)
	if err != nil {
		k.disconnectChan <- handle
		return err, 0
	}
	return nil, nRet
}

func (k *TCPClient) Start(listen string) (error, uint64) {
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
	linker := NewLinker(handle, remoteIp, remotePort, conn, Client)
	linker.Init(k)
	k.AddLinker(linker)
	log4plus.Info("%s Listen Success handle=[%d] remoteIp=[%s] remotePort=[%d]---->>>>", funName, handle, remoteIp, remotePort)

	return nil, handle
}

func (k *TCPClient) Init(event TCPEvent) {
	k.event = event
	go k.timeOut()
}

func (k *TCPClient) forceClose(handle uint64) {
	linker := k.findLinker(handle)
	if linker != nil {
		_ = linker.conn.Close()

		k.lock.Lock()
		defer k.lock.Unlock()
		delete(k.linkers, linker.Handle())
	}
}

func (k *TCPClient) timeOut() {
	funName := "timeOut"
	for {
		time.Sleep(time.Duration(5) * time.Second)
		now := time.Now().Add(time.Duration(-1*k.timeOutSecond) * time.Second)
		for _, linker := range k.linkers {
			if now.Sub(linker.Heartbeat()) > 0 {
				log4plus.Info("%s linker Object timeOut handle=[%d] remoteIp=[%s] remotePort=[%d] ---->>>>", funName, linker.Handle(), linker.Ip(), linker.Port())
				if k.event != nil {
					k.event.OnDisconnect(linker.Handle(), linker.Ip(), linker.Port())
				}
				k.forceClose(linker.Handle())
			}
		}
	}
}

func (k *TCPClient) OnSend(handle uint64, remoteIp string, remotePort int, size int) {
	if k.event != nil {
		k.event.OnSend(handle, remoteIp, remotePort, size)
	}
}

func (k *TCPClient) OnRead(handle uint64, remoteIp string, remotePort int, data []byte, size int) bool {
	if k.event != nil {
		return k.event.OnRead(handle, remoteIp, remotePort, data, size)
	}
	return true
}

func (k *TCPClient) OnDisconnect(handle uint64, remoteIp string, remotePort int) {
	if k.event != nil {
		k.event.OnDisconnect(handle, remoteIp, remotePort)
	}
	k.disconnectChan <- handle
}

func (k *TCPClient) OnError(handle uint64, remoteIp string, remotePort int, err error) {
	if k.event != nil {
		k.event.OnError(handle, remoteIp, remotePort, err)
	}
}

func (k *TCPClient) waitEvent() {
	for {
		select {
		case handle := <-k.disconnectChan:
			k.forceClose(handle)
		}
	}
}

func NewTCPClient() *TCPClient {
	gTCPClient := &TCPClient{
		timeOutSecond:  15,
		curHandle:      1000,
		linkers:        make(map[uint64]*Linker),
		disconnectChan: make(chan uint64),
	}
	go gTCPClient.waitEvent()
	return gTCPClient
}
