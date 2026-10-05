package main

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

// readUntilType 一直读到指定类型的消息；期间容忍聊天/状态等其它广播。
// 连接被服务端断开时会直接失败，这正是"投掷后卡死"的回归点。
func readUntilType(t *testing.T, conn *websocket.Conn, want string, deadline time.Duration) ServerMessage {
	t.Helper()
	end := time.Now().Add(deadline)
	for {
		if err := conn.SetReadDeadline(end); err != nil {
			t.Fatalf("设置读取超时失败: %v", err)
		}
		_, data, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("等待 %s 时连接被断开: %v", want, err)
		}
		var msg ServerMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			t.Fatalf("解析消息失败: %v", err)
		}
		if msg.Type == want {
			return msg
		}
	}
}

// readStatePhase 读到指定阶段的 GAME_STATE
func readStatePhase(t *testing.T, conn *websocket.Conn, phase string, deadline time.Duration) GameState {
	t.Helper()
	end := time.Now().Add(deadline)
	for {
		msg := readUntilType(t, conn, "GAME_STATE", time.Until(end))
		if msg.State != nil && msg.State.Phase == phase {
			return *msg.State
		}
	}
}

// setupRollingTable 让两名玩家入座并准备，返回投掷者连接与旁观者连接
func setupRollingTable(t *testing.T, srv *httptest.Server) (*websocket.Conn, *websocket.Conn, ServerMessage) {
	t.Helper()
	alice := dialTable(t, srv, "apple")
	sendWS(t, alice, ClientMessage{Type: "JOIN_ROOM", PlayerID: "a", Name: "Alice", RoomID: "apple"})
	readUntilType(t, alice, "GAME_STATE", 2*time.Second)

	bob := dialTable(t, srv, "apple")
	sendWS(t, bob, ClientMessage{Type: "JOIN_ROOM", PlayerID: "b", Name: "Bob", RoomID: "apple"})
	readUntilType(t, bob, "GAME_STATE", 2*time.Second)
	readUntilType(t, alice, "GAME_STATE", 2*time.Second)

	sendWS(t, alice, ClientMessage{Type: "PLAYER_READY", PlayerID: "a", RoomID: "apple"})
	readUntilType(t, alice, "GAME_STATE", 2*time.Second)
	sendWS(t, bob, ClientMessage{Type: "PLAYER_READY", PlayerID: "b", RoomID: "apple"})

	rolled := readUntilType(t, alice, "ROLL_REQUESTED", 2*time.Second)
	if len(rolled.Roll) != 2 {
		t.Fatalf("ROLL_REQUESTED 应带 [骰子数, 种子]，实际 %v", rolled.Roll)
	}
	return alice, bob, rolled
}

func trajectoryFor(dice, frames int) []float64 {
	out := make([]float64, frames*dice*TrajectoryNumbersPerDie)
	for i := range out {
		// 刻意用高精度小数，模拟真实物理数值（也是消息体积的主要来源）
		out[i] = float64(i%97)/7 + 0.123456789
	}
	return out
}

// 投掷者上报的轨迹消息有几十 KB，必须能被服务端收下并且不断开连接。
func TestWebSocketAcceptsLargeTrajectoryResult(t *testing.T) {
	srv := newChatTestServer(t)
	alice, bob, rolled := setupRollingTable(t, srv)
	if rolled.Roll[0] != 6 {
		t.Fatalf("开局应投 6 颗，实际 %d", rolled.Roll[0])
	}

	const frames = 60 // 6 颗 × 60 帧 × 7 个数 ≈ 45KB JSON，远超旧的 4KB 读上限
	traj := trajectoryFor(6, frames)
	sendWS(t, alice, ClientMessage{
		Type: "ROLL_RESULT", PlayerID: "a", RoomID: "apple",
		Roll: []int{1, 2, 3, 4, 5, 6}, Trajectory: traj, StepsPerFrame: 3,
	})

	state := readStatePhase(t, alice, StateScoring, 3*time.Second)
	if len(state.Trajectory) != len(traj) || state.TrajectoryDice != 6 || state.StepsPerFrame != 3 {
		t.Fatalf("轨迹应原样广播给全桌: dice=%d len=%d steps=%d",
			state.TrajectoryDice, len(state.Trajectory), state.StepsPerFrame)
	}
	// 旁观者同样要拿到完整轨迹（否则对面看不到投掷过程）
	observed := readStatePhase(t, bob, StateScoring, 3*time.Second)
	if len(observed.Trajectory) != len(traj) {
		t.Fatalf("旁观者应收到完整轨迹，实际 %d", len(observed.Trajectory))
	}
}

// 锁骰后只剩 N 颗，轨迹步长必须按 N×7 校验，不能写死 6×7。
func TestTrajectoryStrideFollowsRemainingDice(t *testing.T) {
	srv := newChatTestServer(t)
	alice, _, _ := setupRollingTable(t, srv)

	sendWS(t, alice, ClientMessage{
		Type: "ROLL_RESULT", PlayerID: "a", RoomID: "apple",
		Roll: []int{1, 2, 3, 4, 5, 6}, Trajectory: trajectoryFor(6, 20), StepsPerFrame: 3,
	})
	readStatePhase(t, alice, StateScoring, 3*time.Second)

	// 锁定一颗 1，剩 5 颗继续投
	sendWS(t, alice, ClientMessage{Type: "KEEP_DICE", PlayerID: "a", RoomID: "apple", Keep: []int{1}})
	kept := readStatePhase(t, alice, StateScoring, 3*time.Second)
	if len(kept.LockedDice) != 1 {
		t.Fatalf("应已锁定 1 颗骰子，实际 %v", kept.LockedDice)
	}

	sendWS(t, alice, ClientMessage{Type: "ROLL_DICE", PlayerID: "a", RoomID: "apple"})
	next := readUntilType(t, alice, "ROLL_REQUESTED", 2*time.Second)
	if next.Roll[0] != 5 {
		t.Fatalf("锁 1 颗后应投 5 颗，实际 %d", next.Roll[0])
	}

	sendWS(t, alice, ClientMessage{
		Type: "ROLL_RESULT", PlayerID: "a", RoomID: "apple",
		Roll: []int{2, 3, 4, 5, 6}, Trajectory: trajectoryFor(5, 20), StepsPerFrame: 3,
	})
	state := readStatePhase(t, alice, StateScoring, 3*time.Second)
	if state.TrajectoryDice != 5 || len(state.Trajectory) != 20*5*TrajectoryNumbersPerDie {
		t.Fatalf("5 颗骰子的轨迹应被接受: dice=%d len=%d", state.TrajectoryDice, len(state.Trajectory))
	}
}
