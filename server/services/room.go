package services

import (
	"crypto/rand"
	"encoding/json"
	"log"
	"math/big"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"farkle-server/game"
	"farkle-server/models"
)

const (
	MinPlayersToStart = 2 // 单人不许开局
	RollTimeoutMillis = 12000
	MaxChatLength     = 200 // 单条聊天最大字符数
	// 客户端物理回放的步长：与前端 PHYSICS_DT（1/60 秒）一致
	physicsStep = time.Second / 60
	// 骰子停稳后额外等待的时间，让玩家看清落点再公布结果
	rollSettleGrace = 400 * time.Millisecond
)

type Room struct {
	ID          string
	Name        string
	MaxPlayers  int
	Clients     map[Peer]bool
	Players     map[string]*models.Player // playerID -> player
	Order       []string                  // 稳定的玩家顺序（加入先后），currentTurn 按下标引用
	State       models.GameState
	Register    chan Peer
	Unregister  chan Peer
	Broadcast   chan []byte
	Actions     chan func() // 所有游戏逻辑串行化到此执行，避免竞态
	done        chan struct{}
	gen         int // 每次投掷递增，用于作废过期的定时事件
	playerCount atomic.Int32
	inGame      atomic.Bool
	onStatus    func()
}

func NewRoom(t models.Table, onStatus func()) *Room {
	return &Room{
		ID:         t.ID,
		Name:       t.Name,
		MaxPlayers: t.MaxPlayers,
		Clients:    make(map[Peer]bool),
		Players:    make(map[string]*models.Player),
		Order:      []string{},
		Register:   make(chan Peer),
		Unregister: make(chan Peer),
		Broadcast:  make(chan []byte, 64),
		Actions:    make(chan func(), 256),
		done:       make(chan struct{}),
		onStatus:   onStatus,
		State: models.GameState{
			RoomID:       t.ID,
			Phase:        models.PhaseWaiting,
			DiceValues:   []int{},
			LockedDice:   []int{},
			RematchReady: []string{},
			ReadyPlayers: []string{},
			Players:      []*models.Player{},
		},
	}
}

func (r *Room) IsEmpty() bool { return len(r.Clients) == 0 }

// Done 返回房间关闭信号，供传输层在房间退出时停止排队。
func (r *Room) Done() <-chan struct{} { return r.done }

// Status 返回可供大厅 API 安全读取的桌子状态
func (r *Room) Status() (int, bool) {
	return int(r.playerCount.Load()), r.inGame.Load()
}

func (r *Room) updateStatus() {
	playerCount := int32(len(r.Players))
	phase := r.State.Phase
	inGame := phase != "" && phase != models.PhaseWaiting && phase != models.PhaseGameOver
	countChanged := r.playerCount.Swap(playerCount) != playerCount
	gameChanged := r.inGame.Swap(inGame) != inGame
	if (countChanged || gameChanged) && r.onStatus != nil {
		r.onStatus()
	}
}

// After 在 d 毫秒后将 f 排入串行事件队列；房间关闭时丢弃
func (r *Room) After(d time.Duration, f func()) {
	time.AfterFunc(d, func() {
		select {
		case r.Actions <- f:
		case <-r.done:
		}
	})
}

func (r *Room) Run() {
	for {
		select {
		case <-r.done:
			return
		case c := <-r.Register:
			r.Clients[c] = true
		case c := <-r.Unregister:
			r.removePlayer(c)
			// 桌子由配置预热，空桌保留并等待下一位玩家，不能退出 Run 循环
		case f := <-r.Actions:
			f()
		case msg := <-r.Broadcast:
			for c := range r.Clients {
				if c.PlayerID() == "" {
					continue
				}
				if !c.Push(msg) {
					c.Close()
					delete(r.Clients, c)
				}
			}
		}
	}
}

func (r *Room) rebuildPlayerList() {
	players := make([]*models.Player, 0, len(r.Order))
	for _, id := range r.Order {
		if p, ok := r.Players[id]; ok {
			players = append(players, p)
		}
	}
	r.State.Players = players
	if len(players) == 0 {
		r.State.Phase = models.PhaseWaiting
		r.State.CurrentTurn = 0
		r.State.DiceValues = []int{}
		r.State.LockedDice = []int{}
		r.State.TurnScore = 0
		r.State.Selection = []int{}
		r.State.RollDice = 0
		r.State.Trajectory = nil
		r.State.TrajectoryDice = 0
		r.State.StepsPerFrame = 0
		r.State.DicePoses = nil
		r.State.KeptThisRoll = false
	}
	if r.State.CurrentTurn >= len(players) {
		r.State.CurrentTurn = 0
	}
}

