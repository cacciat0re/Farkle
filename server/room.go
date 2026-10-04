package main

import (
	"crypto/rand"
	"encoding/json"
	"log"
	"math/big"
	"sync/atomic"
	"time"
)

const (
	StateWaiting  = "waiting"
	StateRolling  = "rolling"
	StateScoring  = "scoring"
	StateTurnEnd  = "turnEnd"
	StateGameOver = "gameOver"

	TargetScore       = 3000
	MinBankScore      = 300 // 首次入库最低分（经典规则）
	MinPlayersToStart = 2   // 单人不许开局
	RollTimeoutMillis = 12000
)

type Player struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Score     int    `json:"score"`
	HasBanked bool   `json:"hasBanked"` // 本局是否已首次入库
	JoinedAt  int64  `json:"-"`
}

type GameState struct {
	RoomID        string    `json:"roomId"`
	Phase         string    `json:"phase"`
	Players       []*Player `json:"players"`
	CurrentTurn   int       `json:"currentTurn"` // 玩家索引
	DiceValues    []int     `json:"diceValues"`  // 当前散骰
	LockedDice    []int     `json:"lockedDice"`  // 已锁定骰子（值）
	TurnScore     int       `json:"turnScore"`   // 本回合已累积（未入库）
	RollCount     int       `json:"rollCount"`
	Winner        string    `json:"winner,omitempty"`
	Farkled       bool      `json:"farkled"`       // 本回合是否爆牌
	LastBanked    int       `json:"lastBanked"`    // 上一次入库得分
	RollID        int       `json:"rollId"`        // 每次投掷递增
	AnimatingTill int64     `json:"animatingTill"` // 投掷超时兜底截止 unix ms
	RematchReady  []string  `json:"rematchReady"`  // 已选择继续游戏的玩家
	ReadyPlayers  []string  `json:"readyPlayers"`  // 已点击准备的玩家
}

type ClientMessage struct {
	Type     string `json:"type"`
	PlayerID string `json:"playerId"`
	Name     string `json:"name,omitempty"`
	RoomID   string `json:"roomId"`
	Keep     []int  `json:"keep,omitempty"` // 要锁定的骰子值
	Roll     []int  `json:"roll,omitempty"` // 客户端物理动画的最终点数
}

type ServerMessage struct {
	Type      string     `json:"type"`
	State     *GameState `json:"state,omitempty"`
	Roll      []int      `json:"roll,omitempty"`
	Error     string     `json:"error,omitempty"`
	PlayerID  string     `json:"playerId,omitempty"`
	RoomID    string     `json:"roomId,omitempty"`
	Timestamp int64      `json:"timestamp"`
}

type Room struct {
	ID          string
	Name        string
	MaxPlayers  int
	Clients     map[*Client]bool
	Players     map[string]*Player // playerID -> player
	Order       []string           // 稳定的玩家顺序（加入先后），currentTurn 按下标引用
	State       GameState
	Register    chan *Client
	Unregister  chan *Client
	Broadcast   chan []byte
	Actions     chan func() // 所有游戏逻辑串行化到此执行，避免竞态
	done        chan struct{}
	gen         int // 每次投掷递增，用于作废过期的定时事件
	playerCount atomic.Int32
	inGame      atomic.Bool
	onStatus    func()
}

func NewRoom(t TableConfig, onStatus func()) *Room {
	return &Room{
		ID:         t.ID,
		Name:       t.Name,
		MaxPlayers: t.MaxPlayers,
		Clients:    make(map[*Client]bool),
		Players:    make(map[string]*Player),
		Order:      []string{},
		Register:   make(chan *Client),
		Unregister: make(chan *Client),
		Broadcast:  make(chan []byte, 64),
		Actions:    make(chan func(), 256),
		done:       make(chan struct{}),
		onStatus:   onStatus,
		State: GameState{
			RoomID:       t.ID,
			Phase:        StateWaiting,
			DiceValues:   []int{},
			LockedDice:   []int{},
			RematchReady: []string{},
			ReadyPlayers: []string{},
			Players:      []*Player{},
		},
	}
}

func (r *Room) IsEmpty() bool { return len(r.Clients) == 0 }

// Status 返回可供大厅 API 安全读取的桌子状态
func (r *Room) Status() (int, bool) {
	return int(r.playerCount.Load()), r.inGame.Load()
}

func (r *Room) updateStatus() {
	playerCount := int32(len(r.Players))
	phase := r.State.Phase
	inGame := phase != "" && phase != StateWaiting && phase != StateGameOver
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
				if c.PlayerID == "" {
					continue
				}
				select {
				case c.Send <- msg:
				default:
					close(c.Send)
					delete(r.Clients, c)
				}
			}
		}
	}
}

