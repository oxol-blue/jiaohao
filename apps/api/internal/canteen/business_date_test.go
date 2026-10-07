package canteen

import (
	"testing"
	"time"
)

func TestBusinessDateUsesShanghaiCalendarDay(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 10, 8, 23, 30, 0, 0, loc)
	got := BusinessDate(now, loc)
	if got.Year() != 2026 || got.Month() != 10 || got.Day() != 8 {
		t.Fatalf("got %v", got)
	}
	utc := time.Date(2026, 10, 8, 16, 30, 0, 0, time.UTC) // 次日 00:30 CST
	got = BusinessDate(utc, loc)
	if got.Day() != 9 {
		t.Fatalf("expected Oct 9 in Shanghai, got %v", got)
	}
}
