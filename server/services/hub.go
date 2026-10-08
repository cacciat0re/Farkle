package services

import (
	"encoding/json"
	"sync"

	"farkle-server/config"
	"farkle-server/models"
)

// Hub 管理所有活跃桌子（房间）和大厅订阅者。
type Hub struct {
	mu           sync.RWMutex
	Rooms        map[string]*Room
	Config       *config.Config
	lobbyMu      sync.Mutex
	lobbyClients map[LobbyPeer]struct{}
}

func NewHub(cfg *config.Config) *Hub {
	h := &Hub{
		Rooms:        make(map[string]*Room),
		Config:       cfg,
		lobbyClients: make(map[LobbyPeer]struct{}),
	}
	// 按配置预热所有桌子（服务器满载 = 配置中的桌子数）
	for _, t := range cfg.Tables {
		h.Rooms[t.ID] = NewRoom(t, h.BroadcastTables)
		go h.Rooms[t.ID].Run()
	}
	return h
}

// GetRoom 仅返回配置中存在的桌子
func (h *Hub) GetRoom(id string) *Room {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.Rooms[id]
}

// ListTables 返回所有桌子的实时状态（固定集合，不会动态增删）
func (h *Hub) ListTables() []models.TableStatus {
	h.mu.RLock()
	defer h.mu.RUnlock()
	status := make([]models.TableStatus, 0, len(h.Config.Tables))
	for _, t := range h.Config.Tables {
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
