package main

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func readRoomBroadcast(t *testing.T, room *Room) ServerMessage {
	t.Helper()
	select {
	case raw := <-room.Broadcast:
		var msg ServerMessage
		if err := json.Unmarshal(raw, &msg); err != nil {
			t.Fatalf("解析房间广播失败: %v", err)
		}
		return msg
	case <-time.After(time.Second):
		t.Fatal("等待房间广播超时")
		return ServerMessage{}
	}
}

func TestJoinLeaveAndChatEvents(t *testing.T) {
	room := NewRoom(TableConfig{ID: "test-table", Name: "Test", MaxPlayers: MaxPlayersPerTable}, nil)
	a := &Client{Send: make(chan []byte, 16)}
	room.Clients[a] = true

	room.HandleJoin(a, ClientMessage{PlayerID: "a", Name: "Alice"})
	if msg := readRoomBroadcast(t, room); msg.Type != "GAME_STATE" {
		t.Fatalf("加入后应先广播状态，实际 %s", msg.Type)
	}
	join := readRoomBroadcast(t, room)
	if join.Type != "CHAT" || join.Chat == nil || join.Chat.Kind != "system" || join.Chat.Text != "Alice 加入了牌桌" {
		t.Fatalf("加入事件不正确: %+v", join)
	}

	// 聊天内容去掉首尾空白后广播
	room.HandleChat(a, ClientMessage{PlayerID: "a", Text: "  晚上好  "})
	chat := readRoomBroadcast(t, room)
	if chat.Type != "CHAT" || chat.Chat == nil || chat.Chat.Kind != "chat" ||
		chat.Chat.Name != "Alice" || chat.Chat.Text != "晚上好" {
		t.Fatalf("聊天广播不正确: %+v", chat)
	}

	// 空消息不广播
	room.HandleChat(a, ClientMessage{PlayerID: "a", Text: "   "})
	select {
	case raw := <-room.Broadcast:
		t.Fatalf("空消息不应广播: %s", raw)
	default:
	}

	// 超长消息按字符截断
	room.HandleChat(a, ClientMessage{PlayerID: "a", Text: strings.Repeat("骰", MaxChatLength+50)})
	truncated := readRoomBroadcast(t, room)
	if truncated.Chat == nil || len([]rune(truncated.Chat.Text)) != MaxChatLength {
		t.Fatalf("超长聊天应截断到 %d 个字符", MaxChatLength)
	}

	room.removePlayer(a)
	if msg := readRoomBroadcast(t, room); msg.Type != "GAME_STATE" {
		t.Fatalf("离开后应先广播状态，实际 %s", msg.Type)
	}
	leave := readRoomBroadcast(t, room)
	if leave.Type != "CHAT" || leave.Chat == nil || leave.Chat.Text != "Alice 离开了牌桌" {
		t.Fatalf("离开事件不正确: %+v", leave)
	}
}

func assertClientError(t *testing.T, c *Client, want string) {
	t.Helper()
	select {
	case raw := <-c.Send:
		var msg ServerMessage
		if err := json.Unmarshal(raw, &msg); err != nil {
			t.Fatalf("解析服务端消息失败: %v", err)
		}
		if msg.Type != "ERROR" || msg.Error != want {
			t.Fatalf("期望 ERROR %q，实际 %+v", want, msg)
		}
	default:
		t.Fatalf("期望收到错误 %q，但没有收到任何消息", want)
	}
}

