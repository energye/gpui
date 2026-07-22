package kit

import (
	"github.com/energye/gpui/render/text"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/primitive"
)

// Transfer is a simplified dual-list picker (B4 simplified).
type Transfer struct {
	Root     *primitive.Flex
	Source   *List
	Target   *List
	Face     text.Face
	Theme    *core.Theme
	OnChange func(target []string)
}

// NewTransfer creates a transfer with source items.
func NewTransfer(source []string) *Transfer {
	tr := &Transfer{}
	tr.Source = NewList(source...)
	tr.Target = NewList()
	tr.Source.Face = tr.Face
	tr.Target.Face = tr.Face
	tr.Source.OnSelect = func(i int, item string) {
		// move to target
		items := append([]string{}, tr.Source.Items...)
		if i < 0 || i >= len(items) {
			return
		}
		moved := items[i]
		tr.Source.SetItems(append(items[:i], items[i+1:]...))
		tr.Target.SetItems(append(tr.Target.Items, moved))
		if tr.OnChange != nil {
			tr.OnChange(tr.Target.Items)
		}
	}
	tr.Target.OnSelect = func(i int, item string) {
		items := append([]string{}, tr.Target.Items...)
		if i < 0 || i >= len(items) {
			return
		}
		moved := items[i]
		tr.Target.SetItems(append(items[:i], items[i+1:]...))
		tr.Source.SetItems(append(tr.Source.Items, moved))
		if tr.OnChange != nil {
			tr.OnChange(tr.Target.Items)
		}
	}
	tr.Root = primitive.Row(tr.Source.Node(), tr.Target.Node())
	tr.Root.Gap = 16 // Ant Transfer list gap
	return tr
}

// Node returns the root.
func (tr *Transfer) Node() core.Node { return tr.Root }

// TargetItems returns a copy of target list items.
func (tr *Transfer) TargetItems() []string {
	if tr == nil || tr.Target == nil {
		return nil
	}
	return append([]string(nil), tr.Target.Items...)
}

// MoveAllToTarget moves every source item into the target list.
func (tr *Transfer) MoveAllToTarget() {
	if tr == nil || tr.Source == nil || tr.Target == nil {
		return
	}
	src := append([]string(nil), tr.Source.Items...)
	if len(src) == 0 {
		return
	}
	tr.Source.SetItems(nil)
	tr.Target.SetItems(append(append([]string(nil), tr.Target.Items...), src...))
	if tr.OnChange != nil {
		tr.OnChange(tr.Target.Items)
	}
}

// ClearTarget moves all target items back to source.
func (tr *Transfer) ClearTarget() {
	if tr == nil || tr.Source == nil || tr.Target == nil {
		return
	}
	tgt := append([]string(nil), tr.Target.Items...)
	if len(tgt) == 0 {
		return
	}
	tr.Target.SetItems(nil)
	tr.Source.SetItems(append(append([]string(nil), tr.Source.Items...), tgt...))
	if tr.OnChange != nil {
		tr.OnChange(tr.Target.Items)
	}
}