func (r *Room) rebuildPlayerList() {
	players := make([]*Player, 0, len(r.Order))
	for _, id := range r.Order {
		if p, ok := r.Players[id]; ok {
			players = append(players, p)
		}
	}
	r.State.Players = players
	if len(players) == 0 {
		r.State.Phase = StateWaiting
		r.State.CurrentTurn = 0
		r.State.DiceValues = []int{}
		r.State.LockedDice = []int{}
		r.State.TurnScore = 0
	}
	if r.State.CurrentTurn >= len(players) {
		r.State.CurrentTurn = 0
	}
}

func (r *Room) BroadcastState() {
	r.rebuildPlayerList()
	r.updateStatus()
	payload, _ := json.Marshal(ServerMessage{Type: "GAME_STATE", State: &r.State, Timestamp: nowMs()})
	r.Broadcast <- payload
}

func (r *Room) resetToWaiting() {
	r.gen++
	r.State.Phase = StateWaiting
	r.State.CurrentTurn = 0
	r.State.DiceValues = []int{}
	r.State.LockedDice = []int{}
	r.State.TurnScore = 0
	r.State.RollCount = 0
	r.State.Winner = ""
	r.State.Farkled = false
	r.State.LastBanked = 0
	r.State.RematchReady = []string{}
	r.State.ReadyPlayers = []string{}
	for _, p := range r.Players {
		p.Score = 0
		p.HasBanked = false
	}
}

func (r *Room) SendError(c *Client, err string) {
	payload, _ := json.Marshal(ServerMessage{Type: "ERROR", Error: err, Timestamp: nowMs()})
	select {
	case c.Send <- payload:
	default:
	}
}

func nowMs() int64 { return time.Now().UnixMilli() }

func rollDie() int {
	n, err := rand.Int(rand.Reader, big.NewInt(6))
	if err != nil {
		log.Fatal(err)
	}
	return int(n.Int64()) + 1
}

// ---- 游戏逻辑（服务端权威） ----

func (r *Room) HandleJoin(c *Client, m ClientMessage) {
	for existing := range r.Clients {
		if existing != c && existing.PlayerID == m.PlayerID {
			r.SendError(c, "该玩家已在桌内")
			return
		}
	}
	// 已在桌内的同一玩家重连：不重复加入
	if _, ok := r.Players[m.PlayerID]; ok {
		c.PlayerID = m.PlayerID
		r.BroadcastState()
		return
	}
	if r.State.Phase != StateWaiting && r.State.Phase != StateGameOver {
		r.SendError(c, "对局进行中，不能中途加入")
		return
	}
	if len(r.Players) >= r.MaxPlayers {
		r.SendError(c, "这张桌子已满")
		return
	}
	player := &Player{ID: m.PlayerID, Name: m.Name, Score: 0, JoinedAt: nowMs()}
	if player.Name == "" {
		player.Name = "traveler"
	}
	r.Players[m.PlayerID] = player
	r.Order = append(r.Order, m.PlayerID)
	c.PlayerID = m.PlayerID
	r.rebuildPlayerList()
	if r.State.Phase == "" || r.State.Phase == StateWaiting {
		r.State.Phase = StateWaiting
	}
	r.BroadcastState()
}

func (r *Room) removePlayer(c *Client) {
	if _, exists := r.Clients[c]; !exists {
		return
	}
	delete(r.Clients, c)
	p, ok := r.Players[c.PlayerID]
	if !ok {
		return
	}
	delete(r.Players, c.PlayerID)
	for i, id := range r.Order {
		if id == c.PlayerID {
			r.Order = append(r.Order[:i], r.Order[i+1:]...)
			break
		}
	}
	log.Printf("player %s left room %s", p.Name, r.ID)
	r.State.ReadyPlayers = removePlayerID(r.State.ReadyPlayers, c.PlayerID)
	if r.State.Phase != StateWaiting {
		r.resetToWaiting()
	}
	r.rebuildPlayerList()
	r.BroadcastState()
}

func (r *Room) HandleLeave(c *Client, m ClientMessage) {
	if r.State.Phase != StateWaiting {
		r.SendError(c, "对局期间不能退出房间")
		return
	}
	p, ok := r.Players[c.PlayerID]
	if !ok {
		r.SendError(c, "你不在当前牌桌")
		return
	}
	if nowMs()-p.JoinedAt < 10000 {
		r.SendError(c, "进入房间 10 秒后才能退出")
		return
	}

	r.removePlayer(c)
	payload, _ := json.Marshal(ServerMessage{Type: "ROOM_LEFT", Timestamp: nowMs()})
	select {
	case c.Send <- payload:
	default:
	}
	if c.Conn != nil {
		go func() {
			time.Sleep(100 * time.Millisecond)
			c.Conn.Close()
		}()
	}
}