func TestMustKeepDiceBeforeRollOrBank(t *testing.T) {
	room := NewRoom(TableConfig{ID: "test-table", Name: "Test", MaxPlayers: MaxPlayersPerTable}, nil)
	a := &Client{Send: make(chan []byte, 16), PlayerID: "a"}
	b := &Client{Send: make(chan []byte, 16), PlayerID: "b"}
	pa := &Player{ID: "a", Name: "Alice"}
	pb := &Player{ID: "b", Name: "Bob"}
	room.Clients[a] = true
	room.Clients[b] = true
	room.Players["a"] = pa
	room.Players["b"] = pb
	room.Order = []string{"a", "b"}
	room.State.Phase = StateScoring
	room.State.CurrentTurn = 0
	room.State.Players = []*Player{pa, pb}
	room.State.DiceValues = []int{1, 2, 3, 4, 5, 6}
	room.State.KeptThisRoll = false

	// 本次投掷还没锁定任何骰子时，不允许继续投掷
	room.HandleRollAgain(a, ClientMessage{PlayerID: "a"})
	if room.State.Phase != StateScoring {
		t.Fatalf("未锁定不得继续投掷，phase=%s", room.State.Phase)
	}
	assertClientError(t, a, "请先锁定至少一颗得分骰子，再继续投掷")

	// 同理，未锁定也不允许入库
	room.HandleBank(a, ClientMessage{PlayerID: "a"})
	if room.State.Phase != StateScoring {
		t.Fatalf("未锁定不得入库，phase=%s", room.State.Phase)
	}
	assertClientError(t, a, "请先锁定至少一颗得分骰子，再入库")

	// 锁定一颗 1 后，keptThisRoll 置位
	room.HandleKeep(a, ClientMessage{PlayerID: "a", Keep: []int{1}})
	if !room.State.KeptThisRoll {
		t.Fatal("锁定骰子后 keptThisRoll 应为 true")
	}
	if room.State.TurnScore != 100 {
		t.Fatalf("锁定单颗 1 应得 100 分，实际 %d", room.State.TurnScore)
	}

	// 锁定后可以继续投掷，且新一次投掷会重置 keptThisRoll
	room.HandleRollAgain(a, ClientMessage{PlayerID: "a"})
	if room.State.Phase != StateRolling {
		t.Fatalf("锁定后应进入 rolling，实际 %s", room.State.Phase)
	}
	if room.State.KeptThisRoll {
		t.Fatal("新一次投掷必须重置 keptThisRoll")
	}
}

func TestSinglePlayerReadyDoesNotStart(t *testing.T) {
	room := NewRoom(TableConfig{ID: "test-table", Name: "Test", MaxPlayers: MaxPlayersPerTable}, nil)
	client := &Client{Send: make(chan []byte, 4)}

	room.HandleJoin(client, ClientMessage{PlayerID: "a", Name: "Alice"})
	room.HandleReady(client, ClientMessage{PlayerID: "a"})

	if room.State.Phase != StateWaiting {
		t.Fatalf("单玩家准备后仍应等待，实际: %s", room.State.Phase)
	}
	if len(room.State.ReadyPlayers) != 1 || room.State.ReadyPlayers[0] != "a" {
		t.Fatalf("单玩家准备状态未记录: %v", room.State.ReadyPlayers)
	}
}

func TestTableRejectsThirdPlayer(t *testing.T) {
	room := NewRoom(TableConfig{ID: "test-table", Name: "Test", MaxPlayers: MaxPlayersPerTable}, nil)
	a := &Client{Send: make(chan []byte, 4)}
	b := &Client{Send: make(chan []byte, 4)}
	c := &Client{Send: make(chan []byte, 4)}

	room.HandleJoin(a, ClientMessage{PlayerID: "a", Name: "Alice"})
	room.HandleJoin(b, ClientMessage{PlayerID: "b", Name: "Bob"})
	room.HandleJoin(c, ClientMessage{PlayerID: "c", Name: "Cara"})

	select {
	case raw := <-c.Send:
		var msg ServerMessage
		if err := json.Unmarshal(raw, &msg); err != nil {
			t.Fatalf("解析服务端消息失败: %v", err)
		}
		if msg.Type != "ERROR" {
			t.Fatalf("第三人应被拒绝，实际: %+v", msg)
		}
	default:
		t.Fatal("第三人没有被拒绝")
	}
}

func TestJoinRejectedDuringActiveGame(t *testing.T) {
	room := NewRoom(TableConfig{ID: "test-table", Name: "Test", MaxPlayers: MaxPlayersPerTable}, nil)
	a := &Client{Send: make(chan []byte, 4)}
	b := &Client{Send: make(chan []byte, 4)}
	c := &Client{Send: make(chan []byte, 4)}

	room.HandleJoin(a, ClientMessage{PlayerID: "a", Name: "Alice"})
	room.HandleJoin(b, ClientMessage{PlayerID: "b", Name: "Bob"})
	room.startGame()
	room.HandleJoin(c, ClientMessage{PlayerID: "c", Name: "Cara"})

	select {
	case raw := <-c.Send:
		var msg ServerMessage
		if err := json.Unmarshal(raw, &msg); err != nil {
			t.Fatalf("解析服务端消息失败: %v", err)
		}
		if msg.Type != "ERROR" {
			t.Fatalf("对局中加入应被拒绝，实际: %+v", msg)
		}
	default:
		t.Fatal("对局中第三人没有被拒绝")
	}
}

