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
