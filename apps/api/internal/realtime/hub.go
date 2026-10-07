package realtime

import (
	"encoding/json"
	"sync"
)

type Envelope struct {
	Event string         `json:"event"`
	Body  map[string]any `json:"-"`
}

type Message map[string]any

type client struct {
	send   chan []byte
	topics map[string]struct{}
	userID string
}

type Hub struct {
	mu      sync.RWMutex
	clients map[*client]struct{}
}

func NewHub() *Hub {
	return &Hub{clients: map[*client]struct{}{}}
}

func (h *Hub) add(c *client) {
	h.mu.Lock()
	h.clients[c] = struct{}{}
	h.mu.Unlock()
}

func (h *Hub) remove(c *client) {
	h.mu.Lock()
	delete(h.clients, c)
	h.mu.Unlock()
}

func (h *Hub) Publish(topics []string, payload map[string]any) {
	b, err := json.Marshal(payload)
	if err != nil {
		return
	}
	want := map[string]struct{}{}
	for _, t := range topics {
		want[t] = struct{}{}
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for c := range h.clients {
		for topic := range c.topics {
			if _, ok := want[topic]; ok {
				select {
				case c.send <- b:
				default:
				}
				break
			}
		}
	}
}
