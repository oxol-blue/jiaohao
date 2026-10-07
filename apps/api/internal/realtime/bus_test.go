package realtime

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"

	"jiaohao/internal/logx"
)

func TestRedisFanoutReachesOtherInstance(t *testing.T) {
	mr := miniredis.RunT(t)
	url := "redis://" + mr.Addr()
	log := logx.New("error")
	hubA := NewHub()
	hubB := NewHub()
	busA, err := NewBus(hubA, url, log)
	if err != nil {
		t.Fatal(err)
	}
	busB, err := NewBus(hubB, url, log)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(busA.Close)
	t.Cleanup(busB.Close)
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	busA.Start(ctx)
	busB.Start(ctx)

	local := &client{send: make(chan []byte, 4), topics: map[string]struct{}{"window:1": {}}}
	remote := &client{send: make(chan []byte, 4), topics: map[string]struct{}{"window:1": {}}}
	hubA.add(local)
	hubB.add(remote)

	deadline := time.Now().Add(2 * time.Second)
	for {
		busA.Publish([]string{"window:1"}, map[string]any{"event": "queue.window.updated", "probe": true})
		select {
		case raw := <-remote.send:
			var msg map[string]any
			if err := json.Unmarshal(raw, &msg); err != nil {
				t.Fatal(err)
			}
			if msg["event"] != "queue.window.updated" {
				t.Fatalf("event=%v", msg["event"])
			}
			return
		case <-time.After(50 * time.Millisecond):
			if time.Now().After(deadline) {
				t.Fatal("other instance did not receive the event")
			}
		}
		drain(local.send)
	}
}

func drain(ch chan []byte) {
	for {
		select {
		case <-ch:
		default:
			return
		}
	}
}
