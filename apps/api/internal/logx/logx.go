package logx

import (
	"encoding/json"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

type Logger struct {
	out   io.Writer
	level int
	mu    sync.Mutex
}

const (
	levelDebug = 10
	levelInfo  = 20
	levelWarn  = 30
	levelError = 40
)

func New(level string) *Logger {
	return &Logger{out: os.Stdout, level: parseLevel(level)}
}

func parseLevel(level string) int {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		return levelDebug
	case "warn", "warning":
		return levelWarn
	case "error":
		return levelError
	default:
		return levelInfo
	}
}

func (l *Logger) Debug(event string, fields map[string]any) { l.write("DEBUG", event, fields) }
func (l *Logger) Info(event string, fields map[string]any)  { l.write("INFO", event, fields) }
func (l *Logger) Warn(event string, fields map[string]any)  { l.write("WARN", event, fields) }
func (l *Logger) Error(event string, fields map[string]any) { l.write("ERROR", event, fields) }

func levelRank(level string) int {
	switch strings.ToUpper(strings.TrimSpace(level)) {
	case "DEBUG":
		return levelDebug
	case "WARN", "WARNING":
		return levelWarn
	case "ERROR":
		return levelError
	default:
		return levelInfo
	}
}

func (l *Logger) write(level, event string, fields map[string]any) {
	if l == nil {
		return
	}
	if levelRank(level) < l.level {
		return
	}
	rec := map[string]any{
		"ts":    time.Now().Format(time.RFC3339Nano),
		"level": level,
		"event": event,
	}
	for k, v := range fields {
		if k == "password" || k == "token" || k == "authorization" {
			continue
		}
		rec[k] = v
	}
	b, err := json.Marshal(rec)
	if err != nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	_, _ = l.out.Write(append(b, '\n'))
}
