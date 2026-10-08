package models

// Phase 标识牌桌的对局阶段，字符串值直接作为前端协议的一部分。
type Phase string

const (
	PhaseWaiting  Phase = "waiting"
	PhaseRolling  Phase = "rolling"
	PhaseScoring  Phase = "scoring"
	PhaseTurnEnd  Phase = "turnEnd"
	PhaseGameOver Phase = "gameOver"
)

type Player struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Score     int    `json:"score"`
	HasBanked bool   `json:"hasBanked"` // 本局是否已首次入库
	JoinedAt  int64  `json:"-"`
}

// DiePose 一颗骰子停稳后的姿态（位置 + 四元数）
type DiePose struct {
	P [3]float64 `json:"p"`
	Q [4]float64 `json:"q"`
}

type GameState struct {
	RoomID         string    `json:"roomId"`
	Phase          Phase     `json:"phase"`
	Players        []*Player `json:"players"`
	CurrentTurn    int       `json:"currentTurn"` // 玩家索引
	DiceValues     []int     `json:"diceValues"`  // 当前散骰
	LockedDice     []int     `json:"lockedDice"`  // 已锁定骰子（值）
	TurnScore      int       `json:"turnScore"`   // 本回合已累积（未入库）
	RollDice       int       `json:"rollDice"`    // 本次投掷的骰子数量（选择索引的上界）
	Selection      []int     `json:"selection"`   // 当前玩家选中的骰子下标（托盘下标，全桌可见）
	RollCount      int       `json:"rollCount"`
	Winner         string    `json:"winner,omitempty"`
	Farkled        bool      `json:"farkled"`                  // 本回合是否爆牌
	LastBanked     int       `json:"lastBanked"`               // 上一次入库得分
	RollID         int       `json:"rollId"`                   // 每次投掷递增
	AnimatingTill  int64     `json:"animatingTill"`            // 投掷超时兜底截止 unix ms
	RematchReady   []string  `json:"rematchReady"`             // 已选择继续游戏的玩家
	ReadyPlayers   []string  `json:"readyPlayers"`             // 已点击准备的玩家
	KeptThisRoll   bool      `json:"keptThisRoll"`             // 本次投掷后是否已锁定至少一颗骰子
	DicePoses      []DiePose `json:"dicePoses,omitempty"`      // 本轮权威姿态（投掷者上报，全桌对齐用）
	Trajectory     []float64 `json:"trajectory,omitempty"`     // 本轮关键帧轨迹（扁平：每帧 N×7 个数）
	TrajectoryDice int       `json:"trajectoryDice,omitempty"` // 轨迹里每帧包含的骰子数
	StepsPerFrame  int       `json:"stepsPerFrame,omitempty"`  // 每个关键帧间隔的物理步数
}

// ChatMessage 一条桌内消息：玩家发言或系统事件（加入/离开）
type ChatMessage struct {
	Kind      string `json:"kind"` // "chat" 玩家发言 / "system" 系统事件
	PlayerID  string `json:"playerId,omitempty"`
	Name      string `json:"name,omitempty"`
	Text      string `json:"text"`
	Timestamp int64  `json:"timestamp"`
}
