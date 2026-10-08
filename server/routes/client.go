package routes

import (
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"farkle-server/models"
	"farkle-server/services"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
)

// Client 把一条牌桌 WebSocket 连接适配成 services.Peer。
type Client struct {
	room      *services.Room
	conn      *websocket.Conn
	send      chan []byte
	playerID  string
	closeOnce sync.Once
}

func (c *Client) PlayerID() string {
	return c.playerID
}

func (c *Client) SetPlayerID(id string) {
	c.playerID = id
}

// Push 非阻塞投递消息；队列满时返回 false，由房间负责断开该连接。
func (c *Client) Push(msg []byte) bool {
	select {
	case c.send <- msg:
		return true
	default:
		return false
	}
}

func (c *Client) Close() {
	c.closeOnce.Do(func() {
		_ = c.conn.Close()
	})
}

func (c *Client) readPump() {
	defer func() {
		c.room.Unregister <- c
		c.Close()
	}()
	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})
	for {
		_, data, err := c.conn.ReadMessage()
		if err != nil {
			break
		}
		var msg models.ClientMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			continue
		}
		c.dispatch(msg)
	}
}

func (c *Client) dispatch(m models.ClientMessage) {
	// 所有游戏逻辑排入 Room 的串行队列，与定时事件统一顺序执行，避免竞态
	action := func() {}
	switch m.Type {
	case "JOIN_ROOM":
		action = func() { c.room.HandleJoin(c, m) }
	case "PLAYER_READY":
		action = func() { c.room.HandleReady(c, m) }
	case "ROLL_DICE":
		action = func() { c.room.HandleRollAgain(c, m) }
	case "ROLL_RESULT":
		action = func() { c.room.HandleRollResult(c, m) }
	case "KEEP_DICE":
		action = func() { c.room.HandleKeep(c, m) }
	case "KEEP_AND_ROLL":
		action = func() { c.room.HandleKeepAndRoll(c, m) }
	case "KEEP_AND_BANK":
		action = func() { c.room.HandleKeepAndBank(c, m) }
	case "SELECT_DICE":
		action = func() { c.room.HandleSelect(c, m) }
	case "BANK_SCORE":
		action = func() { c.room.HandleBank(c, m) }
	case "REMATCH_READY":
		action = func() { c.room.HandleRematchReady(c, m) }
	case "LEAVE_TABLE":
		action = func() { c.room.HandleLeave(c, m) }
	case "CHAT":
		action = func() { c.room.HandleChat(c, m) }
	default:
		return
	}
	select {
	case c.room.Actions <- action:
	case <-c.room.Done():
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.Close()
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

func serveGameWS(hub *services.Hub, w http.ResponseWriter, r *http.Request) {
	tableID := chi.URLParam(r, "tableId")
	room := hub.GetRoom(tableID)
	if room == nil {
		http.Error(w, `{"error":"桌子不存在"}`, http.StatusNotFound)
		return
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("upgrade:", err)
		return
	}
	client := &Client{room: room, conn: conn, send: make(chan []byte, 64)}
	room.Register <- client

	go client.writePump()
	go client.readPump()
}
