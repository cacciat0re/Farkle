package main

import (
	"encoding/json"
	"testing"
)

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