func (r *Room) BroadcastState() {
	r.rebuildPlayerList()
	r.updateStatus()
	payload, _ := json.Marshal(models.ServerMessage{Type: "GAME_STATE", State: &r.State, Timestamp: nowMs()})
	r.Broadcast <- payload
}

// broadcastChat 向桌内所有客户端推送一条聊天或系统事件；
// 队列满时直接丢弃，绝不阻塞房间的事件循环。
func (r *Room) broadcastChat(chat models.ChatMessage) {
	if chat.Timestamp == 0 {
		chat.Timestamp = nowMs()
	}
	payload, err := json.Marshal(models.ServerMessage{Type: "CHAT", Chat: &chat, Timestamp: nowMs()})
	if err != nil {
		return
	}
	select {
	case r.Broadcast <- payload:
	default:
	}
}

func (r *Room) resetToWaiting() {
	r.gen++
	r.State.Phase = models.PhaseWaiting
	r.State.CurrentTurn = 0
	r.State.DiceValues = []int{}
	r.State.LockedDice = []int{}
	r.State.TurnScore = 0
	r.State.Selection = []int{}
	r.State.RollDice = 0
	r.State.RollCount = 0
	// 上一局的轨迹必须一起清掉，否则等待中的房间还会把旧骰子摆回桌面
	r.State.Trajectory = nil
	r.State.TrajectoryDice = 0
	r.State.StepsPerFrame = 0
	r.State.DicePoses = nil
	r.State.Winner = ""
	r.State.Farkled = false
	r.State.LastBanked = 0
	r.State.KeptThisRoll = false
	r.State.RematchReady = []string{}
	r.State.ReadyPlayers = []string{}
	for _, p := range r.Players {
		p.Score = 0
		p.HasBanked = false
	}
}

func (r *Room) SendError(c Peer, err string) {
	payload, _ := json.Marshal(models.ServerMessage{Type: "ERROR", Error: err, Timestamp: nowMs()})
	c.Push(payload)
}

func nowMs() int64 { return time.Now().UnixMilli() }

// randomIndex 返回 [0, n) 内的密码学随机数
func randomIndex(n int) int {
	if n <= 1 {
		return 0
	}
	v, err := rand.Int(rand.Reader, big.NewInt(int64(n)))
	if err != nil {
		log.Fatal(err)
	}
	return int(v.Int64())
}

func rollDie() int {
	return randomIndex(6) + 1
}

// ---- 游戏逻辑（服务端权威） ----

func (r *Room) HandleJoin(c Peer, m models.ClientMessage) {
	for existing := range r.Clients {
		if existing != c && existing.PlayerID() == m.PlayerID {
			r.SendError(c, "该玩家已在桌内")
			return
		}
	}
	// 已在桌内的同一玩家重连：不重复加入
	if _, ok := r.Players[m.PlayerID]; ok {
		c.SetPlayerID(m.PlayerID)
		r.BroadcastState()
		return
	}
	if r.State.Phase != models.PhaseWaiting && r.State.Phase != models.PhaseGameOver {
		r.SendError(c, "对局进行中，不能中途加入")
		return
	}
	if len(r.Players) >= r.MaxPlayers {
		r.SendError(c, "这张桌子已满")
		return
	}
	player := &models.Player{ID: m.PlayerID, Name: m.Name, Score: 0, JoinedAt: nowMs()}
	if player.Name == "" {
		player.Name = "traveler"
	}
	r.Players[m.PlayerID] = player
	r.Order = append(r.Order, m.PlayerID)
	c.SetPlayerID(m.PlayerID)
	r.rebuildPlayerList()
	if r.State.Phase == "" || r.State.Phase == models.PhaseWaiting {
		r.State.Phase = models.PhaseWaiting
	}
	r.BroadcastState()
	r.broadcastChat(models.ChatMessage{Kind: "system", Text: player.Name + " 加入了牌桌"})
}

