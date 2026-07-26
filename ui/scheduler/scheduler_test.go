package scheduler_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/energye/gpui/ui/scheduler"
)

type onceTicker struct{ n int }

func (t *onceTicker) Tick(dt float64) bool {
	t.n++
	return t.n < 2
}

func TestMetrics_JSON(t *testing.T) {
	s := scheduler.New()
	s.Metrics().NotePresent()
	s.Metrics().SetPipeline(1, 2)
	b, err := s.Metrics().JSON()
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatal(err)
	}
	if m["present_count"].(float64) != 1 {
		t.Fatalf("metrics=%s", b)
	}
}

func TestSchedule_PendingAndMode(t *testing.T) {
	s := scheduler.New()
	if s.WaitTimeout() != -1 {
		t.Fatal("idle should wait forever")
	}
	s.ScheduleFrame()
	s.RecomputeMode()
	if s.Mode() != scheduler.ModeTransient {
		t.Fatalf("mode=%v", s.Mode())
	}
	if s.WaitTimeout() != 0 {
		t.Fatal("transient pending should poll")
	}
	s.ClearPending()
	s.RecomputeMode()
	if s.Mode() != scheduler.ModeIdle {
		t.Fatalf("mode=%v", s.Mode())
	}
}

func TestTickers(t *testing.T) {
	s := scheduler.New()
	tk := &onceTicker{}
	s.Tickers().Add(tk)
	s.RecomputeMode()
	if s.Mode() != scheduler.ModePersistent {
		t.Fatal(s.Mode())
	}
	if s.WaitTimeout() != scheduler.DefaultAnimTick {
		t.Fatalf("timeout=%v", s.WaitTimeout())
	}
	s.Tick()
	if !s.Tickers().HasActive() {
		t.Fatal("expected still active after first tick")
	}
	s.Tick()
	if s.Tickers().HasActive() {
		t.Fatal("expected removed after second tick")
	}
}

func TestMetrics_Interval(t *testing.T) {
	s := scheduler.New()
	s.Metrics().NoteFrameInterval(time.Now())
	time.Sleep(5 * time.Millisecond)
	s.Metrics().NoteFrameInterval(time.Now())
	m := s.Metrics().Snapshot()
	if m.FrameCount != 2 || m.LastFrameIntervalMs <= 0 {
		t.Fatalf("%+v", m)
	}
}
