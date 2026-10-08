package services

import (
	"encoding/json"
	"sync"

	"farkle-server/models"
)

// Hub 管理所有活跃桌子（房间）和大厅订阅者。
type Hub struct {
	mu           sync.RWMutex
	Rooms        map[string]*Room
	tables       []models.Table // 启动时从数据库加载的桌子，保持大厅展示顺序
	lobbyMu      sync.Mutex
	lobbyClients map[LobbyPeer]struct{}
}

// NewHub 按数据库中的桌子列表预热所有房间（服务器满载 = 桌子数）
func NewHub(tables []models.Table) *Hub {
	h := &Hub{
		Rooms:        make(map[string]*Room),
		tables:       tables,
		lobbyClients: make(map[LobbyPeer]struct{}),
	}
	for _, t := range tables {
		h.Rooms[t.ID] = NewRoom(t, h.BroadcastTables)
		go h.Rooms[t.ID].Run()
	}
	return h
}

// GetRoom 仅返回已加载的桌子
func (h *Hub) GetRoom(id string) *Room {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.Rooms[id]
}

// ListTables 返回所有桌子的实时状态（固定集合，启动后不会动态增删）
func (h *Hub) ListTables() []models.TableStatus {
	h.mu.RLock()
	defer h.mu.RUnlock()
	status := make([]models.TableStatus, 0, len(h.tables))
	for _, t := range h.tables {
		room := h.Rooms[t.ID]
		players, inGame := room.Status()
		status = append(status, models.TableStatus{
			ID:         t.ID,
			Name:       t.Name,
			MaxPlayers: t.MaxPlayers,
			Players:    players,
			InGame:     inGame,
		})
	}
	return status
}

func (h *Hub) RegisterLobby(c LobbyPeer) {
	h.lobbyMu.Lock()
	h.lobbyClients[c] = struct{}{}
	h.lobbyMu.Unlock()
	h.BroadcastTables()
}

func (h *Hub) UnregisterLobby(c LobbyPeer) {
	h.lobbyMu.Lock()
	defer h.lobbyMu.Unlock()
	if _, ok := h.lobbyClients[c]; ok {
		delete(h.lobbyClients, c)
		c.Close()
	}
}

// BroadcastTables 只在桌子人数或对局状态变化时推送，不做定时轮询。
func (h *Hub) BroadcastTables() {
	payload, err := json.Marshal(models.LobbyMessage{
		Type:      "TABLE_STATUS",
		Tables:    h.ListTables(),
		Timestamp: nowMs(),
	})
	if err != nil {
		return
	}

	h.lobbyMu.Lock()
	defer h.lobbyMu.Unlock()
	for c := range h.lobbyClients {
		if !c.Push(payload) {
			delete(h.lobbyClients, c)
			c.Close()
		}
	}
}
