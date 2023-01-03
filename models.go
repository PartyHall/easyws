package easyws

import (
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

const (
	MSG_TYPE_ERR_MODAL    = "ERR_MODAL"
	MSG_TYPE_ERR_SNACKBAR = "ERR_SNACKBAR"
)

type SocketMessage struct {
	MsgType string      `json:"type"`
	Payload interface{} `json:"payload"`
}

type Socket struct {
	Type string
	Open bool
	Conn *websocket.Conn

	mtx *sync.Mutex
}

func (s *Socket) Send(msgType string, data interface{}) error {
	s.mtx.Lock()
	err := s.Conn.WriteJSON(SocketMessage{
		MsgType: msgType,
		Payload: data,
	})
	s.mtx.Unlock()

	return err
}

func (s *Socket) StartPings() {
	go func() {
		for s.Open {
			time.Sleep(1 * time.Second)
			s.Send("PING", time.Now().Format("2006-01-02 15:04:05"))
		}
	}()
}

type MessageHandler interface {
	GetType() string
	Do(s *Socket, payload interface{})
}
