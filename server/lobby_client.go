package main

import (
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

type LobbyClient struct {
	Hub  *Hub
	Conn *websocket.Conn
	Send chan []byte
}

func (c *LobbyClient) readPump() {
	defer func() {
		c.Hub.UnregisterLobby(c)
		c.Conn.Close()
	}()
	c.Conn.SetReadLimit(maxMessageSize)
	c.Conn.SetReadDeadline(time.Now().Add(pongWait))
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})
	for {
		if _, _, err := c.Conn.ReadMessage(); err != nil {
			return
		}
	}
}

func (c *LobbyClient) writePump() {
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

func serveLobbyWs(hub *Hub, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println("lobby upgrade:", err)
		return
	}
	client := &LobbyClient{
		Hub:  hub,
		Conn: conn,
		Send: make(chan []byte, 16),
	}
	hub.RegisterLobby(client)
	go client.writePump()
	go client.readPump()
}
