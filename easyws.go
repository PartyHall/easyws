package easyws

import (
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
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
	OnJoin                   OnJoinEvent

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

func (easyWs *EasyWS) Route(c echo.Context) error {
	socketType := ""
	if len(easyWs.SocketTypes) > 0 {
		socketType = strings.ToUpper(c.Param("type"))
		if !slices.Contains(easyWs.SocketTypes, socketType) {
			return echo.NewHTTPError(http.StatusBadRequest, "Invalid socket type requested")
		}

		if !easyWs.ConnectionAllowedChecker(socketType, &c) {
			return echo.NewHTTPError(http.StatusUnauthorized, "Not authorized")
		}
	}

	conn, err := upgrader.Upgrade(c.Response(), c.Request(), nil)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "Failed to upgrade connection")
	}
	defer conn.Close()

	socket := Socket{
		Type: socketType,
		Open: true,
		Conn: conn,
		mtx:  &sync.Mutex{},
	}

	easyWs.Sockets = append(easyWs.Sockets, &socket)

	socket.StartPings()

	if easyWs.OnJoin != nil {
		easyWs.OnJoin(socketType, &socket)
	}

	for {
		data := SocketMessage{}
		err := conn.ReadJSON(&data)
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				socket.Open = false
				return echo.NewHTTPError(http.StatusInternalServerError, "Unexpected websocket closure")
			} else if websocket.IsCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				socket.Open = false
				return echo.NewHTTPError(http.StatusOK, "Bye bye")
			}

			continue
		}

		switch data.MsgType {
		case "PONG":
			continue
		default:
		}

		found := false
		for name, handler := range easyWs.MessageHandlers {
			if name == data.MsgType {
				handler.Do(&socket, data.Payload)
				found = true
				break
			}
		}

		if !found {
			fmt.Printf("Unhandled socket message: %v\n", data.MsgType)
		}
	}
}
