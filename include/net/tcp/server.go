package tcp

import (
	"errors"
	"fmt"
	log4plus "github.com/include/log4go"
	"net"
	"sync"
	"time"
)

type TCPEvent interface {
	OnSend(handle uint64, remoteIp string, remotePort int, size int)
	OnRead(handle uint64, remoteIp string, remotePort int, data []byte, size int) bool
	OnDisconnect(handle uint64, remoteIp string, remotePort int)
	OnError(handle uint64, remoteIp string, remotePort int, err error)
}

type TCPServer struct {
	lock           sync.Mutex
	linkers        map[uint64]*Linker
	curHandle      uint64
	localIp        string
	localPort      int
	event          TCPEvent
	timeOutSecond  int
	disconnectChan chan uint64
}

func (k *TCPServer) CreateHandler() uint64 {
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

func (k *TCPServer) AddLinker(linker *Linker) {
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

func (k *TCPServer) deleteLinker(handle uint64) {
	k.lock.Lock()
	defer k.lock.Unlock()
	linker, Ok := k.linkers[handle]
	if Ok {
		_ = linker.conn.Close()
		delete(k.linkers, handle)
	}
}

func (k *TCPServer) findLinker(handler uint64) *Linker {
	k.lock.Lock()
	defer k.lock.Unlock()
	linker, Ok := k.linkers[handler]
	if Ok {
		return linker
	}
	return nil
}

func (k *TCPServer) Send(handle uint64, data []byte) (error, int) {
	funName := "Send"
	now := time.Now().Unix()
	defer func() {
		log4plus.Info("%s handle=[%d] data=[%d] consumption time=%d(s)", funName, handle, len(data), time.Now().Unix()-now)
	}()
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

func (k *TCPServer) accept(listener net.Listener) {
	funName := "accept"
	for {
		conn, err := listener.Accept()
		if err != nil {
			errString := fmt.Sprintf("%s Accept Failed err=[%s]", funName, err.Error())
			log4plus.Error(errString)
			continue
		}
		var remoteIp string = ""
		var remotePort int = -1
		if tcpAddr, ok := conn.RemoteAddr().(*net.TCPAddr); ok {
			remoteIp = tcpAddr.IP.String()
			remotePort = tcpAddr.Port
		} else {
			errString := fmt.Sprintf("%s conn.RemoteAddr() Failed", funName)
			log4plus.Error(errString)
			continue
		}
		log4plus.Info("%s New Connection remoteIp=[%s] remotePort=[%d]---->>>>", funName, remoteIp, remotePort)
		handle := k.CreateHandler()
		linker := NewLinker(handle, remoteIp, remotePort, conn, Server)
		linker.Init(k)
		k.AddLinker(linker)
	}
}

func (k *TCPServer) Start(listen string) error {
	funName := "Start"
	listener, err := net.Listen("tcp", listen)
	if err != nil {
		errString := fmt.Sprintf("%s Listen Failed err=[%s]", funName, err.Error())
		log4plus.Error(errString)
		return errors.New(errString)
	}
	addr := listener.Addr()
	if tcpAddr, ok := addr.(*net.TCPAddr); ok {
		k.localIp = tcpAddr.IP.String()
		k.localPort = tcpAddr.Port
		log4plus.Info("%s Listen Success localIp=[%s] localPort=[%d]---->>>>", funName, k.localIp, k.localPort)

		go k.accept(listener)
		return nil
	}
	errString := fmt.Sprintf("%s addr.(*net.TCPAddr) Failed listen=[%s]", funName, listen)
	log4plus.Error(errString)
	return errors.New(errString)
}

func (k *TCPServer) Init(event TCPEvent) {
	k.event = event
	go k.timeOut()
}

func (k *TCPServer) OnSend(handle uint64, remoteIp string, remotePort int, size int) {
	if k.event != nil {
		k.event.OnSend(handle, remoteIp, remotePort, size)
	}
}

func (k *TCPServer) OnRead(handle uint64, remoteIp string, remotePort int, data []byte, size int) bool {
	if k.event != nil {
		return k.event.OnRead(handle, remoteIp, remotePort, data, size)
	}
	return true
}

func (k *TCPServer) OnDisconnect(handle uint64, remoteIp string, remotePort int) {
	if k.event != nil {
		k.event.OnDisconnect(handle, remoteIp, remotePort)
	}
	k.disconnectChan <- handle
}

func (k *TCPServer) OnError(handle uint64, remoteIp string, remotePort int, err error) {
	if k.event != nil {
		k.event.OnError(handle, remoteIp, remotePort, err)
	}
}

func (k *TCPServer) forceClose(handle uint64) {
	linker := k.findLinker(handle)
	if linker != nil {
		_ = linker.conn.Close()

		k.lock.Lock()
		defer k.lock.Unlock()
		delete(k.linkers, linker.Handle())
	}
}

func (k *TCPServer) timeOut() {
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

func (k *TCPServer) waitEvent() {
	for {
		select {
		case handle := <-k.disconnectChan:
			k.forceClose(handle)
		}
	}
}

func NewTCPServer() *TCPServer {
	gTCPServer := &TCPServer{
		timeOutSecond:  15,
		curHandle:      1000,
		localIp:        "",
		localPort:      -1,
		linkers:        make(map[uint64]*Linker),
		disconnectChan: make(chan uint64),
	}
	go gTCPServer.waitEvent()
	return gTCPServer
}
