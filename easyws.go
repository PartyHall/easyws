package easyws

import (
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"golang.org/x/exp/slices"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type EasyWS struct {
	SocketTypes              []string
	ConnectionAllowedChecker IsConnectionAllowedChecker

	Sockets         []*Socket
	MessageHandlers map[string]MessageHandler
}

func New() EasyWS {
	return EasyWS{
		SocketTypes:     []string{},
		Sockets:         []*Socket{},
		MessageHandlers: make(map[string]MessageHandler),
	}
}

func NewWithTypes(socketTypes []string, connectionAllowedChecker IsConnectionAllowedChecker) EasyWS {
	easyWs := New()

	easyWs.SocketTypes = socketTypes
	easyWs.ConnectionAllowedChecker = connectionAllowedChecker

	return easyWs
}

func (easyWs *EasyWS) RegisterMessageHandler(handler MessageHandler) {
	easyWs.MessageHandlers[handler.GetType()] = handler
}

func (easyWs *EasyWS) RegisterMessageHandlers(handlers ...MessageHandler) {
	for _, h := range handlers {
		easyWs.RegisterMessageHandler(h)
	}
}

func (easyWs *EasyWS) BroadcastTo(to, msgType string, payload interface{}) {
	for _, socket := range easyWs.Sockets {
		if len(to) > 0 && socket.Type != to {
			continue
		}

		socket.Send(msgType, payload)
	}
}

func (easyWs *EasyWS) Route(w http.ResponseWriter, r *http.Request) {
	socketType := ""
	if len(easyWs.SocketTypes) > 0 {
		vars := mux.Vars(r)
		socketType = strings.ToUpper(vars["type"])
		if !slices.Contains(easyWs.SocketTypes, socketType) {
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		if !easyWs.ConnectionAllowedChecker(socketType, r) {
			w.WriteHeader(http.StatusForbidden)
			return
		}
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	socket := Socket{
		Type: socketType,
		Open: true,
		Conn: conn,
		mtx:  &sync.Mutex{},
	}

	easyWs.Sockets = append(easyWs.Sockets, &socket)

	socket.StartPings()

	go func() {
		for {
			data := SocketMessage{}
			err := conn.ReadJSON(&data)
			if err != nil {
				if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					socket.Open = false
					return
				} else if websocket.IsCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
					socket.Open = false
					return
				}

				continue
			}

			for name, handler := range easyWs.MessageHandlers {
				if name == data.MsgType {
					handler.Do(&socket, data.Payload)
					continue
				}
			}

			fmt.Printf("Unhandled socket message: %v\n", data.MsgType)
		}
	}()
}
