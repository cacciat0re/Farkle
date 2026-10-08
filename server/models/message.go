package models

const (
	// TrajectoryNumbersPerDie 每颗骰子每帧 7 个数（x,y,z,qx,qy,qz,qw）
	TrajectoryNumbersPerDie = 7
	// MaxTrajectoryFrames 上限保护：400 帧 ≈ 20 秒，远大于正常投掷
	MaxTrajectoryFrames = 400
)

type ClientMessage struct {
	Type          string    `json:"type"`
	PlayerID      string    `json:"playerId"`
	Name          string    `json:"name,omitempty"`
	RoomID        string    `json:"roomId"`
	Keep          []int     `json:"keep,omitempty"`       // 要锁定的骰子值
	Select        []int     `json:"select,omitempty"`     // 选中的骰子下标（仅用于全桌显示）
	Roll          []int     `json:"roll,omitempty"`       // 客户端物理动画的最终点数
	Poses         []DiePose `json:"poses,omitempty"`      // 客户端物理动画的最终姿态
	Trajectory    []float64 `json:"trajectory,omitempty"` // 预计算的关键帧轨迹
	StepsPerFrame int       `json:"stepsPerFrame,omitempty"`
	Text          string    `json:"text,omitempty"` // 聊天内容
}

type ServerMessage struct {
	Type      string       `json:"type"`
	State     *GameState   `json:"state,omitempty"`
	Roll      []int        `json:"roll,omitempty"`
	Selection []int        `json:"selection,omitempty"` // SELECTION 消息：当前玩家的选择
	RollID    int          `json:"rollId,omitempty"`    // SELECTION 消息对应的投掷序号
	Chat      *ChatMessage `json:"chat,omitempty"`
	Error     string       `json:"error,omitempty"`
	PlayerID  string       `json:"playerId,omitempty"`
	RoomID    string       `json:"roomId,omitempty"`
	Timestamp int64        `json:"timestamp"`
}
