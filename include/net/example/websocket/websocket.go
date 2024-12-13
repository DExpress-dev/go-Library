package websocket

import (
	"fmt"
	"github.com/gin-gonic/gin"
	log4plus "github.com/include/log4go"
	"github.com/include/middleware"
	"github.com/include/net/websocket"
	"time"
)

type Websocket struct {
	socket *websocket.WebSocket
	webGin *gin.Engine
}

var gWebsocket *Websocket

func (w *Websocket) OnRead(messageType int, message []byte) error {
	log4plus.Info(fmt.Sprintf("messageType=[%d] message=[%s]", messageType, message))
	return nil
}

func (w *Websocket) sends() {
	for {
		time.Sleep(time.Duration(30) * time.Second)
		_, _ = w.socket.WriteMessage(1, []byte("hello my name is Ethan"))
	}
}

func (w *Websocket) userStart() {
	userGroup := w.webGin.Group("/user")
	{
		w.socket.Init("ws", "http://allowed-origin.com", w, userGroup)
	}
	if err := w.webGin.Run("0.0.0.0:8083"); err != nil {
		log4plus.Error("start Run Failed Not Use Http Error=[%s]", err.Error())
		return
	}
}

func SingtonWebsocket() *Websocket {
	if gWebsocket == nil {
		gWebsocket = &Websocket{}
		gWebsocket.webGin = gin.Default()
		gWebsocket.webGin.Use(middleware.Cors())
		gin.SetMode(gin.DebugMode)
		gWebsocket.socket = websocket.NewWebSocket()
		go gWebsocket.userStart()
		go gWebsocket.sends()
	}
	return gWebsocket
}