func (r *Room) startGame() {
	for _, p := range r.Players {
		p.Score = 0
		p.HasBanked = false
	}
	r.State.Phase = StateRolling
	r.State.CurrentTurn = 0
	r.State.TurnScore = 0
	r.State.LockedDice = []int{}
	r.State.RollCount = 0
	r.State.Winner = ""
	r.State.RematchReady = []string{}
	r.State.ReadyPlayers = []string{}
	r.doRoll()
}

func (r *Room) HandleReady(c *Client, m ClientMessage) {
	if r.State.Phase != StateWaiting {
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
	r.State.RollCount++
	r.State.Farkled = false
	r.State.LastBanked = 0
	r.State.Phase = StateRolling
	r.State.RollID = gen
	r.State.AnimatingTill = nowMs() + RollTimeoutMillis
	r.updateStatus()

	// roll 字段携带 [骰子数量, 随机种子]；种子让全桌客户端跑出一致的物理结果
	seed := int(rollDie()*1000000) + int(nowMs()%1000000)
	payload, _ := json.Marshal(ServerMessage{
		Type:      "ROLL_REQUESTED",
		Roll:      []int{remaining, seed},
		State:     &r.State,
		Timestamp: nowMs(),
	})
	r.Broadcast <- payload

	// 兜底：客户端迟迟不上报（掉线/异常）时服务端自己掷
	r.After(RollTimeoutMillis*time.Millisecond, func() {
		if r.gen != gen || r.State.Phase != StateRolling {
			return
		}
		fallback := make([]int, remaining)
		for i := range fallback {
			fallback[i] = rollDie()
		}
		r.applyRoll(fallback)
	})
}

// applyRoll 结算一次投掷（来自客户端上报或服务端兜底）
func (r *Room) applyRoll(dice []int) {
	r.State.DiceValues = dice
	isFarkle := bestScore(dice) == 0
	gen := r.gen

	if isFarkle {
		r.State.TurnScore = 0
		r.State.Farkled = true
		r.State.Phase = StateTurnEnd
		r.BroadcastState()
		r.After(1500*time.Millisecond, func() {
			if r.gen != gen {
				return
			}
			r.nextTurn()
		})
	} else {
		r.State.Phase = StateScoring
		r.BroadcastState()
	}
}

// HandleRollResult 接收投掷者客户端上报的物理动画结果
func (r *Room) HandleRollResult(c *Client, m ClientMessage) {
	if !r.isCurrentPlayer(m.PlayerID) {
		r.SendError(c, "还没轮到你")
		return
	}
	if r.State.Phase != StateRolling {
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
	r.applyRoll(m.Roll)
}

func (r *Room) HandleKeep(c *Client, m ClientMessage) {
	if r.State.Phase != StateScoring {
		r.SendError(c, "现在不能锁定骰子")
		return
	}
	if !r.isCurrentPlayer(m.PlayerID) {
		r.SendError(c, "还没轮到你")
		return
	}
	if len(m.Keep) == 0 {
		r.SendError(c, "请至少选择一颗得分骰")
		return
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
			r.SendError(c, "所选骰子不存在")
			return
		}
	}
	score, ok := validSelection(m.Keep)
	if !ok {
		r.SendError(c, "所选骰子组合不得分")
		return
	}
	r.State.TurnScore += score
	r.State.LockedDice = append(r.State.LockedDice, m.Keep...)
	r.State.DiceValues = pool
	r.State.Phase = StateScoring
	r.BroadcastState()
}

func (r *Room) HandleRollAgain(c *Client, m ClientMessage) {
	if r.State.Phase != StateScoring {
		r.SendError(c, "现在不能掷骰")
		return
	}
	if !r.isCurrentPlayer(m.PlayerID) {
		r.SendError(c, "还没轮到你")
		return
	}
	r.doRoll()
}

func (r *Room) HandleBank(c *Client, m ClientMessage) {
	if r.State.Phase != StateScoring {
		r.SendError(c, "现在不能入库")
		return
	}
	if !r.isCurrentPlayer(m.PlayerID) {
		r.SendError(c, "还没轮到你")
		return
	}
	p := r.Players[m.PlayerID]
	if !p.HasBanked && r.State.TurnScore < MinBankScore {
		r.SendError(c, "首次入库至少需要 300 分")
		return
	}
	p.Score += r.State.TurnScore
	p.HasBanked = true
	r.State.LastBanked = r.State.TurnScore
	r.State.TurnScore = 0
	r.State.Phase = StateTurnEnd
	if p.Score >= TargetScore {
		r.State.Phase = StateGameOver
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

func (r *Room) HandleRematchReady(c *Client, m ClientMessage) {
	if r.State.Phase != StateGameOver {
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
		r.resetToWaiting()
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
	r.State.Phase = StateRolling
	r.doRoll()
}

func (r *Room) isCurrentPlayer(playerID string) bool {
	players := r.State.Players
	if len(players) == 0 || r.State.CurrentTurn >= len(players) {
		return false
	}
	return players[r.State.CurrentTurn].ID == playerID
}
