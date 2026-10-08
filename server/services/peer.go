package services

// Peer 是房间看到的玩家连接抽象，避免房间逻辑依赖具体 WebSocket 实现。
type Peer interface {
	PlayerID() string
	SetPlayerID(string)
	Push([]byte) bool
	Close()
}

// LobbyPeer 是大厅广播看到的连接抽象。
type LobbyPeer interface {
	Push([]byte) bool
	Close()
}
