package semantics_test

import (
	"testing"

	"github.com/energye/gpui/ui/semantics"
)

func TestSemantics_ReadWrite(t *testing.T) {
	n := semantics.New(semantics.RoleButton, "OK")
	if n.Role != semantics.RoleButton || n.Label != "OK" {
		t.Fatalf("%+v", n)
	}
	n.Focusable = true
	n.Value = "pressed"
	if !n.Focusable || n.Value != "pressed" {
		t.Fatal("fields")
	}
}

func TestSemantics_Flatten(t *testing.T) {
	root := semantics.New(semantics.RoleGeneric, "root")
	root.Add(semantics.New(semantics.RoleButton, "A"))
	list := root.Add(semantics.New(semantics.RoleList, "items"))
	list.Add(semantics.New(semantics.RoleListItem, "1"))
	list.Add(semantics.New(semantics.RoleListItem, "2"))

	flat := semantics.Flatten(root)
	if len(flat) != 5 {
		t.Fatalf("len=%d %v", len(flat), flat)
	}
	if flat[0].Depth != 0 || flat[0].Label != "root" {
		t.Fatalf("root %+v", flat[0])
	}
	if flat[3].Depth != 2 || flat[3].Label != "1" {
		t.Fatalf("item %+v", flat[3])
	}
	if flat[4].Role != semantics.RoleListItem {
		t.Fatal("role")
	}
	if semantics.Flatten(nil) != nil {
		t.Fatal("want nil")
	}
}
