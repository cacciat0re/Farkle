package models

// TableStatus 大厅接口返回的桌子状态
type TableStatus struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	MaxPlayers int    `json:"maxPlayers"`
	Players    int    `json:"players"`
	InGame     bool   `json:"inGame"`
}

type LobbyMessage struct {
	Type      string        `json:"type"`
	Tables    []TableStatus `json:"tables"`
	Timestamp int64         `json:"timestamp"`
}
