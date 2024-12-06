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
	OnError(handle uint64, remoteIp string, remotePort int)
}

type KCPServer struct {
	lock      sync.Mutex
	linkers   map[uint64]*Linker
	curHandle uint64
	localIp   string
	localPort int
	event     KCPEvent
}

var gKCPServer *KCPServer

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

func (k *KCPServer) AddLinker(linker *Linker) error {
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

func (k *KCPServer) Send(handle uint64, data []byte) error {
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

func (k *KCPServer) acceptKCP(listener *kcp.Listener) {
	funName := "acceptKCP"
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
		linker := NewLinker(handle, remoteIp, remotePort, conn)
		linker.Init(gKCPServer)
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

	go k.acceptKCP(listener)
	return nil
}

func (k *KCPServer) Init(event KCPEvent) error {
	funName := "Init"
	if nil == gKCPClient {
		errString := fmt.Sprintf("%s Init Failed Object is nil", funName)
		log4plus.Error(errString)
		return errors.New(errString)
	}
	gKCPClient.event = event
	return nil
}

func (k *KCPServer) OnSend(handle uint64, remoteIp string, remotePort int, size int) {
	k.event.OnSend(handle, remoteIp, remotePort, size)
}

func (k *KCPServer) OnRead(handle uint64, remoteIp string, remotePort int, data []byte, size int) bool {
	return k.event.OnRead(handle, remoteIp, remotePort, data, size)
}

func (k *KCPServer) OnDisconnect(handle uint64, remoteIp string, remotePort int) {
	k.event.OnDisconnect(handle, remoteIp, remotePort)
}

func (k *KCPServer) OnError(handle uint64, remoteIp string, remotePort int) {
	k.event.OnError(handle, remoteIp, remotePort)
}

func SingtonKCPServer() *KCPServer {
	if nil == gKCPServer {
		gKCPServer = &KCPServer{
			curHandle: 1000,
			localIp:   "",
			localPort: -1,
			linkers:   make(map[uint64]*Linker),
		}
	}
	return gKCPServer
}
