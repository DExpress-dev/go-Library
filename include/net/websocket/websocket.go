package websocket

import (
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	log4plus "github.com/include/log4go"
	"net/http"
)

type OnEvent interface {
	OnRead(messageType int, message []byte) error
}

type Packet struct {
	MessageType int
	Message     []byte
}

type WebSocket struct {
	routePath string
	origin    string
	conn      *websocket.Conn
	recvChan  chan *Packet
	event     OnEvent
}

func (w *WebSocket) upgrader() *websocket.Upgrader {
	//funName := "upgrader"
	var tmpUpgrader = websocket.Upgrader{
		ReadBufferSize:  1024, // 设置读取缓冲区大小
		WriteBufferSize: 1024, // 设置写入缓冲区大小
		CheckOrigin: func(r *http.Request) bool {
			//// 检查请求来源，为安全起见，可以根据业务需求自定义验证逻辑
			//origin := r.Header.Get("Origin")
			//if origin == w.origin {
			//	return true
			//}
			//log4plus.Error(fmt.Sprintf("%s Blocked connection from origin=[%s]", funName, origin))
			//return false
			return true
		},
	}
	return &tmpUpgrader
}

func (w *WebSocket) WriteMessage(messageType int, data []byte) (error, int) {
	funName := "WriteMessage"
	if w.conn != nil {
		if err := w.conn.WriteMessage(int(messageType), data); err != nil {
			errString := fmt.Sprintf("%s w.conn.WriteMessage Failed err=[%s]", funName, err.Error())
			log4plus.Error(errString)
			return err, 0
		}
		log4plus.Info("%s messageType=[%d] len(data)=[%d]", funName, messageType, len(data))
		return nil, len(data)
	}
	return errors.New(fmt.Sprintf("%s w.conn is nil")), 0
}

func (w *WebSocket) OnRead(packet *Packet) error {
	return w.event.OnRead(packet.MessageType, packet.Message)
}

func (w *WebSocket) handle(c *gin.Context) {
	funName := "handle"
	conn, err := w.upgrader().Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		errString := fmt.Sprintf("%s upgrader.Upgrade Failed err=[%s]", funName, err.Error())
		log4plus.Error(errString)
		return
	}
	w.conn = conn
	defer w.conn.Close()
	for {
		messageType, message, err := w.conn.ReadMessage()
		if err != nil {
			errString := fmt.Sprintf("%s w.conn.ReadMessage Failed err=[%s]", funName, err.Error())
			log4plus.Error(errString)
			return
		}
		packet := &Packet{
			MessageType: messageType,
			Message:     message,
		}
		log4plus.Info("%s messageType=[%d] message=[%s]", funName, packet.MessageType, packet.Message)
		w.recvChan <- packet
	}
}

func (w *WebSocket) Init(routePath, origin string, event OnEvent, websocketGroup *gin.RouterGroup) {
	w.routePath = routePath
	w.origin = origin
	w.event = event
	websocketGroup.GET(w.routePath, w.handle)
}

func (w *WebSocket) waitEvent() {
	for {
		select {
		case packet := <-w.recvChan:
			_ = w.OnRead(packet)
		}
	}
}

func NewWebSocket() *WebSocket {
	gWebSocket := &WebSocket{
		routePath: "ws",
		recvChan:  make(chan *Packet, 1024),
	}
	go gWebSocket.waitEvent()
	return gWebSocket
}
