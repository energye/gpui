package kit_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/energye/gpui/ui/kit"
)

func loadNoticeCases(t *testing.T) map[string]any {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "notice_queue_cases.json"))
	if err != nil {
		t.Fatalf("read cases: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("parse cases: %v", err)
	}
	return m
}

func TestNoticeQueue_DefaultsMatchCases(t *testing.T) {
	cases := loadNoticeCases(t)
	def := cases["default_props"].(map[string]any)
	p := kit.DefaultNoticeQueueProps()
	if float64(p.ZIndexBase) != def["z_base"].(float64) {
		t.Fatalf("zbase = %d want 1000", p.ZIndexBase)
	}
	if p.MessageTop != def["message_top"].(float64) {
		t.Fatalf("message top = %v want 8", p.MessageTop)
	}
	if p.MessageDuration != def["message_duration"].(float64) {
		t.Fatalf("message duration = %v want 3", p.MessageDuration)
	}
	if p.NotificationDuration != def["notification_duration"].(float64) {
		t.Fatalf("notification duration = %v want 4.5", p.NotificationDuration)
	}
	if string(p.NotificationPlacement) != def["notification_placement"].(string) {
		t.Fatalf("placement = %q want topRight", p.NotificationPlacement)
	}
	if p.StackThreshold != int(def["stack_threshold"].(float64)) {
		t.Fatalf("threshold = %d want 3", p.StackThreshold)
	}
	if p.PauseOnHover != true {
		t.Fatal("pauseOnHover must default true")
	}
	dm := cases["default_message"].(map[string]any)
	if dm["top"].(float64) != 8 || dm["duration"].(float64) != 3 {
		t.Fatalf("message defaults = %v", dm)
	}
	dn := cases["default_notification"].(map[string]any)
	if dn["width"].(float64) != 384 || dn["duration"].(float64) != 4.5 {
		t.Fatalf("notification defaults = %v", dn)
	}
	if dn["placement"].(string) != "topRight" {
		t.Fatalf("notification placement = %v", dn["placement"])
	}
}
