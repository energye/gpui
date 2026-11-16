package scheduler_test

import (
	"strings"
	"testing"

	"github.com/energye/gpui/ui/scheduler"
)

func TestMetrics_CompositeStats(t *testing.T) {
	s := scheduler.New().Metrics()
	s.SetCompositeStats(52, 0, 88, "fresh_target")

	snap := s.Snapshot()
	if snap.CompositeLayersVisited != 52 || snap.CompositeLayersCulled != 0 ||
		snap.CompositeBlitsIssued != 88 || snap.CompositeFullReason != "fresh_target" {
		t.Fatalf("composite snapshot = %+v", snap)
	}

	b, err := s.JSON()
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{
		`"composite_layers_visited":52`,
		`"composite_layers_culled":0`,
		`"composite_blits_issued":88`,
		`"composite_full_reason":"fresh_target"`,
	} {
		if !strings.Contains(string(b), key) {
			t.Fatalf("JSON missing %s: %s", key, b)
		}
	}
}
