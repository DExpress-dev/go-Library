package tcp

import (
	"encoding/json"
	"fmt"
	log4plus "github.com/include/log4go"
	"net"
	"time"
	"unsafe"
)

type PacketMode uint8

const (
	HeartBeat PacketMode = iota
	Message
)

type ServerMode uint8

const (
	Server ServerMode = iota
	Client
)

type Packet struct {
	Mode PacketMode
	Size int
}

type Linker struct {
	serverMode  ServerMode
	remoteIp    string
	remotePort  int
	handle      uint64
	conn        net.Conn
	event       TCPEvent
	recvSize    uint64
	sendSize    uint64
	heartbeat   time.Time
	recvChan    chan []byte
	sendChan    chan []byte
	closeChan   chan error
	spareBuffer []byte
	spareSize   int
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

func (l *Linker) Heartbeat() time.Time {
	return l.heartbeat
}

func (l *Linker) assemblyPacket(data []byte) []byte {
	packet := Packet{
		Mode: Message,
		Size: len(data),
	}
	buffer, _ := json.Marshal(packet)
	buffer = append(buffer, data...)
	return buffer
}

func (l *Linker) Send(data []byte) (error, int) {
	l.sendChan <- l.assemblyPacket(data)
	return nil, len(data)
}

func (l *Linker) Recv() {
	funName := "Recv"
	for {
		buffer := make([]byte, 4*1024)
		nRet, err := l.conn.Read(buffer)
		if err != nil {
			errString := fmt.Sprintf("%s Read Failed handle=[%d] err=[%s]", funName, l.handle, err.Error())
			log4plus.Error(errString)
			l.closeChan <- err
			return
		}
		if nRet == 0 {
			continue
		} else {
			log4plus.Info("%s nRet=[%d]", funName, nRet)
			l.recvChan <- buffer[:nRet]
		}
	}
}

func (l *Linker) heartbeatMsg(data []byte) {
	l.heartbeat = time.Now()
	l.sendChan <- data
}

func (l *Linker) dataMsg(data []byte) {
	if l.event != nil {
		if !l.event.OnRead(l.handle, l.remoteIp, l.remotePort, data, len(data)) {
			l.event.OnDisconnect(l.handle, l.remoteIp, l.remotePort)
			return
		}
	}
	l.heartbeat = time.Now()
	l.recvSize += uint64(len(data))
}

func (l *Linker) parseData(data []byte) {
	l.spareSize += len(data)
	l.spareBuffer = append(l.spareBuffer, data...)
	for {
		//Get packet header
		header := Packet{}
		headerSize := unsafe.Sizeof(header)
		if l.spareSize < int(headerSize) {
			return
		}
		//Parse packet header
		headerBuffer := l.spareBuffer[:headerSize]
		if err := json.Unmarshal(headerBuffer, &header); err != nil {
			l.closeChan <- err
			return
		}
		//Determine protocol type
		if header.Mode == HeartBeat {
			//heartbeat
			l.heartbeatMsg(headerBuffer)
			//Reduce buffer data length
			l.spareSize -= int(headerSize)
			//Move buffer
			copy(l.spareBuffer, l.spareBuffer[int(headerSize):])
		} else if header.Mode == Message {
			//message
			if l.spareSize < header.Size+int(headerSize) {
				return
			}
			messageBuffer := l.spareBuffer[int(headerSize) : int(headerSize)+header.Size]
			l.dataMsg(messageBuffer)
			//Reduce buffer data length
			l.spareSize = l.spareSize - (int(headerSize) + header.Size)
			//Move buffer
			copy(l.spareBuffer, l.spareBuffer[int(headerSize)+header.Size:])
		}
	}
}

func (l *Linker) sendBuffer(data []byte) {
	funName := "sendBuffer"
	nRet, err := l.conn.Write(data)
	if err != nil {
		errString := fmt.Sprintf("%s Write Failed handle=[%d] err=[%s]", funName, l.handle, err.Error())
		log4plus.Error(errString)
		l.closeChan <- err
		return
	}
	if l.event != nil {
		l.event.OnSend(l.handle, l.remoteIp, l.remotePort, nRet)
	}
	l.sendSize += uint64(nRet)
}

func (l *Linker) linkerClose(err error) {
	if l.event != nil {
		l.event.OnError(l.handle, l.remoteIp, l.remotePort, err)
		l.event.OnDisconnect(l.handle, l.remoteIp, l.remotePort)
	}
}

func (l *Linker) waitData() {
	for {
		select {
		case msg := <-l.recvChan:
			l.parseData(msg)
		case msg := <-l.sendChan:
			l.sendBuffer(msg)
		case err := <-l.closeChan:
			l.linkerClose(err)
			break
		}
	}
}

func (l *Linker) sendHeartbeat() {
	for {
		time.Sleep(time.Duration(5) * time.Second)
		l.sendChan <- l.assemblyPacket([]byte(""))
	}
}

func (l *Linker) Init(event TCPEvent) {
	l.event = event
	l.heartbeat = time.Now()
	go l.Recv()
	go l.waitData()
	if l.serverMode == Client {
		go l.sendHeartbeat()
	}
}

func NewLinker(handle uint64, remoteIp string, remotePort int, conn net.Conn, mode ServerMode) *Linker {
	funName := "NewLinker"
	linker := &Linker{
		serverMode: mode,
		handle:     handle,
		remoteIp:   remoteIp,
		remotePort: remotePort,
		conn:       conn,
		recvSize:   0,
		sendSize:   0,
		recvChan:   make(chan []byte, 1024),
		sendChan:   make(chan []byte, 1024),
		closeChan:  make(chan error),
	}
	log4plus.Info("%s handle=[%d] remoteIp=[%s] remotePort=[%d]", funName, linker.handle, linker.remoteIp, linker.remotePort)
	return linker
}
