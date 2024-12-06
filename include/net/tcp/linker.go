package tcp

import (
	"fmt"
	log4plus "github.com/include/log4go"
	"net"
	"time"
)

type Linker struct {
	remoteIp   string
	remotePort int
	handle     uint64
	conn       net.Conn
	buffer     []byte
	event      TCPEvent
	recvSize   uint64
	sendSize   uint64
}

func (l *Linker) Handle() uint64 {
	return l.handle
}

func (l *Linker) Ip() string {
	return l.remoteIp
}

func (l *Linker) Port() int {
	return l.remotePort
}

func (l *Linker) SendSize() uint64 {
	return l.sendSize
}

func (l *Linker) RecvSize() uint64 {
	return l.recvSize
}

func (l *Linker) Send(data []byte) error {
	funName := "Send"
	now := time.Now().Unix()
	defer func() {
		log4plus.Info("%s handler=[%d] remoteIp=[%s] remotePort=[%s] size=[%d]  consumption time=%d(s)",
			funName, l.handle, l.remoteIp, l.remotePort, len(data), time.Now().Unix()-now)
	}()
	nRet, err := l.conn.Write(data)
	if err != nil {
		errString := fmt.Sprintf("%s Write Failed handler=[%d] err=[%s]", funName, l.handle, err.Error())
		log4plus.Error(errString)
		l.event.OnDisconnect(l.handle, l.remoteIp, l.remotePort)
		return err
	}
	l.event.OnSend(l.handle, l.remoteIp, l.remotePort, nRet)
	l.sendSize += uint64(nRet)
	return nil
}

func (l *Linker) Recv() {
	funName := "Recv"
	for {
		nRet, err := l.conn.Read(l.buffer)
		if err != nil {
			errString := fmt.Sprintf("%s Read Failed handler=[%d] err=[%s]", funName, l.handle, err.Error())
			log4plus.Error(errString)
			l.event.OnDisconnect(l.handle, l.remoteIp, l.remotePort)
			return
		}
		if !l.event.OnRead(l.handle, l.remoteIp, l.remotePort, l.buffer, nRet) {
			errString := fmt.Sprintf("%s ReadEvent Failed handler=[%d] err=[%s]", funName, l.handle, err.Error())
			log4plus.Error(errString)
			l.event.OnDisconnect(l.handle, l.remoteIp, l.remotePort)
			return
		}
		l.recvSize += uint64(nRet)
	}
}

func (l *Linker) Init(event TCPEvent) {
	l.event = event
	go l.Recv()
}

func NewLinker(handle uint64, remoteIp string, remotePort int, conn net.Conn) *Linker {
	funName := "NewLinker"
	linker := &Linker{
		handle:     handle,
		remoteIp:   remoteIp,
		remotePort: remotePort,
		conn:       conn,
		recvSize:   0,
		sendSize:   0,
	}
	log4plus.Info("%s handler=[%d] remoteIp=[%s] remotePort=[%d]", funName, linker.handle, linker.remoteIp, linker.remotePort)
	return linker
}
