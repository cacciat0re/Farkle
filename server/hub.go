package main

import "sync"

// Hub 管理所有活跃房间
type Hub struct {
	mu      sync.RWMutex
	Rooms   map[string]*Room
}

func NewHub() *Hub {
	return &Hub{
		Rooms: make(map[string]*Room),
	}
}

// GetOrCreateRoom 获取或创建房间
func (h *Hub) GetOrCreateRoom(id string) *Room {
	h.mu.Lock()
	defer h.mu.Unlock()
	if r, ok := h.Rooms[id]; ok {
		return r
	}
	r := NewRoom(id)
	h.Rooms[id] = r
	go r.Run()
	return r
}

// RemoveRoom 当房间无人时回收
func (h *Hub) RemoveRoom(id string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if r, ok := h.Rooms[id]; ok && r.IsEmpty() {
		close(r.done)
		delete(h.Rooms, id)
	}
}
