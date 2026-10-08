package routes

import (
	"log"
	"net/http"
	"sync"
	"time"

	"farkle-server/services"

	"github.com/gorilla/websocket"
)

// LobbyClient 把一条大厅 WebSocket 连接适配成 services.LobbyPeer。
type LobbyClient struct {
	hub       *services.Hub
	conn      *websocket.Conn
	send      chan []byte
	closeOnce sync.Once
}

// Push 非阻塞投递消息；队列满时返回 false，由 Hub 负责断开该连接。
func (c *LobbyClient) Push(msg []byte) bool {
	select {
	case c.send <- msg:
		return true
	default:
		return false
	}
}

func (c *LobbyClient) Close() {
	c.closeOnce.Do(func() {
		close(c.send)
	})
}

func (c *LobbyClient) readPump() {
	defer func() {
		c.hub.UnregisterLobby(c)
		_ = c.conn.Close()
	}()
	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})
	for {
		if _, _, err := c.conn.ReadMessage(); err != nil {
			return
		}
	}
}

func (c *LobbyClient) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		_ = c.conn.Close()
	}()
	for {
		select {
		case msg, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func serveLobbyWS(hub *services.Hub, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("lobby upgrade:", err)
		return
	}
	client := &LobbyClient{
		hub:  hub,
		conn: conn,
		send: make(chan []byte, 16),
	}
	hub.RegisterLobby(client)
	go client.writePump()
	go client.readPump()
}
