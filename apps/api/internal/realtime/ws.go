package realtime

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"

	"jiaohao/internal/httpx"
	"jiaohao/internal/identity"
	"jiaohao/internal/logx"
)

type Handler struct {
	hub      *Hub
	identity *identity.Service
	log      *logx.Logger
	upgrader websocket.Upgrader
}

func NewHandler(hub *Hub, ids *identity.Service, origins []string, log *logx.Logger) *Handler {
	allowed := map[string]struct{}{}
	for _, o := range origins {
		allowed[o] = struct{}{}
	}
	return &Handler{
		hub:      hub,
		identity: ids,
		log:      log,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			CheckOrigin: func(r *http.Request) bool {
				origin := r.Header.Get("Origin")
				if origin == "" {
					return true
				}
				if _, ok := allowed[origin]; ok {
					return true
				}
				return strings.HasPrefix(origin, "http://127.0.0.1") ||
					strings.HasPrefix(origin, "http://localhost") ||
					strings.HasPrefix(origin, "https://localhost")
			},
		},
	}
}

type inMsg struct {
	Type   string   `json:"type"`
	Token  string   `json:"token"`
	Topics []string `json:"topics"`
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	user, err := h.identity.LoadSession(r.Context(), r.URL.Query().Get("token"))
	if err != nil {
		httpx.WriteError(w, http.StatusUnauthorized, httpx.CodeUnauthenticated, "请先登录")
		return
	}
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.log.Warn("ws.upgrade", map[string]any{"outcome": "error", "error": err.Error()})
		return
	}
	c := &client{
		send:   make(chan []byte, 16),
		topics: map[string]struct{}{},
		userID: user.ID.String(),
	}
	h.hub.add(c)
	defer func() {
		h.hub.remove(c)
		close(c.send)
		_ = conn.Close()
	}()

	go func() {
		for msg := range c.send {
			_ = conn.SetWriteDeadline(time.Now().Add(8 * time.Second))
			if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		}
	}()

	conn.SetReadLimit(4096)
	_ = conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	conn.SetPongHandler(func(string) error {
		_ = conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, raw, err := conn.ReadMessage()
		if err != nil {
			return
		}
		var msg inMsg
		if err := json.Unmarshal(raw, &msg); err != nil {
			continue
		}
		if msg.Type != "subscribe" {
			continue
		}
		next := map[string]struct{}{}
		for _, topic := range msg.Topics {
			if mapped, ok := h.allowTopic(user, topic); ok {
				next[mapped] = struct{}{}
			}
		}
		c.topics = next
	}
}

func (h *Handler) allowTopic(user identity.User, topic string) (string, bool) {
	switch {
	case topic == "ticket:mine":
		return "ticket:" + user.ID.String(), true
	case strings.HasPrefix(topic, "window:"):
		return topic, true
	default:
		return "", false
	}
}
