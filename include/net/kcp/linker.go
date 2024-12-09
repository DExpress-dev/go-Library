package kcp

import (
	"bytes"
	"encoding/binary"
	"fmt"
	log4plus "github.com/include/log4go"
	"net"
	"time"
)

type ServerMode uint8

const (
	Server ServerMode = iota
	Client
)

type PacketMode uint8

const (
	HeartBeat PacketMode = iota
	Message
)

type Packet struct {
	Mode PacketMode
	Size int32
}

type Linker struct {
	serverMode  ServerMode
	remoteIp    string
	remotePort  int
	handle      uint64
	conn        net.Conn
	buffer      []byte
	event       KCPEvent
	recvSize    int32
	sendSize    int32
	heartbeat   time.Time
	recvChan    chan []byte
	sendChan    chan []byte
	closeChan   chan error
	spareBuffer []byte
	spareSize   int32
	headerSize  int32
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

func (l *Linker) SendSize() int32 {
	return l.sendSize
}

func (l *Linker) RecvSize() int32 {
	return l.recvSize
}

func (l *Linker) Heartbeat() time.Time {
	return l.heartbeat
}

func (l *Linker) assemblyPacket(mode PacketMode, data []byte) []byte {
	packet := Packet{
		Mode: mode,
		Size: int32(len(data)),
	}
	buffer, _ := l.serializePacket(packet)
	buffer = append(buffer, data...)
	return buffer
}

func (l *Linker) Send(data []byte) (error, int) {
	l.sendChan <- l.assemblyPacket(Message, data)
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
	if l.serverMode == Server {
		l.sendChan <- data
	}
}

func (l *Linker) dataMsg(data []byte) {
	if l.event != nil {
		if !l.event.OnRead(l.handle, l.remoteIp, l.remotePort, data, len(data)) {
			l.event.OnDisconnect(l.handle, l.remoteIp, l.remotePort)
			return
		}
	}
	l.heartbeat = time.Now()
	l.recvSize += int32(len(data))
}

// Serialize to binary
func (l *Linker) getHeaderSize() int32 {
	packet := Packet{
		Mode: HeartBeat,
		Size: 1024,
	}
	buf := new(bytes.Buffer)
	_ = binary.Write(buf, binary.LittleEndian, packet.Mode)
	_ = binary.Write(buf, binary.LittleEndian, packet.Size)
	return int32(buf.Len())
}

// Serialize to binary
func (l *Linker) serializePacket(p Packet) ([]byte, error) {
	buf := new(bytes.Buffer)
	if err := binary.Write(buf, binary.BigEndian, p.Mode); err != nil {
		return nil, err
	}
	if err := binary.Write(buf, binary.BigEndian, int32(p.Size)); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Deserialize from binary
func (l *Linker) deserializePacket(data []byte) (Packet, error) {
	buf := bytes.NewReader(data)
	var p Packet
	if err := binary.Read(buf, binary.BigEndian, &p.Mode); err != nil {
		return Packet{}, err
	}
	var size int32
	if err := binary.Read(buf, binary.BigEndian, &size); err != nil {
		return Packet{}, err
	}
	p.Size = size
	return p, nil
}

func (l *Linker) parseData(data []byte) {
	l.spareSize += int32(len(data))
	l.spareBuffer = append(l.spareBuffer, data...)
	for {
		//check spareSize
		if l.spareSize <= 0 {
			break
		}
		//Get packet header
		if l.spareSize < l.headerSize {
			return
		}
		//Parse packet header
		headerBuffer := l.spareBuffer[:l.headerSize]
		header, err := l.deserializePacket(headerBuffer)
		if err != nil {
			l.closeChan <- err
			return
		}
		//Determine protocol type
		if header.Mode == HeartBeat {
			//heartbeat
			l.heartbeatMsg(headerBuffer)
			//Reduce buffer data length
			l.spareSize -= l.headerSize
			//Move buffer
			copy(l.spareBuffer, l.spareBuffer[l.headerSize:])
			//clear slice
			l.spareBuffer = l.spareBuffer[:l.spareSize]
		} else if header.Mode == Message {
			//message
			if l.spareSize < header.Size+l.headerSize {
				return
			}
			messageBuffer := l.spareBuffer[l.headerSize : l.headerSize+header.Size]
			l.dataMsg(messageBuffer)
			//Reduce buffer data length
			l.spareSize = l.spareSize - (l.headerSize + header.Size)
			//Move buffer copy(A, A[start:end])
			copy(l.spareBuffer, l.spareBuffer[l.headerSize+header.Size:])
			//clear slice A = A[:end-start]
			l.spareBuffer = l.spareBuffer[:l.spareSize]
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
	l.sendSize += int32(nRet)
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
		l.sendChan <- l.assemblyPacket(HeartBeat, []byte(""))
	}
}

func (l *Linker) Init(event KCPEvent) {
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
	linker.headerSize = linker.getHeaderSize()
	log4plus.Info("%s handle=[%d] remoteIp=[%s] remotePort=[%d] headerSize=[%d]",
		funName, linker.handle, linker.remoteIp, linker.remotePort, linker.headerSize)
	return linker
}