func TestBothPlayersReadyStartGame(t *testing.T) {
	room := NewRoom(TableConfig{ID: "test-table", Name: "Test", MaxPlayers: MaxPlayersPerTable}, nil)
	a := &Client{Send: make(chan []byte, 4)}
	b := &Client{Send: make(chan []byte, 4)}

	room.HandleJoin(a, ClientMessage{PlayerID: "a", Name: "Alice"})
	room.HandleJoin(b, ClientMessage{PlayerID: "b", Name: "Bob"})
	room.HandleReady(a, ClientMessage{PlayerID: "a"})
	if room.State.Phase != StateWaiting {
		t.Fatalf("first ready must keep waiting, got %s", room.State.Phase)
	}
	room.HandleReady(b, ClientMessage{PlayerID: "b"})
	if room.State.Phase != StateRolling {
		t.Fatalf("both ready must start game, got %s", room.State.Phase)
	}
	if len(room.State.ReadyPlayers) != 0 {
		t.Fatalf("readyPlayers must clear on start: %v", room.State.ReadyPlayers)
	}
}

func TestDuplicatePlayerIDRejected(t *testing.T) {
	room := NewRoom(TableConfig{ID: "test-table", Name: "Test", MaxPlayers: MaxPlayersPerTable}, nil)
	original := &Client{Send: make(chan []byte, 4)}
	duplicate := &Client{Send: make(chan []byte, 4)}
	room.Clients[original] = true
	room.Clients[duplicate] = true

	room.HandleJoin(original, ClientMessage{PlayerID: "a", Name: "Alice"})
	room.HandleJoin(duplicate, ClientMessage{PlayerID: "a", Name: "Alice"})

	select {
	case raw := <-duplicate.Send:
		var msg ServerMessage
		if err := json.Unmarshal(raw, &msg); err != nil {
			t.Fatalf("解析服务端消息失败: %v", err)
		}
		if msg.Type != "ERROR" {
			t.Fatalf("重复玩家 ID 应被拒绝，实际: %+v", msg)
		}
	default:
		t.Fatal("重复玩家 ID 没有被拒绝")
	}
}

func TestRematchRequiresBothPlayers(t *testing.T) {
	room := NewRoom(TableConfig{ID: "test-table", Name: "Test", MaxPlayers: MaxPlayersPerTable}, nil)
	a := &Client{Send: make(chan []byte, 4), PlayerID: "a"}
	b := &Client{Send: make(chan []byte, 4), PlayerID: "b"}
	pa := &Player{ID: "a", Name: "Alice", Score: 3000}
	pb := &Player{ID: "b", Name: "Bob", Score: 1750}
	room.Clients[a] = true
	room.Clients[b] = true
	room.Players["a"] = pa
	room.Players["b"] = pb
	room.Order = []string{"a", "b"}
	room.State.Phase = StateGameOver
	room.State.Players = []*Player{pa, pb}
	room.State.Winner = pa.Name

	room.HandleRematchReady(a, ClientMessage{PlayerID: "a"})
	if room.State.Phase != StateGameOver {
		t.Fatalf("only one player ready must keep gameOver, got %s", room.State.Phase)
	}
	if len(room.State.RematchReady) != 1 || room.State.RematchReady[0] != "a" {
		t.Fatalf("unexpected rematchReady: %v", room.State.RematchReady)
	}

	room.HandleRematchReady(b, ClientMessage{PlayerID: "b"})
	if room.State.Phase != StateWaiting {
		t.Fatalf("both players ready must reset to waiting, got %s", room.State.Phase)
	}
	if len(room.State.RematchReady) != 0 {
		t.Fatalf("rematchReady must be cleared, got %v", room.State.RematchReady)
	}
	if pa.Score != 0 || pb.Score != 0 {
		t.Fatalf("scores must reset, got %d and %d", pa.Score, pb.Score)
	}
}

func TestWaitingRoomLeaveRequiresTenSeconds(t *testing.T) {
	room := NewRoom(TableConfig{ID: "test-table", Name: "Test", MaxPlayers: MaxPlayersPerTable}, nil)
	client := &Client{Send: make(chan []byte, 4)}
	room.Clients[client] = true
	room.HandleJoin(client, ClientMessage{PlayerID: "a", Name: "Alice"})
	room.Players["a"].JoinedAt = nowMs()

	room.HandleLeave(client, ClientMessage{PlayerID: "a"})
	if len(room.Players) != 1 {
		t.Fatal("player must not leave before ten seconds")
	}

	room.Players["a"].JoinedAt = nowMs() - 10001
	room.HandleLeave(client, ClientMessage{PlayerID: "a"})
	if len(room.Players) != 0 {
		t.Fatal("player should leave after ten seconds")
	}
}
