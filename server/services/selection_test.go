package services

import (
	"testing"

	"farkle-server/models"
)

// scoringRoom 搭一个「a 正在选骰」的房间，骰面固定为 1..6
func scoringRoom() (*Room, *fakePeer, *fakePeer) {
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
	room.State.RollDice = 6
	room.State.RollID = 7
	return room, a, b
}

func TestSelectBroadcastsToWholeTable(t *testing.T) {
	room, a, b := scoringRoom()

	// a 选了下标 0 和 2
	room.HandleSelect(a, models.ClientMessage{PlayerID: "a", Select: []int{0, 2}})
	msg := readRoomBroadcast(t, room)
	if msg.Type != "SELECTION" {
		t.Fatalf("应广播 SELECTION，实际 %s", msg.Type)
	}
	if len(msg.Selection) != 2 || msg.Selection[0] != 0 || msg.Selection[1] != 2 {
		t.Fatalf("选择内容不正确: %v", msg.Selection)
	}
	if msg.RollID != room.State.RollID {
		t.Fatalf("SELECTION 应带当前 rollId，实际 %d", msg.RollID)
	}
	if len(room.State.Selection) != 2 {
		t.Fatalf("服务端应记录选择: %v", room.State.Selection)
	}

	// 对手不能改选择（静默忽略，不产生广播）
	room.HandleSelect(b, models.ClientMessage{PlayerID: "b", Select: []int{5}})
	select {
	case raw := <-room.Broadcast:
		t.Fatalf("非当前玩家不应改变选择: %s", raw)
	default:
	}
	if len(room.State.Selection) != 2 {
		t.Fatalf("选择不应被对手覆盖: %v", room.State.Selection)
	}

	// 越界与重复下标会被过滤，清空选择也要广播出去
	room.HandleSelect(a, models.ClientMessage{PlayerID: "a", Select: []int{9, 1, 1}})
	msg = readRoomBroadcast(t, room)
	if len(msg.Selection) != 1 || msg.Selection[0] != 1 {
		t.Fatalf("越界/重复下标应被过滤: %v", msg.Selection)
	}
	room.HandleSelect(a, models.ClientMessage{PlayerID: "a", Select: nil})
	msg = readRoomBroadcast(t, room)
	if len(msg.Selection) != 0 {
		t.Fatalf("清空选择应广播空列表: %v", msg.Selection)
	}
}

func TestKeepAndRollLocksThenRolls(t *testing.T) {
	room, a, _ := scoringRoom()

	room.HandleKeepAndRoll(a, models.ClientMessage{PlayerID: "a", Keep: []int{1}})

	if room.State.TurnScore != 100 {
		t.Fatalf("锁定单颗 1 应得 100 分，实际 %d", room.State.TurnScore)
	}
	if len(room.State.LockedDice) != 1 || room.State.LockedDice[0] != 1 {
		t.Fatalf("应记录已锁定的骰子: %v", room.State.LockedDice)
	}
	// 投掷请求刚发出，点数要等投掷者上报，所以此刻散骰池是空的
	if len(room.State.DiceValues) != 0 {
		t.Fatalf("新一投尚未上报，散骰池应为空，实际 %v", room.State.DiceValues)
	}
	if room.State.Phase != models.PhaseRolling {
		t.Fatalf("「选择并投掷」应直接进入 rolling，实际 %s", room.State.Phase)
	}
	if room.State.RollDice != 5 {
		t.Fatalf("下一投应掷 5 颗，实际 %d", room.State.RollDice)
	}
	if len(room.State.Selection) != 0 {
		t.Fatalf("开始新投掷应清空选择: %v", room.State.Selection)
	}

	msg := readRoomBroadcast(t, room)
	if msg.Type != "ROLL_REQUESTED" {
		t.Fatalf("应广播 ROLL_REQUESTED，实际 %s", msg.Type)
	}
	if len(msg.Roll) != 2 || msg.Roll[0] != 5 {
		t.Fatalf("ROLL_REQUESTED 应携带 [5, seed]，实际 %v", msg.Roll)
	}
}

func TestKeepAndBankEndsTurn(t *testing.T) {
	room, a, _ := scoringRoom()
	room.State.TurnScore = 400 // 本轮之前已经攒了 400
	room.State.LockedDice = []int{5}

	room.HandleKeepAndBank(a, models.ClientMessage{PlayerID: "a", Keep: []int{1}})

	if got := room.Players["a"].Score; got != 500 {
		t.Fatalf("入库后总分应为 500，实际 %d", got)
	}
	if !room.Players["a"].HasBanked {
		t.Fatal("入库后 hasBanked 应为 true")
	}
	if room.State.TurnScore != 0 {
		t.Fatalf("入库后回合分应清零，实际 %d", room.State.TurnScore)
	}
	if room.State.Phase != models.PhaseTurnEnd {
		t.Fatalf("「选择并结束回合」应进入 turnEnd，实际 %s", room.State.Phase)
	}
	if room.State.LastBanked != 500 {
		t.Fatalf("lastBanked 应为 500，实际 %d", room.State.LastBanked)
	}
}

// 首次入库不足 300 分时，即使点了「选择并结束回合」也应被拒绝，且不会白白扣掉骰子
func TestKeepAndBankRejectsLowFirstBank(t *testing.T) {
	room, a, _ := scoringRoom()
	room.State.DiceValues = []int{5, 5, 5, 2, 3, 4}

	room.HandleKeepAndBank(a, models.ClientMessage{PlayerID: "a", Keep: []int{5}})

	assertClientError(t, a, "首次入库至少需要 300 分")
	if room.State.Phase != models.PhaseScoring {
		t.Fatalf("被拒绝后应仍在 scoring，实际 %s", room.State.Phase)
	}
	if len(room.State.DiceValues) != 6 {
		t.Fatalf("被拒绝时不应扣掉骰子，实际 %v", room.State.DiceValues)
	}
	if room.State.TurnScore != 0 {
		t.Fatalf("被拒绝时回合分不应变化，实际 %d", room.State.TurnScore)
	}

	// 已经首次入库过的玩家不受 300 分限制
	room.Players["a"].HasBanked = true
	room.HandleKeepAndBank(a, models.ClientMessage{PlayerID: "a", Keep: []int{5}})
	if room.State.Phase != models.PhaseTurnEnd {
		t.Fatalf("已入库玩家应能直接结束回合，实际 %s", room.State.Phase)
	}
	if room.Players["a"].Score != 50 {
		t.Fatalf("入库分应为 50，实际 %d", room.Players["a"].Score)
	}
}
