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
	OnError(handle uint64, remoteIp string, remotePort int)
}

type TCPServer struct {
	lock      sync.Mutex
	linkers   map[uint64]*Linker
	curHandle uint64
	localIp   string
	localPort int
	event     TCPEvent
}

var gTCPServer *TCPServer

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

func (k *TCPServer) AddLinker(linker *Linker) error {
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

func (k *TCPServer) Send(handle uint64, data []byte) error {
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
		linker := NewLinker(handle, remoteIp, remotePort, conn)
		linker.Init(gTCPServer)
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

func (k *TCPServer) Init(event TCPEvent) error {
	funName := "Init"
	if nil == gTCPServer {
		errString := fmt.Sprintf("%s Init Failed Object is nil", funName)
		log4plus.Error(errString)
		return errors.New(errString)
	}
	gTCPServer.event = event
	return nil
}

func (k *TCPServer) OnSend(handle uint64, remoteIp string, remotePort int, size int) {
	k.event.OnSend(handle, remoteIp, remotePort, size)
}

func (k *TCPServer) OnRead(handle uint64, remoteIp string, remotePort int, data []byte, size int) bool {
	return k.event.OnRead(handle, remoteIp, remotePort, data, size)
}

func (k *TCPServer) OnDisconnect(handle uint64, remoteIp string, remotePort int) {
	k.event.OnDisconnect(handle, remoteIp, remotePort)
}

func (k *TCPServer) OnError(handle uint64, remoteIp string, remotePort int) {
	k.event.OnError(handle, remoteIp, remotePort)
}

func SingtonTCPServer() *TCPServer {
	if nil == gTCPServer {
		gTCPServer = &TCPServer{
			curHandle: 1000,
			localIp:   "",
			localPort: -1,
			linkers:   make(map[uint64]*Linker),
		}
	}
	return gTCPServer
}