func (r *Room) removePlayer(c Peer) {
	// 即使该 client 已经因发送队列溢出被移出 Clients，也必须按玩家 ID 清理，
	// 否则会留下永远占座的“幽灵玩家”。
	delete(r.Clients, c)
	playerID := c.PlayerID()
	p, ok := r.Players[playerID]
	if !ok {
		return
	}
	delete(r.Players, playerID)
	for i, id := range r.Order {
		if id == playerID {
			r.Order = append(r.Order[:i], r.Order[i+1:]...)
			break
		}
	}
	log.Printf("player %s left room %s", p.Name, r.ID)
	r.State.ReadyPlayers = removePlayerID(r.State.ReadyPlayers, playerID)
	if r.State.Phase != models.PhaseWaiting {
		r.resetToWaiting()
	}
	r.rebuildPlayerList()
	r.BroadcastState()
	r.broadcastChat(models.ChatMessage{Kind: "system", Text: p.Name + " 离开了牌桌"})
}

func (r *Room) HandleLeave(c Peer, m models.ClientMessage) {
	if r.State.Phase != models.PhaseWaiting {
		r.SendError(c, "对局期间不能退出房间")
		return
	}
	p, ok := r.Players[c.PlayerID()]
	if !ok {
		r.SendError(c, "你不在当前牌桌")
		return
	}
	if nowMs()-p.JoinedAt < 10000 {
		r.SendError(c, "进入房间 10 秒后才能退出")
		return
	}

	r.removePlayer(c)
	payload, _ := json.Marshal(models.ServerMessage{Type: "ROOM_LEFT", Timestamp: nowMs()})
	c.Push(payload)
	go func() {
		time.Sleep(100 * time.Millisecond)
		c.Close()
	}()
}

// HandleChat 广播一条桌内聊天；聊天不限制对局阶段，等待和游戏中都能发言。
func (r *Room) HandleChat(c Peer, m models.ClientMessage) {
	// 以连接自身的身份发言，忽略客户端自报的 playerId，避免冒名顶替
	player, ok := r.Players[c.PlayerID()]
	if !ok {
		r.SendError(c, "你不在当前牌桌")
		return
	}
	text := strings.TrimSpace(m.Text)
	if text == "" {
		return
	}
	if runes := []rune(text); len(runes) > MaxChatLength {
		text = string(runes[:MaxChatLength])
	}
	r.broadcastChat(models.ChatMessage{
		Kind:     "chat",
		PlayerID: player.ID,
		Name:     player.Name,
		Text:     text,
	})
}

func (r *Room) startGame() {
	for _, p := range r.Players {
		p.Score = 0
		p.HasBanked = false
	}
	r.State.Phase = models.PhaseRolling
	// 每局（包括「继续游戏」）都随机决定先手，不再固定为先加入的玩家
	r.State.CurrentTurn = randomIndex(len(r.State.Players))
	r.State.TurnScore = 0
	r.State.LockedDice = []int{}
	r.State.Selection = []int{}
	r.State.RollDice = 0
	r.State.RollCount = 0
	r.State.Winner = ""
	r.State.KeptThisRoll = false
	r.State.RematchReady = []string{}
	r.State.ReadyPlayers = []string{}
	r.doRoll()
}

func (r *Room) HandleReady(c Peer, m models.ClientMessage) {
	if r.State.Phase != models.PhaseWaiting {
		r.SendError(c, "当前不能准备")
		return
	}
	if _, ok := r.Players[m.PlayerID]; !ok {
		r.SendError(c, "你不在当前牌桌")
		return
	}
	if !containsPlayerID(r.State.ReadyPlayers, m.PlayerID) {
		r.State.ReadyPlayers = append(r.State.ReadyPlayers, m.PlayerID)
	}
	if len(r.Players) >= MinPlayersToStart && len(r.State.ReadyPlayers) == len(r.Players) {
		r.startGame()
		return
	}
	r.BroadcastState()
}

func containsPlayerID(ids []string, id string) bool {
	for _, existing := range ids {
		if existing == id {
			return true
		}
	}
	return false
}

func removePlayerID(ids []string, id string) []string {
	result := ids[:0]
	for _, existing := range ids {
		if existing != id {
			result = append(result, existing)
		}
	}
	return result
}

