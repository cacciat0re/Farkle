package main

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 60 * time.Second
	pingPeriod     = 45 * time.Second
	maxMessageSize = 4096
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true }, // 开发阶段放开跨域
}

type Client struct {
	Room     *Room
	Conn     *websocket.Conn
	Send     chan []byte
	PlayerID string
}

func (c *Client) readPump() {
	defer func() {
		c.Room.Unregister <- c
		c.Conn.Close()
	}()
	c.Conn.SetReadLimit(maxMessageSize)
	c.Conn.SetReadDeadline(time.Now().Add(pongWait))
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})
	for {
		_, data, err := c.Conn.ReadMessage()
		if err != nil {
			break
		}
		var msg ClientMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			continue
		}
		c.dispatch(msg)
	}
}

func (c *Client) dispatch(m ClientMessage) {
	// 所有游戏逻辑排入 Room 的串行队列，与定时事件统一顺序执行，避免竞态
	action := func() {}
	switch m.Type {
	case "JOIN_ROOM":
		action = func() { c.Room.HandleJoin(c, m) }
	case "PLAYER_READY":
		action = func() { c.Room.HandleReady(c, m) }
	case "ROLL_DICE":
		action = func() { c.Room.HandleRollAgain(c, m) }
	case "ROLL_RESULT":
		action = func() { c.Room.HandleRollResult(c, m) }
	case "KEEP_DICE":
		action = func() { c.Room.HandleKeep(c, m) }
	case "BANK_SCORE":
		action = func() { c.Room.HandleBank(c, m) }
	case "REMATCH_READY":
		action = func() { c.Room.HandleRematchReady(c, m) }
	case "LEAVE_TABLE":
		action = func() { c.Room.HandleLeave(c, m) }
	default:
		return
	}
	select {
	case c.Room.Actions <- action:
	case <-c.Room.done:
	}
}

func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()
	for {
		select {
		case msg, ok := <-c.Send:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.Conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

func serveWs(hub *Hub, w http.ResponseWriter, r *http.Request) {
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
	client := &Client{Room: room, Conn: conn, Send: make(chan []byte, 64)}
	room.Register <- client

	go client.writePump()
	go client.readPump()
}
