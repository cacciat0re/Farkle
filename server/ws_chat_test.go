package main

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
)

// 不依赖 PostgreSQL 的整链路测试：真 WebSocket + 真 Room 事件循环
func newChatTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	// 某些受限环境不允许监听本地端口，这时跳过整链路测试而不是直接失败
	probe, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Skipf("当前环境不允许监听本地端口，跳过 WebSocket 测试: %v", err)
	}
	probe.Close()

	cfg := &Config{}
	cfg.Server.Addr = ":0"
	cfg.Database.DSN = "unused"
	cfg.Tables = []TableConfig{{ID: "apple", Name: "Apple", MaxPlayers: MaxPlayersPerTable}}
	hub := NewHub(cfg, nil)

	r := chi.NewRouter()
	r.Get("/ws/{tableId}", func(w http.ResponseWriter, req *http.Request) {
		serveWs(hub, w, req)
	})
	srv := httptest.NewServer(r)
	t.Cleanup(srv.Close)
	return srv
}

func dialTable(t *testing.T, srv *httptest.Server, table string) *websocket.Conn {
	t.Helper()
	url := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws/" + table
	conn, _, err := websocket.DefaultDialer.Dial(url, nil)
	if err != nil {
		t.Fatalf("连接牌桌失败: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	return conn
}

func sendWS(t *testing.T, conn *websocket.Conn, msg ClientMessage) {
	t.Helper()
	if err := conn.WriteJSON(msg); err != nil {
		t.Fatalf("发送消息失败: %v", err)
	}
}

func readWS(t *testing.T, conn *websocket.Conn) ServerMessage {
	t.Helper()
	if err := conn.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
		t.Fatalf("设置读取超时失败: %v", err)
	}
	_, data, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("读取消息失败: %v", err)
	}
	var msg ServerMessage
	if err := json.Unmarshal(data, &msg); err != nil {
		t.Fatalf("解析消息失败: %v", err)
	}
	return msg
}

func TestWebSocketChatAndPresenceEvents(t *testing.T) {
	srv := newChatTestServer(t)

	alice := dialTable(t, srv, "apple")
	sendWS(t, alice, ClientMessage{Type: "JOIN_ROOM", PlayerID: "a", Name: "Alice", RoomID: "apple"})
	if msg := readWS(t, alice); msg.Type != "GAME_STATE" {
		t.Fatalf("Alice 加入后应先收到状态，实际 %s", msg.Type)
	}
	if msg := readWS(t, alice); msg.Chat == nil || msg.Chat.Text != "Alice 加入了牌桌" {
		t.Fatalf("Alice 未收到自己的加入事件: %+v", msg)
	}

	bob := dialTable(t, srv, "apple")
	sendWS(t, bob, ClientMessage{Type: "JOIN_ROOM", PlayerID: "b", Name: "Bob", RoomID: "apple"})
	readWS(t, bob) // GAME_STATE
	if msg := readWS(t, bob); msg.Chat == nil || msg.Chat.Text != "Bob 加入了牌桌" {
		t.Fatalf("Bob 未收到加入事件: %+v", msg)
	}
	readWS(t, alice) // GAME_STATE
	if msg := readWS(t, alice); msg.Chat == nil || msg.Chat.Text != "Bob 加入了牌桌" {
		t.Fatalf("Alice 未收到 Bob 的加入事件: %+v", msg)
	}

	// 聊天广播给全桌，并去掉首尾空白
	sendWS(t, bob, ClientMessage{Type: "CHAT", PlayerID: "b", RoomID: "apple", Text: "  hello  "})
	for name, conn := range map[string]*websocket.Conn{"Alice": alice, "Bob": bob} {
		msg := readWS(t, conn)
		if msg.Type != "CHAT" || msg.Chat == nil || msg.Chat.Kind != "chat" ||
			msg.Chat.Name != "Bob" || msg.Chat.Text != "hello" {
			t.Fatalf("%s 收到的聊天不正确: %+v", name, msg)
		}
	}

	// Bob 断开后，Alice 应收到回到等待状态和离开事件
	bob.Close()
	state := readWS(t, alice)
	if state.Type != "GAME_STATE" || state.State == nil || len(state.State.Players) != 1 {
		t.Fatalf("Bob 断开后 Alice 应收到单人等待状态: %+v", state)
	}
	if msg := readWS(t, alice); msg.Chat == nil || msg.Chat.Text != "Bob 离开了牌桌" {
		t.Fatalf("Alice 未收到 Bob 的离开事件: %+v", msg)
	}
}