// beginRoll 广播"请投掷"（rollId + 骰子数），等待当前玩家客户端物理动画结束后
// 通过 ROLL_RESULT 上报点数；超时未上报则由服务端兜底掷骰，保证游戏永不卡死。
func (r *Room) doRoll() {
	remaining := 6 - len(r.State.LockedDice)
	if remaining <= 0 {
		remaining = 6 // 全锁后再掷 6 颗
		r.State.LockedDice = []int{}
	}
	r.gen++ // 作废此前所有未到期的定时事件
	gen := r.gen

	r.State.DiceValues = []int{}
	r.State.DicePoses = nil
	r.State.Trajectory = nil
	r.State.TrajectoryDice = 0
	r.State.StepsPerFrame = 0
	r.State.Selection = []int{}
	r.State.RollDice = remaining
	r.State.RollCount++
	r.State.Farkled = false
	r.State.LastBanked = 0
	r.State.KeptThisRoll = false
	r.State.Phase = models.PhaseRolling
	r.State.RollID = gen
	r.State.AnimatingTill = nowMs() + RollTimeoutMillis
	r.updateStatus()

	// roll 字段携带 [骰子数量, 随机种子]；种子让全桌客户端跑出一致的物理结果
	seed := int(rollDie()*1000000) + int(nowMs()%1000000)
	payload, _ := json.Marshal(models.ServerMessage{
		Type:      "ROLL_REQUESTED",
		Roll:      []int{remaining, seed},
		State:     &r.State,
		Timestamp: nowMs(),
	})
	r.Broadcast <- payload

	// 兜底：客户端迟迟不上报（掉线/异常）时服务端自己掷
	r.After(RollTimeoutMillis*time.Millisecond, func() {
		if r.gen != gen || r.State.Phase != models.PhaseRolling {
			return
		}
		fallback := make([]int, remaining)
		for i := range fallback {
			fallback[i] = rollDie()
		}
		// 兜底投掷没有轨迹，客户端没有动画可等，立即公布
		r.applyRoll(fallback, 0)
	})
}

// applyRoll 结算一次投掷（来自客户端上报或服务端兜底）。
// 轨迹先广播出去让全桌开始翻滚，阶段保持 rolling；等动画播完才公布点数，
// 所以 farkle 和得分都不会抢在骰子停稳之前出现。
func (r *Room) applyRoll(dice []int, animDelay time.Duration) {
	gen := r.gen
	r.BroadcastState()
	r.After(animDelay, func() {
		if r.gen != gen || r.State.Phase != models.PhaseRolling {
			return
		}
		r.revealRoll(dice)
	})
}

// revealRoll 公布本次投掷的点数：farkle 结束回合，否则进入选骰阶段
func (r *Room) revealRoll(dice []int) {
	r.State.DiceValues = dice
	gen := r.gen
	if game.BestScore(dice) == 0 {
		r.State.TurnScore = 0
		r.State.Farkled = true
		r.State.Phase = models.PhaseTurnEnd
		r.BroadcastState()
		r.After(1500*time.Millisecond, func() {
			if r.gen != gen {
				return
			}
			r.nextTurn()
		})
		return
	}
	r.State.Phase = models.PhaseScoring
	r.BroadcastState()
}

// trajectoryAnimDelay 计算客户端回放这段轨迹需要的时长：
// 播放头从第 0 帧走到末帧，每帧代表 stepsPerFrame 个物理步；再留一小段停顿让骰子看清落点
func trajectoryAnimDelay(frames, stepsPerFrame int) time.Duration {
	if frames < 2 {
		return 0
	}
	play := time.Duration(frames-1) * time.Duration(stepsPerFrame) * physicsStep
	return play + rollSettleGrace
}

// HandleRollResult 接收投掷者客户端上报的物理动画结果
func (r *Room) HandleRollResult(c Peer, m models.ClientMessage) {
	if !r.isCurrentPlayer(m.PlayerID) {
		r.SendError(c, "还没轮到你")
		return
	}
	if r.State.Phase != models.PhaseRolling {
		r.SendError(c, "当前不在投掷阶段")
		return
	}
	expected := 6 - len(r.State.LockedDice)
	if expected <= 0 {
		expected = 6
	}
	if len(m.Roll) != expected {
		r.SendError(c, "骰子数量不正确")
		return
	}
	for _, v := range m.Roll {
		if v < 1 || v > 6 {
			r.SendError(c, "骰子点数非法")
			return
		}
	}
	r.gen++ // 作废兜底定时器
	// 投掷者上报的最终姿态作为全桌权威姿态，观察者据此对齐画面
	if len(m.Poses) == expected {
		r.State.DicePoses = m.Poses
	}
	// 关键帧轨迹：结构校验通过才接受（每帧 = 本次投掷的骰子数 × 7 个数）。
	// 注意步长必须按实际骰子数算：锁骰后只剩 N 颗，帧长度是 N×7 而不是固定的 42。
	stride := expected * models.TrajectoryNumbersPerDie
	var animDelay time.Duration
	if n := len(m.Trajectory); n > 0 && n%stride == 0 && n/stride <= models.MaxTrajectoryFrames &&
		m.StepsPerFrame >= 1 && m.StepsPerFrame <= 60 {
		r.State.Trajectory = m.Trajectory
		r.State.TrajectoryDice = expected
		r.State.StepsPerFrame = m.StepsPerFrame
		animDelay = trajectoryAnimDelay(n/stride, m.StepsPerFrame)
	}
	r.applyRoll(m.Roll, animDelay)
}

