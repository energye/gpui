package scheduler_test

import (
	"testing"

	"github.com/energye/gpui/ui/scheduler"
)

type lateChild struct {
	n int
}

func (t *lateChild) Tick(dt float64) bool {
	t.n++
	return true
}

type lateAdder struct {
	reg   *scheduler.TickerRegistry
	child *lateChild
	n     int
}

func (t *lateAdder) Tick(dt float64) bool {
	t.n++
	if t.child != nil && t.n == 1 {
		t.reg.Add(t.child)
	}
	return true
}

func TestTickers_LateAddSurvives(t *testing.T) {
	s := scheduler.New()
	child := &lateChild{}
	adder := &lateAdder{reg: s.Tickers(), child: child}
	s.Tickers().Add(adder)
	s.Tickers().TickAll(1.0 / 60)
	if child.n != 0 {
		t.Fatalf("late-added ticker must wait for next round, ticks=%d", child.n)
	}
	if !s.Tickers().HasActive() {
		t.Fatal("registry must stay active")
	}
	s.Tickers().TickAll(1.0 / 60)
	if child.n != 1 {
		t.Fatalf("late-added ticker must tick next round, ticks=%d", child.n)
	}
	if adder.n != 2 {
		t.Fatalf("adder ticks=%d want 2", adder.n)
	}
}
