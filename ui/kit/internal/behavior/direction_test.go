package behavior_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/energye/gpui/ui/kit/internal/behavior"
	"github.com/energye/gpui/ui/kit/internal/scope"
)

type directionFile struct {
	Mirror struct {
		X     float64 `json:"x"`
		Width float64 `json:"width"`
		Dir   string  `json:"dir"`
		Want  float64 `json:"want"`
	} `json:"mirror"`
	Arrows struct {
		Prev        string `json:"prev"`
		Next        string `json:"next"`
		WantPrevRTL string `json:"wantPrevRTL"`
		WantNextRTL string `json:"wantNextRTL"`
	} `json:"arrows"`
	Empty struct {
		Component string `json:"component"`
		Locale    string `json:"locale"`
		Want      string `json:"want"`
	} `json:"empty"`
}

func loadDirection(t *testing.T) directionFile {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "direction_cases.json"))
	if err != nil {
		t.Fatalf("read direction_cases.json: %v", err)
	}
	var f directionFile
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatalf("decode direction_cases.json: %v", err)
	}
	return f
}

func TestDirection_MirrorAndArrows(t *testing.T) {
	f := loadDirection(t)
	got := behavior.MirrorX(f.Mirror.X, f.Mirror.Width, scope.Direction(f.Mirror.Dir))
	if got != f.Mirror.Want {
		t.Fatalf("mirror=%v want %v", got, f.Mirror.Want)
	}
	if got := behavior.MirrorX(100, 1200, scope.DirLTR); got != 100 {
		t.Fatalf("ltr must not mirror: %v", got)
	}
	prev, next := behavior.ArrowPrevNext(scope.DirRTL, f.Arrows.Prev, f.Arrows.Next)
	if prev != f.Arrows.WantPrevRTL || next != f.Arrows.WantNextRTL {
		t.Fatalf("rtl arrows=(%q,%q) want (%q,%q)", prev, next, f.Arrows.WantPrevRTL, f.Arrows.WantNextRTL)
	}
	ctx := scope.DefaultCtx().WithDir(scope.DirRTL)
	if !behavior.IsRTL(ctx) {
		t.Fatal("rtl ctx must report rtl")
	}
	if behavior.IsRTL(scope.DefaultCtx()) {
		t.Fatal("default ctx must be ltr")
	}
}

func TestDirection_EmptyText(t *testing.T) {
	f := loadDirection(t)
	ctx := scope.DefaultCtx().WithLocale(f.Empty.Locale)
	if got := behavior.EmptyText(ctx, f.Empty.Component); got != f.Empty.Want {
		t.Fatalf("empty=%q want %q", got, f.Empty.Want)
	}
}
