package input

import (
	"testing"

	"github.com/energye/gpui/ui/platform"
)

func TestFromPlatform_DragEnterOverLeave(t *testing.T) {
	enter := FromPlatform(platform.Event{
		Type: platform.EventDragEnter, X: 3, Y: 4, MIMETypes: []string{"text/uri-list", "text/plain"},
	}, Modifiers{})
	if enter.Kind != KindDragEnter {
		t.Fatalf("enter kind = %s, want drag-enter", enter.Kind)
	}
	if enter.Drag.X != 3 || enter.Drag.Y != 4 {
		t.Fatalf("enter pos = (%.1f,%.1f)", enter.Drag.X, enter.Drag.Y)
	}
	if len(enter.Drag.MIMETypes) != 2 || enter.Drag.MIMETypes[0] != "text/uri-list" {
		t.Fatalf("enter mimes = %v", enter.Drag.MIMETypes)
	}
	over := FromPlatform(platform.Event{
		Type: platform.EventDragOver, X: 5, Y: 6, MIMETypes: []string{"text/uri-list"},
	}, Modifiers{})
	if over.Kind != KindDragOver || over.Drag.X != 5 || over.Drag.Y != 6 {
		t.Fatalf("over = %s %+v", over.Kind, over.Drag)
	}
	leave := FromPlatform(platform.Event{Type: platform.EventDragLeave}, Modifiers{})
	if leave.Kind != KindDragLeave {
		t.Fatalf("leave kind = %s, want drag-leave", leave.Kind)
	}
	if len(leave.Drag.MIMETypes) != 0 || leave.Drag.X != 0 || len(leave.Drag.Files) != 0 {
		t.Fatalf("leave must carry zero drag: %+v", leave.Drag)
	}
}

func TestFromPlatform_DropDataClone(t *testing.T) {
	src := map[string][]byte{"text/plain": []byte("hi"), "image/png": {1, 2, 3}}
	ev := FromPlatform(platform.Event{
		Type: platform.EventDrop, X: 1, Y: 2,
		Files:    []string{"/tmp/a"},
		DropData: src,
	}, Modifiers{})
	if ev.Kind != KindDrop {
		t.Fatalf("kind = %s, want drop", ev.Kind)
	}
	if string(ev.Drag.Data["text/plain"]) != "hi" || len(ev.Drag.Data["image/png"]) != 3 {
		t.Fatalf("data = %q, want cloned payloads", ev.Drag.Data)
	}
	// Mutating the backend buffers must not alias the unified event.
	src["text/plain"][0] = 'X'
	ev.Drag.Files[0] = "/tmp/changed"
	if string(ev.Drag.Data["text/plain"]) != "hi" {
		t.Fatal("unified Data aliases backend buffer")
	}
	empty := FromPlatform(platform.Event{Type: platform.EventDrop}, Modifiers{})
	if empty.Drag.Data != nil || empty.Drag.Files != nil {
		t.Fatalf("empty drop = %+v, want nil payloads", empty.Drag)
	}
}
