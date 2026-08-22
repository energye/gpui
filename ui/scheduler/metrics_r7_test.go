package scheduler_test

import (
	"strings"
	"testing"

	"github.com/energye/gpui/ui/scheduler"
)

// R7/R7b: virtual-list bind window + scroll fresh-mount totals in JSON.
// Fields stay omitted until a VirtualList has bound (windows without lists).
func TestMetrics_VirtualBind_JSONKeys(t *testing.T) {
	s := scheduler.New().Metrics()

	b, err := s.JSON()
	if err != nil {
		t.Fatal(err)
	}
	js := string(b)
	for _, key := range []string{"bind_count", "item_count", "scroll_rerecord"} {
		if strings.Contains(js, key) {
			t.Fatalf("fresh store must omit %q: %s", key, js)
		}
	}

	s.SetVirtualBind(30, 1000)
	s.SetScrollRerecord(42)
	b, err = s.JSON()
	if err != nil {
		t.Fatal(err)
	}
	js = string(b)
	for _, key := range []string{`"bind_count":30`, `"item_count":1000`, `"scroll_rerecord":42`} {
		if !strings.Contains(js, key) {
			t.Fatalf("JSON missing %s: %s", key, js)
		}
	}
}
