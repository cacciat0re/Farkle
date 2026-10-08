package routes

import (
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

const (
	writeWait  = 10 * time.Second
	pongWait   = 60 * time.Second
	pingPeriod = 45 * time.Second
	// ROLL_RESULT 会带上整段关键帧轨迹（每帧 6 颗骰子 × 7 个数），
	// 单条 JSON 常见 40~100KB。这里必须给足余量，否则 ReadMessage 会判定超限
	// 直接断开连接，表现为"投掷后卡死 + 对手看到自己掉线"。
	maxMessageSize = 512 * 1024
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true }, // 开发阶段放开跨域
}
