package tcp

import (
	"fmt"
	log4plus "github.com/include/log4go"
	"github.com/nGPU/include/net/tcp"
)

type Server struct {
	tcpServer *tcp.TCPServer
}

func (k *Server) OnSend(handle uint64, remoteIp string, remotePort int, size int) {
	log4plus.Info("OnSend handle=[%d] remoteIp=[%s] remotePort=[%d] size=[%d]", handle, remoteIp, remotePort, size)
}

func (k *Server) OnRead(handle uint64, remoteIp string, remotePort int, data []byte, size int) bool {
	log4plus.Info("OnRead handle=[%d] remoteIp=[%s] remotePort=[%d] size=[%d]", handle, remoteIp, remotePort, size)
	return true
}

func (k *Server) OnDisconnect(handle uint64, remoteIp string, remotePort int) {
	log4plus.Info("OnDisconnect handle=[%d] remoteIp=[%s] remotePort=[%d]", handle, remoteIp, remotePort)
}

func (k *Server) OnError(handle uint64, remoteIp string, remotePort int, err error) {
	log4plus.Info("OnError handle=[%d] remoteIp=[%s] remotePort=[%d] err=[%s]", handle, remoteIp, remotePort, err.Error())
}

func (k *Server) Start() bool {
	k.tcpServer = tcp.NewTCPServer()
	if k.tcpServer == nil {
		errString := fmt.Sprintf("NewTCPServer Failed ")
		log4plus.Error(errString)
		return false
	}
	k.tcpServer.Init(k)
	if err := k.tcpServer.Start(":41002"); err != nil {
		log4plus.Error("Start Failed err=[%s]", err.Error())
		return false
	}
	return true
}

func NewServer() *Server {
	gServer := &Server{}
	return gServer
}