// validateKeep 校验一次锁定，返回得分、扣除所选后的散骰池与错误文案（空串表示合法）。
// 只读校验，不改动房间状态，便于「选择并结束回合」先判断能不能入库。
func (r *Room) validateKeep(m models.ClientMessage) (int, []int, string) {
	if r.State.Phase != models.PhaseScoring {
		return 0, nil, "现在不能锁定骰子"
	}
	if !r.isCurrentPlayer(m.PlayerID) {
		return 0, nil, "还没轮到你"
	}
	if len(m.Keep) == 0 {
		return 0, nil, "请至少选择一颗得分骰"
	}
	// 验证所选骰子确实存在于散骰中
	pool := make([]int, len(r.State.DiceValues))
	copy(pool, r.State.DiceValues)
	for _, k := range m.Keep {
		found := false
		for i, d := range pool {
			if d == k {
				pool = append(pool[:i], pool[i+1:]...)
				found = true
				break
			}
		}
		if !found {
			return 0, nil, "所选骰子不存在"
		}
	}
	score, ok := game.ValidSelection(m.Keep)
	if !ok {
		return 0, nil, "所选骰子组合不得分"
	}
	return score, pool, ""
}

// applyKeep 应用一次锁定（扣散骰池、累加回合分）。校验失败时已回错误，返回 false。
func (r *Room) applyKeep(c Peer, m models.ClientMessage) bool {
	score, pool, errMsg := r.validateKeep(m)
	if errMsg != "" {
		r.SendError(c, errMsg)
		return false
	}
	r.State.TurnScore += score
	r.State.LockedDice = append(r.State.LockedDice, m.Keep...)
	r.State.DiceValues = pool
	r.State.KeptThisRoll = true
	r.State.Selection = []int{}
	r.State.Phase = models.PhaseScoring
	return true
}

// HandleKeep 只锁定骰子（保留兼容：新前端用「选择并投掷/结束回合」两个动作）
func (r *Room) HandleKeep(c Peer, m models.ClientMessage) {
	if !r.applyKeep(c, m) {
		return
	}
	r.BroadcastState()
}

// HandleSelect 广播当前玩家选中的骰子下标，让对手也能看到他在选什么。
// 这里只做下标合法性校验，真正的得分校验在 applyKeep 里。
func (r *Room) HandleSelect(c Peer, m models.ClientMessage) {
	// 不是当前玩家/不在可选阶段时静默忽略，避免误触刷出一堆错误提示
	if !r.isCurrentPlayer(m.PlayerID) {
		return
	}
	if r.State.Phase != models.PhaseScoring && r.State.Phase != models.PhaseRolling {
		return
	}
	rollDice := r.State.RollDice
	if rollDice <= 0 {
		rollDice = len(r.State.DiceValues)
	}
	seen := make(map[int]bool, len(m.Select))
	selection := make([]int, 0, len(m.Select))
	for _, idx := range m.Select {
		if idx < 0 || idx >= rollDice || seen[idx] {
			continue
		}
		seen[idx] = true
		selection = append(selection, idx)
	}
	sort.Ints(selection)
	r.State.Selection = selection

	payload, err := json.Marshal(models.ServerMessage{
		Type:      "SELECTION",
		Selection: selection,
		RollID:    r.State.RollID,
		Timestamp: nowMs(),
	})
	if err != nil {
		return
	}
	select {
	case r.Broadcast <- payload:
	default:
	}
}

