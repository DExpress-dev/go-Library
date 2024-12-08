package main

import (
	"fmt"
	log4plus "github.com/include/log4go"
	"github.com/include/net/tcp"
)

type Client struct {
	tcpClient *tcp.TCPClient
	handle    uint64
}

func (k *Client) OnSend(handle uint64, remoteIp string, remotePort int, size int) {
	log4plus.Info("OnSend handle=[%d] remoteIp=[%s] remotePort=[%d] size=[%d]", handle, remoteIp, remotePort, size)
}

func (k *Client) OnRead(handle uint64, remoteIp string, remotePort int, data []byte, size int) bool {
	log4plus.Info("OnRead handle=[%d] remoteIp=[%s] remotePort=[%d] size=[%d]", handle, remoteIp, remotePort, size)
	return true
}

func (k *Client) OnDisconnect(handle uint64, remoteIp string, remotePort int) {
	log4plus.Info("OnDisconnect handle=[%d] remoteIp=[%s] remotePort=[%d]", handle, remoteIp, remotePort)
}

func (k *Client) OnError(handle uint64, remoteIp string, remotePort int, err error) {
	log4plus.Info("OnError handle=[%d] remoteIp=[%s] remotePort=[%d] err=[%s]", handle, remoteIp, remotePort, err.Error())
}

func (k *Client) Send(message string) {
	k.tcpClient.Send(k.handle, []byte(message))
}

func (k *Client) Start() bool {
	k.tcpClient = tcp.NewTCPClient()
	if k.tcpClient == nil {
		errString := fmt.Sprintf("NewTCPClient Failed ")
		log4plus.Error(errString)
		return false
	}
	k.tcpClient.Init(k)
	err, handle := k.tcpClient.Start("192.168.159.145:41002")
	if err != nil {
		log4plus.Error("Start Failed err=[%s]", err.Error())
		return false
	}
	log4plus.Info("Start Success handle=[%d]", handle)
	k.handle = handle
	return true
}

func NewClient() *Client {
	gClient := &Client{}
	return gClient
}
