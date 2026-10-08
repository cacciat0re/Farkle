package services

import (
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"farkle-server/models"
)

type fakePeer struct {
	mu   sync.Mutex
	id   string
	msgs [][]byte
}

func newFakePeer() *fakePeer { return &fakePeer{} }

func (p *fakePeer) PlayerID() string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.id
}

func (p *fakePeer) SetPlayerID(id string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.id = id
}

func (p *fakePeer) Push(msg []byte) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.msgs = append(p.msgs, msg)
	return true
}

func (p *fakePeer) Close() {}

func (p *fakePeer) next(t *testing.T) models.ServerMessage {
	t.Helper()
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.msgs) == 0 {
		t.Fatal("期望收到服务端消息，但没有收到")
	}
	raw := p.msgs[0]
	p.msgs = p.msgs[1:]
	var msg models.ServerMessage
	if err := json.Unmarshal(raw, &msg); err != nil {
		t.Fatalf("解析服务端消息失败: %v", err)
	}
	return msg
}

func assertClientError(t *testing.T, c *fakePeer, want string) {
	t.Helper()
	msg := c.next(t)
	if msg.Type != "ERROR" || msg.Error != want {
		t.Fatalf("期望 ERROR %q，实际 %+v", want, msg)
	}
}

func testTableConfig() models.Table {
	return models.Table{ID: "test-table", Name: "Test", MaxPlayers: models.MaxPlayersPerTable}
}

func readRoomBroadcast(t *testing.T, room *Room) models.ServerMessage {
	t.Helper()
	select {
	case raw := <-room.Broadcast:
		var msg models.ServerMessage
		if err := json.Unmarshal(raw, &msg); err != nil {
			t.Fatalf("解析房间广播失败: %v", err)
		}
		return msg
	case <-time.After(time.Second):
		t.Fatal("等待房间广播超时")
		return models.ServerMessage{}
	}
}