func (r *Room) HandleRollAgain(c Peer, m models.ClientMessage) {
	if r.State.Phase != models.PhaseScoring {
		r.SendError(c, "现在不能掷骰")
		return
	}
	if !r.isCurrentPlayer(m.PlayerID) {
		r.SendError(c, "还没轮到你")
		return
	}
	// 必须先在本次投掷中锁定至少一颗得分骰，才能继续投掷；
	// 否则玩家可以反复重掷直到掷出想要的点数。
	if !r.State.KeptThisRoll {
		r.SendError(c, "请先锁定至少一颗得分骰子，再继续投掷")
		return
	}
	r.doRoll()
}

// applyBank 把当前回合分入库并推进回合
func (r *Room) applyBank(c Peer, m models.ClientMessage) {
	p := r.Players[m.PlayerID]
	if !p.HasBanked && r.State.TurnScore < game.MinBankScore {
		r.SendError(c, "首次入库至少需要 300 分")
		return
	}
	p.Score += r.State.TurnScore
	p.HasBanked = true
	r.State.LastBanked = r.State.TurnScore
	r.State.TurnScore = 0
	r.State.Selection = []int{}
	r.State.Phase = models.PhaseTurnEnd
	if p.Score >= game.TargetScore {
		r.State.Phase = models.PhaseGameOver
		r.State.Winner = p.Name
		r.State.RematchReady = []string{}
		r.BroadcastState()
		return
	}
	gen := r.gen
	r.BroadcastState()
	r.After(1200*time.Millisecond, func() {
		if r.gen != gen {
			return
		}
		r.nextTurn()
	})
}

// HandleKeepAndRoll 「选择并投掷」：锁定所选骰子后立刻投掷剩下的骰子
func (r *Room) HandleKeepAndRoll(c Peer, m models.ClientMessage) {
	if !r.applyKeep(c, m) {
		return
	}
	r.doRoll()
}

// HandleKeepAndBank 「选择并结束回合」：锁定所选骰子后把回合分入库
func (r *Room) HandleKeepAndBank(c Peer, m models.ClientMessage) {
	// 先做只读校验：入库不满足条件时不能把骰子白白扣掉
	score, _, errMsg := r.validateKeep(m)
	if errMsg != "" {
		r.SendError(c, errMsg)
		return
	}
	p := r.Players[m.PlayerID]
	if !p.HasBanked && r.State.TurnScore+score < game.MinBankScore {
		r.SendError(c, "首次入库至少需要 300 分")
		return
	}
	if !r.applyKeep(c, m) {
		return
	}
	r.applyBank(c, m)
}

func (r *Room) HandleBank(c Peer, m models.ClientMessage) {
	if r.State.Phase != models.PhaseScoring {
		r.SendError(c, "现在不能入库")
		return
	}
	if !r.isCurrentPlayer(m.PlayerID) {
		r.SendError(c, "还没轮到你")
		return
	}
	// 与继续投掷一致：每次投掷都必须先锁定得分骰，再决定继续或入库。
	if !r.State.KeptThisRoll {
		r.SendError(c, "请先锁定至少一颗得分骰子，再入库")
		return
	}
	r.applyBank(c, m)
}

func (r *Room) HandleRematchReady(c Peer, m models.ClientMessage) {
	if r.State.Phase != models.PhaseGameOver {
		r.SendError(c, "对局尚未结束")
		return
	}
	if _, ok := r.Players[m.PlayerID]; !ok {
		r.SendError(c, "你不在当前牌桌")
		return
	}
	for _, id := range r.State.RematchReady {
		if id == m.PlayerID {
			return
		}
	}
	r.State.RematchReady = append(r.State.RematchReady, m.PlayerID)
	if len(r.Players) >= MinPlayersToStart && len(r.State.RematchReady) == len(r.Players) {
		// 「继续游戏」等价于双方再次准备：两人都点过之后直接开新局，
		// 不再回到等待房间让两人重新点一次准备。
		r.resetToWaiting()
		r.startGame()
		return
	}
	r.BroadcastState()
}

func (r *Room) nextTurn() {
	players := len(r.State.Players)
	if players == 0 {
		return
	}
	r.State.CurrentTurn = (r.State.CurrentTurn + 1) % players
	r.State.TurnScore = 0
	r.State.LockedDice = []int{}
	r.State.DiceValues = []int{}
	r.State.RollCount = 0
	r.State.Phase = models.PhaseRolling
	r.doRoll()
}

func (r *Room) isCurrentPlayer(playerID string) bool {
	players := r.State.Players
	if len(players) == 0 || r.State.CurrentTurn >= len(players) {
		return false
	}
	return players[r.State.CurrentTurn].ID == playerID
}
