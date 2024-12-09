package kcp

import (
	"errors"
	"fmt"
	log4plus "github.com/include/log4go"
	"github.com/xtaci/kcp-go/v5"
	"net"
	"strings"
	"sync"
	"time"
)

type KCPEvent interface {
	OnSend(handle uint64, remoteIp string, remotePort int, size int)
	OnRead(handle uint64, remoteIp string, remotePort int, data []byte, size int) bool
	OnDisconnect(handle uint64, remoteIp string, remotePort int)
	OnError(handle uint64, remoteIp string, remotePort int, err error)
}

type KCPServer struct {
	lock           sync.Mutex
	linkers        map[uint64]*Linker
	curHandle      uint64
	localIp        string
	localPort      int
	event          KCPEvent
	timeOutSecond  int64
	disconnectChan chan uint64
}

func (k *KCPServer) CreateHandler() uint64 {
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

func (k *KCPServer) AddLinker(linker *Linker) {
	k.lock.Lock()
	defer k.lock.Unlock()
	k.linkers[linker.Handle()] = linker
}

func (k *KCPServer) deleteLinker(handle uint64) {
	k.lock.Lock()
	defer k.lock.Unlock()
	linker, Ok := k.linkers[handle]
	if Ok {
		_ = linker.conn.Close()
		delete(k.linkers, handle)
	}
}

func (k *KCPServer) findLinker(handler uint64) *Linker {
	k.lock.Lock()
	defer k.lock.Unlock()
	linker, Ok := k.linkers[handler]
	if Ok {
		return linker
	}
	return nil
}

func (k *KCPServer) Send(handle uint64, data []byte) (error, int) {
	linker := k.findLinker(handle)
	if linker == nil {
		errString := fmt.Sprintf("findLinker Failed not found Object from handle=[%d]", handle)
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

func (k *KCPServer) accept(listener *kcp.Listener) {
	funName := "accept"
	for {
		conn, err := listener.AcceptKCP()
		if err != nil {
			errString := fmt.Sprintf("%s AcceptKCP Failed err=[%s]", funName, err.Error())
			log4plus.Error(errString)
			continue
		}
		var remoteIp string = ""
		var remotePort int = -1
		if strings.ToLower(conn.RemoteAddr().Network()) == strings.ToLower("tcp") {
			if tcpAddr, ok := conn.RemoteAddr().(*net.TCPAddr); ok {
				remoteIp = tcpAddr.IP.String()
				remotePort = tcpAddr.Port
			}
		} else if strings.ToLower(conn.RemoteAddr().Network()) == strings.ToLower("udp") {
			if udpAddr, ok := conn.RemoteAddr().(*net.UDPAddr); ok {
				remoteIp = udpAddr.IP.String()
				remotePort = udpAddr.Port
			}
		}
		log4plus.Info("%s New Connection remoteIp=[%s] remotePort=[%d]---->>>>", funName, remoteIp, remotePort)
		handle := k.CreateHandler()
		linker := NewLinker(handle, remoteIp, remotePort, conn, Server)
		linker.Init(k)
		k.AddLinker(linker)
	}
}

func (k *KCPServer) Start(listen string) error {
	funName := "Start"
	addr, err := net.ResolveUDPAddr("udp", listen)
	if err != nil {
		errString := fmt.Sprintf("%s ResolveUDPAddr Failed err=[%s]", funName, err.Error())
		log4plus.Error(errString)
		return errors.New(errString)
	}
	listener, err := kcp.ListenWithOptions(addr.String(), nil, 10, 3)
	if err != nil {
		errString := fmt.Sprintf("%s ListenWithOptions Failed err=[%s]", funName, err.Error())
		log4plus.Error(errString)
		return errors.New(errString)
	}
	k.localIp = addr.IP.String()
	k.localPort = addr.Port
	log4plus.Info("%s Listen Success localIp=[%s] localPort=[%d]---->>>>", funName, k.localIp, k.localPort)
	go k.accept(listener)
	return nil
}

func (k *KCPServer) Init(event KCPEvent) {
	k.event = event
	go k.timeOut()
}

func (k *KCPServer) forceClose(handle uint64) {
	linker := k.findLinker(handle)
	if linker != nil {
		_ = linker.conn.Close()

		k.lock.Lock()
		defer k.lock.Unlock()
		delete(k.linkers, linker.Handle())
	}
}

func (k *KCPServer) timeOut() {
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

func (k *KCPServer) waitEvent() {
	for {
		select {
		case handle := <-k.disconnectChan:
			k.forceClose(handle)
		}
	}
}

func (k *KCPServer) OnSend(handle uint64, remoteIp string, remotePort int, size int) {
	if k.event != nil {
		k.event.OnSend(handle, remoteIp, remotePort, size)
	}
}

func (k *KCPServer) OnRead(handle uint64, remoteIp string, remotePort int, data []byte, size int) bool {
	if k.event != nil {
		return k.event.OnRead(handle, remoteIp, remotePort, data, size)
	}
	return true
}

func (k *KCPServer) OnDisconnect(handle uint64, remoteIp string, remotePort int) {
	if k.event != nil {
		k.event.OnDisconnect(handle, remoteIp, remotePort)
	}
	k.disconnectChan <- handle
}

func (k *KCPServer) OnError(handle uint64, remoteIp string, remotePort int, err error) {
	if k.event != nil {
		k.event.OnError(handle, remoteIp, remotePort, err)
	}
}

func NewKCPServer() *KCPServer {
	gKCPServer := &KCPServer{
		timeOutSecond:  15,
		curHandle:      1000,
		localIp:        "",
		localPort:      -1,
		linkers:        make(map[uint64]*Linker),
		disconnectChan: make(chan uint64),
	}
	go gKCPServer.waitEvent()
	return gKCPServer
}