func TestJoinLeaveAndChatEvents(t *testing.T) {
	room := NewRoom(testTableConfig(), nil)
	a := newFakePeer()
	room.Clients[a] = true

	room.HandleJoin(a, models.ClientMessage{PlayerID: "a", Name: "Alice"})
	if msg := readRoomBroadcast(t, room); msg.Type != "GAME_STATE" {
		t.Fatalf("加入后应先广播状态，实际 %s", msg.Type)
	}
	join := readRoomBroadcast(t, room)
	if join.Type != "CHAT" || join.Chat == nil || join.Chat.Kind != "system" || join.Chat.Text != "Alice 加入了牌桌" {
		t.Fatalf("加入事件不正确: %+v", join)
	}

	// 聊天内容去掉首尾空白后广播
	room.HandleChat(a, models.ClientMessage{PlayerID: "a", Text: "  晚上好  "})
	chat := readRoomBroadcast(t, room)
	if chat.Type != "CHAT" || chat.Chat == nil || chat.Chat.Kind != "chat" ||
		chat.Chat.Name != "Alice" || chat.Chat.Text != "晚上好" {
		t.Fatalf("聊天广播不正确: %+v", chat)
	}

	// 空消息不广播
	room.HandleChat(a, models.ClientMessage{PlayerID: "a", Text: "   "})
	select {
	case raw := <-room.Broadcast:
		t.Fatalf("空消息不应广播: %s", raw)
	default:
	}

	// 超长消息按字符截断
	room.HandleChat(a, models.ClientMessage{PlayerID: "a", Text: strings.Repeat("骰", MaxChatLength+50)})
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

func TestMustKeepDiceBeforeRollOrBank(t *testing.T) {
	room := NewRoom(testTableConfig(), nil)
	a := newFakePeer()
	a.id = "a"
	b := newFakePeer()
	b.id = "b"
	pa := &models.Player{ID: "a", Name: "Alice"}
	pb := &models.Player{ID: "b", Name: "Bob"}
	room.Clients[a] = true
	room.Clients[b] = true
	room.Players["a"] = pa
	room.Players["b"] = pb
	room.Order = []string{"a", "b"}
	room.State.Phase = models.PhaseScoring
	room.State.CurrentTurn = 0
	room.State.Players = []*models.Player{pa, pb}
	room.State.DiceValues = []int{1, 2, 3, 4, 5, 6}
	room.State.KeptThisRoll = false

	// 本次投掷还没锁定任何骰子时，不允许继续投掷
	room.HandleRollAgain(a, models.ClientMessage{PlayerID: "a"})
	if room.State.Phase != models.PhaseScoring {
		t.Fatalf("未锁定不得继续投掷，phase=%s", room.State.Phase)
	}
	assertClientError(t, a, "请先锁定至少一颗得分骰子，再继续投掷")

	// 同理，未锁定也不允许入库
	room.HandleBank(a, models.ClientMessage{PlayerID: "a"})
	if room.State.Phase != models.PhaseScoring {
		t.Fatalf("未锁定不得入库，phase=%s", room.State.Phase)
	}
	assertClientError(t, a, "请先锁定至少一颗得分骰子，再入库")

	// 锁定一颗 1 后，keptThisRoll 置位
	room.HandleKeep(a, models.ClientMessage{PlayerID: "a", Keep: []int{1}})
	if !room.State.KeptThisRoll {
		t.Fatal("锁定骰子后 keptThisRoll 应为 true")
	}
	if room.State.TurnScore != 100 {
		t.Fatalf("锁定单颗 1 应得 100 分，实际 %d", room.State.TurnScore)
	}

	// 锁定后可以继续投掷，且新一次投掷会重置 keptThisRoll
	room.HandleRollAgain(a, models.ClientMessage{PlayerID: "a"})
	if room.State.Phase != models.PhaseRolling {
		t.Fatalf("锁定后应进入 rolling，实际 %s", room.State.Phase)
	}
	if room.State.KeptThisRoll {
		t.Fatal("新一次投掷必须重置 keptThisRoll")
	}
}

func TestSinglePlayerReadyDoesNotStart(t *testing.T) {
	room := NewRoom(testTableConfig(), nil)
	client := newFakePeer()

	room.HandleJoin(client, models.ClientMessage{PlayerID: "a", Name: "Alice"})
	room.HandleReady(client, models.ClientMessage{PlayerID: "a"})

	if room.State.Phase != models.PhaseWaiting {
		t.Fatalf("单玩家准备后仍应等待，实际: %s", room.State.Phase)
	}
	if len(room.State.ReadyPlayers) != 1 || room.State.ReadyPlayers[0] != "a" {
		t.Fatalf("单玩家准备状态未记录: %v", room.State.ReadyPlayers)
	}
}

func TestTableRejectsThirdPlayer(t *testing.T) {
	room := NewRoom(testTableConfig(), nil)
	a := newFakePeer()
	b := newFakePeer()
	c := newFakePeer()

	room.HandleJoin(a, models.ClientMessage{PlayerID: "a", Name: "Alice"})
	room.HandleJoin(b, models.ClientMessage{PlayerID: "b", Name: "Bob"})
	room.HandleJoin(c, models.ClientMessage{PlayerID: "c", Name: "Cara"})

	if msg := c.next(t); msg.Type != "ERROR" {
		t.Fatalf("第三人应被拒绝，实际: %+v", msg)
	}
}

func TestJoinRejectedDuringActiveGame(t *testing.T) {
	room := NewRoom(testTableConfig(), nil)
	a := newFakePeer()
	b := newFakePeer()
	c := newFakePeer()

	room.HandleJoin(a, models.ClientMessage{PlayerID: "a", Name: "Alice"})
	room.HandleJoin(b, models.ClientMessage{PlayerID: "b", Name: "Bob"})
	room.startGame()
	room.HandleJoin(c, models.ClientMessage{PlayerID: "c", Name: "Cara"})

	if msg := c.next(t); msg.Type != "ERROR" {
		t.Fatalf("对局中加入应被拒绝，实际: %+v", msg)
	}
}

func TestBothPlayersReadyStartGame(t *testing.T) {
	room := NewRoom(testTableConfig(), nil)
	a := newFakePeer()
	b := newFakePeer()

	room.HandleJoin(a, models.ClientMessage{PlayerID: "a", Name: "Alice"})
	room.HandleJoin(b, models.ClientMessage{PlayerID: "b", Name: "Bob"})
	room.HandleReady(a, models.ClientMessage{PlayerID: "a"})
	if room.State.Phase != models.PhaseWaiting {
		t.Fatalf("first ready must keep waiting, got %s", room.State.Phase)
	}
	room.HandleReady(b, models.ClientMessage{PlayerID: "b"})
	if room.State.Phase != models.PhaseRolling {
		t.Fatalf("both ready must start game, got %s", room.State.Phase)
	}
	if len(room.State.ReadyPlayers) != 0 {
		t.Fatalf("readyPlayers must clear on start: %v", room.State.ReadyPlayers)
	}
}

func TestDuplicatePlayerIDRejected(t *testing.T) {
	room := NewRoom(testTableConfig(), nil)
	original := newFakePeer()
	duplicate := newFakePeer()
	room.Clients[original] = true
	room.Clients[duplicate] = true

	room.HandleJoin(original, models.ClientMessage{PlayerID: "a", Name: "Alice"})
	room.HandleJoin(duplicate, models.ClientMessage{PlayerID: "a", Name: "Alice"})

	if msg := duplicate.next(t); msg.Type != "ERROR" {
		t.Fatalf("重复玩家 ID 应被拒绝，实际: %+v", msg)
	}
}

func TestRematchRequiresBothPlayers(t *testing.T) {
	room := NewRoom(testTableConfig(), nil)
	a := newFakePeer()
	a.id = "a"
	b := newFakePeer()
	b.id = "b"
	pa := &models.Player{ID: "a", Name: "Alice", Score: 3000}
	pb := &models.Player{ID: "b", Name: "Bob", Score: 1750}
	room.Clients[a] = true
	room.Clients[b] = true
	room.Players["a"] = pa
	room.Players["b"] = pb
	room.Order = []string{"a", "b"}
	room.State.Phase = models.PhaseGameOver
	room.State.Players = []*models.Player{pa, pb}
	room.State.Winner = pa.Name

	room.HandleRematchReady(a, models.ClientMessage{PlayerID: "a"})
	if room.State.Phase != models.PhaseGameOver {
		t.Fatalf("only one player ready must keep gameOver, got %s", room.State.Phase)
	}
	if len(room.State.RematchReady) != 1 || room.State.RematchReady[0] != "a" {
		t.Fatalf("unexpected rematchReady: %v", room.State.RematchReady)
	}

	room.HandleRematchReady(b, models.ClientMessage{PlayerID: "b"})
	// 双方都点「继续游戏」后应直接开新局（等价于两人再次准备）
	if room.State.Phase != models.PhaseRolling {
		t.Fatalf("both players ready must start a new game, got %s", room.State.Phase)
	}
	if len(room.State.RematchReady) != 0 {
		t.Fatalf("rematchReady must be cleared, got %v", room.State.RematchReady)
	}
	if len(room.State.ReadyPlayers) != 0 {
		t.Fatalf("readyPlayers must be cleared, got %v", room.State.ReadyPlayers)
	}
	if pa.Score != 0 || pb.Score != 0 {
		t.Fatalf("scores must reset, got %d and %d", pa.Score, pb.Score)
	}
	if room.State.RollDice != 6 {
		t.Fatalf("new game must request a 6 dice roll, got %d", room.State.RollDice)
	}
}

func TestWaitingRoomLeaveRequiresTenSeconds(t *testing.T) {
	room := NewRoom(testTableConfig(), nil)
	client := newFakePeer()
	room.Clients[client] = true
	room.HandleJoin(client, models.ClientMessage{PlayerID: "a", Name: "Alice"})
	room.Players["a"].JoinedAt = nowMs()

	room.HandleLeave(client, models.ClientMessage{PlayerID: "a"})
	if len(room.Players) != 1 {
		t.Fatal("player must not leave before ten seconds")
	}

	room.Players["a"].JoinedAt = nowMs() - 10001
	room.HandleLeave(client, models.ClientMessage{PlayerID: "a"})
	if len(room.Players) != 0 {
		t.Fatal("player should leave after ten seconds")
	}
}

// 先手应随机：多局开局后两个座位都会轮到先手，且始终落在合法下标内
func TestStartGameRandomizesFirstPlayer(t *testing.T) {
	seen := map[int]bool{}
	for i := 0; i < 40; i++ {
		room := NewRoom(testTableConfig(), nil)
		pa := &models.Player{ID: "a", Name: "Alice"}
		pb := &models.Player{ID: "b", Name: "Bob"}
		room.Players["a"] = pa
		room.Players["b"] = pb
		room.Order = []string{"a", "b"}
		room.rebuildPlayerList()

		room.startGame()
		if room.State.CurrentTurn < 0 || room.State.CurrentTurn >= len(room.State.Players) {
			t.Fatalf("先手下标越界: %d", room.State.CurrentTurn)
		}
		seen[room.State.CurrentTurn] = true
	}
	if len(seen) != 2 {
		t.Fatalf("40 局内应该两个座位都出现过先手，实际 %v", seen)
	}
}
