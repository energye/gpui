package textinput

import (
	"github.com/energye/gpui/ui/platform"
	"github.com/energye/gpui/ui/rendering"
)

// BaseEditable 是 F-E0a 的 ≤15 行接入基座，封装 Editor/IMERect/ContentType/DrawPreedit 四件套。
// 业务控件嵌入它即可拥有占位/禁用/只读等通用逻辑，单测与真窗同源。
type BaseEditable struct {
	ed          *Editor
	placeholder string
	disabled    bool
	purpose     platform.ContentPurpose
}

// NewBaseEditable 创建基座，ed 非空。
func NewBaseEditable(ed *Editor) *BaseEditable { return &BaseEditable{ed: ed} }

// Editor 返回编辑状态。
func (b *BaseEditable) Editor() *Editor { if b == nil { return nil }; return b.ed }

// ContentType 返回输入法类型（可配置，默认 Normal，B13）。
func (b *BaseEditable) ContentType() platform.ContentType {
	if b == nil {
		return platform.ContentType{Purpose: platform.PurposeNormal}
	}
	return platform.ContentType{Purpose: b.purpose}
}

// SetContentType 设置输入法类型（B13：替代硬编码 Normal）。
func (b *BaseEditable) SetContentType(p platform.ContentPurpose) {
	if b != nil {
		b.purpose = p
	}
}

// SetPurpose 兼容别名
func (b *BaseEditable) SetPurpose(p platform.ContentPurpose) { b.SetContentType(p) }

// IMERect 返回零（嵌入方应覆盖为 caret/composing 锚点）。
func (b *BaseEditable) IMERect() platform.Rect { return platform.Rect{} }

// DrawPreedit 高亮 composing 区间（TextLayout 行盒并集半透明覆盖）。
func (b *BaseEditable) DrawPreedit(pc *rendering.PaintContext, text string, composing TextRange, lay *rendering.TextLayout) {
	if pc == nil || pc.DC == nil || lay == nil || composing.Collapsed() {
		return
	}
	s := byteOffsetForUtf16(text, composing.Start())
	e := byteOffsetForUtf16(text, composing.End())
	for _, r := range lay.BoxesForRange(s, e) {
		pc.DC.SetRGBA(0.30, 0.60, 1.0, 0.22)
		pc.DC.DrawRectangle(pc.OriginX+r.Min.X, pc.OriginY+r.Min.Y, r.Size().Width, r.Size().Height)
		_ = pc.DC.Fill()
	}
}

// SetPlaceholder 设置占位 hint（空+未聚焦才显，不进缓冲）。
func (b *BaseEditable) SetPlaceholder(s string) { if b != nil { b.placeholder = s } }

// Placeholder 返回占位。
func (b *BaseEditable) Placeholder() string { if b == nil { return "" }; return b.placeholder }

// SetDisabled 设置禁用态（样式置灰，编辑仍由 Editor.readOnly 控制）。
func (b *BaseEditable) SetDisabled(v bool) {
	if b == nil {
		return
	}
	b.disabled = v
	if b.ed != nil {
		b.ed.SetReadOnly(v)
	}
}

// Disabled 返回禁用态。
func (b *BaseEditable) Disabled() bool { if b == nil { return false }; return b.disabled }
