package realtime

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"

	"jiaohao/internal/logx"
)

const redisChannel = "jiaohao.realtime"

type wireMessage struct {
	Origin  string         `json:"origin"`
	Topics  []string       `json:"topics"`
	Payload map[string]any `json:"payload"`
}

type Bus struct {
	hub    *Hub
	origin string
	redis  *redis.Client
	log    *logx.Logger
}

func NewBus(hub *Hub, redisURL string, log *logx.Logger) (*Bus, error) {
	bus := &Bus{hub: hub, origin: uuid.NewString(), log: log}
	if redisURL == "" {
		return bus, nil
	}
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, err
	}
	client := redis.NewClient(opt)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, err
	}
	bus.redis = client
	return bus, nil
}

func (b *Bus) Enabled() bool {
	return b != nil && b.redis != nil
}

func (b *Bus) Publish(topics []string, payload map[string]any) {
	if b == nil || b.hub == nil {
		return
	}
	b.hub.Publish(topics, payload)
	if b.redis == nil {
		return
	}
	body, err := json.Marshal(wireMessage{Origin: b.origin, Topics: topics, Payload: payload})
	if err != nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := b.redis.Publish(ctx, redisChannel, body).Err(); err != nil && b.log != nil {
		b.log.Error("realtime.redis_publish", map[string]any{"outcome": "error", "error": err.Error()})
	}
}

func (b *Bus) Start(ctx context.Context) {
	if b == nil || b.redis == nil {
		return
	}
	go func() {
		sub := b.redis.Subscribe(ctx, redisChannel)
		defer sub.Close()
		ch := sub.Channel()
		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-ch:
				if !ok {
					return
				}
				var wire wireMessage
				if err := json.Unmarshal([]byte(msg.Payload), &wire); err != nil || wire.Origin == b.origin {
					continue
				}
				b.hub.Publish(wire.Topics, wire.Payload)
			}
		}
	}()
}

func (b *Bus) Close() {
	if b != nil && b.redis != nil {
		_ = b.redis.Close()
	}
}
