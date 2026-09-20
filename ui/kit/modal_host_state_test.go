package kit_test

import (
	"testing"

	"github.com/energye/gpui/ui/kit"
)

func TestModalHost_StackOrderAndSingleClose(t *testing.T) {
	host := kit.BuildModalHost(kit.DefaultScopeCtx(), kit.DefaultModalHostProps())
	h1 := host.Confirm(kit.DefaultModalHostConfirmConfig())
	h2 := host.Info(kit.DefaultModalHostConfirmConfig())
	h3 := host.Warning(kit.DefaultModalHostConfirmConfig())
	if host.Depth() != 3 {
		t.Fatalf("depth = %d want 3", host.Depth())
	}
	ents := host.Entries()
	if len(ents) != 3 {
		t.Fatalf("entries = %d want 3", len(ents))
	}
	if !(ents[0].ZIndex < ents[1].ZIndex && ents[1].ZIndex < ents[2].ZIndex) {
		t.Fatalf("z order broken %+v", ents)
	}
	cases := loadHostCases(t)
	want := cases["stack_z"].(map[string]any)["want"].([]any)
	if float64(ents[0].ZIndex) != want[0].(float64) || float64(ents[1].ZIndex) != want[1].(float64) || float64(ents[2].ZIndex) != want[2].(float64) {
		t.Fatalf("z = %d,%d,%d want 1000,1010,1020", ents[0].ZIndex, ents[1].ZIndex, ents[2].ZIndex)
	}
	h2.Destroy()
	if host.Depth() != 2 {
		t.Fatalf("after single close depth = %d want 2", host.Depth())
	}
	ents = host.Entries()
	if ents[0].ID != h1.ID() || ents[1].ID != h3.ID() {
		t.Fatal("neighbours must keep order after middle close")
	}
	select {
	case r := <-h2.Done():
		if r.Confirmed {
			t.Fatal("destroy must resolve cancel")
		}
	default:
		t.Fatal("destroyed handle promise must resolve")
	}
	if !host.ScrollLocked() {
		t.Fatal("scroll must lock while entries open")
	}
	host.DestroyAll()
	if host.Depth() != 0 || host.ScrollLocked() {
		t.Fatal("destroyAll must clear stack and unlock")
	}
}
