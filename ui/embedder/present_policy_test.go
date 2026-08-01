package embedder_test

import (
	"testing"

	"github.com/energye/gpui/ui/embedder"
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/rendering"
	"github.com/energye/gpui/ui/scheduler"
)

// TestNewPipelineApp_DefaultsPresentPolicyRetained: W2 retained is the engine
// default policy and appears on Metrics snapshot/JSON without a real window open.
func TestNewPipelineApp_DefaultsPresentPolicyRetained(t *testing.T) {
	host := platform.NewStubHost(64, 64)
	root := rendering.NewAbsoluteBox(64, 64)
	app := embedder.NewPipelineApp(host, root, embedder.PipelineOptions{})
	m := app.Metrics()
	if m == nil {
		t.Fatal("nil metrics")
	}
	if got := m.PresentPolicy(); got != scheduler.PresentPolicyRetained {
		t.Fatalf("PresentPolicy=%q want %q", got, scheduler.PresentPolicyRetained)
	}
	b, err := m.JSON()
	if err != nil {
		t.Fatal(err)
	}
	js := string(b)
	if !containsSub(js, `"present_policy"`) || !containsSub(js, "retained") {
		t.Fatalf("JSON missing present_policy: %s", js)
	}
}

func containsSub(s, sub string) bool {
	return len(s) >= len(sub) && (func() bool {
		for i := 0; i+len(sub) <= len(s); i++ {
			if s[i:i+len(sub)] == sub {
				return true
			}
		}
		return false
	})()
}
