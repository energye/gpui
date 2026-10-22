package recording

import (
	"image"
	"testing"

	"github.com/energye/gpui/render"
)

func TestCommandType_String(t *testing.T) {
	tests := []struct {
		ct   CommandType
		want string
	}{
		{CmdSave, "Save"},
		{CmdRestore, "Restore"},
		{CmdConcat, "Concat"},
		{CmdSetClip, "SetClip"},
		{CmdClearClip, "ClearClip"},
		{CmdFillPath, "FillPath"},
		{CmdStrokePath, "StrokePath"},
		{CmdFillRect, "FillRect"},
		{CmdStrokeRect, "StrokeRect"},
		{CmdDrawImage, "DrawImage"},
		{CmdDrawText, "DrawText"},
		{CmdSetFillStyle, "SetFillStyle"},
		{CmdSetStrokeStyle, "SetStrokeStyle"},
		{CmdSetLineWidth, "SetLineWidth"},
		{CmdSetLineCap, "SetLineCap"},
		{CmdSetLineJoin, "SetLineJoin"},
		{CmdSetMiterLimit, "SetMiterLimit"},
		{CmdSetDash, "SetDash"},
		{CmdSetFillRule, "SetFillRule"},
		{CommandType(254), "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.ct.String(); got != tt.want {
				t.Errorf("CommandType.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCommandInterface(t *testing.T) {
	// Verify all command types implement Command interface
	commands := []Command{
		SaveCommand{},
		RestoreCommand{},
		ConcatCommand{Matrix: Identity()},
		SetClipCommand{Path: PathRef(0), Rule: FillRuleNonZero},
		ClearClipCommand{},
		FillPathCommand{Path: PathRef(0), Brush: BrushRef(0), Rule: FillRuleNonZero},
		StrokePathCommand{Path: PathRef(0), Brush: BrushRef(0), Stroke: DefaultStroke()},
		FillRectCommand{Rect: NewRect(0, 0, 100, 100), Brush: BrushRef(0)},
		StrokeRectCommand{Rect: NewRect(0, 0, 100, 100), Brush: BrushRef(0), Stroke: DefaultStroke()},
		DrawImageCommand{Image: ImageRef(0), Opts: render.DrawImageOptions{DstWidth: 100, DstHeight: 100}},
		DrawTextCommand{Text: "Hello", X: 10, Y: 20, FontSize: 12, Brush: BrushRef(0)},
		SetFillStyleCommand{Brush: BrushRef(0)},
		SetStrokeStyleCommand{Brush: BrushRef(0)},
		SetLineWidthCommand{Width: 2.0},
		SetLineCapCommand{Cap: LineCapRound},
		SetLineJoinCommand{Join: LineJoinRound},
		SetMiterLimitCommand{Limit: 4.0},
		SetDashCommand{Pattern: []float64{5, 3}, Offset: 0},
		SetFillRuleCommand{Rule: FillRuleEvenOdd},
	}

	expectedTypes := []CommandType{
		CmdSave,
		CmdRestore,
		CmdConcat,
		CmdSetClip,
		CmdClearClip,
		CmdFillPath,
		CmdStrokePath,
		CmdFillRect,
		CmdStrokeRect,
		CmdDrawImage,
		CmdDrawText,
		CmdSetFillStyle,
		CmdSetStrokeStyle,
		CmdSetLineWidth,
		CmdSetLineCap,
		CmdSetLineJoin,
		CmdSetMiterLimit,
		CmdSetDash,
		CmdSetFillRule,
	}

	if len(commands) != len(expectedTypes) {
		t.Fatalf("commands count %d != expectedTypes count %d", len(commands), len(expectedTypes))
	}

	for i, cmd := range commands {
		if got := cmd.Type(); got != expectedTypes[i] {
			t.Errorf("command[%d].Type() = %v, want %v", i, got, expectedTypes[i])
		}
	}
}

func TestPathRef_IsValid(t *testing.T) {
	tests := []struct {
		name string
		ref  PathRef
		want bool
	}{
		{"valid zero", PathRef(0), true},
		{"valid non-zero", PathRef(42), true},
		{"invalid", PathRef(InvalidRef), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.ref.IsValid(); got != tt.want {
				t.Errorf("PathRef.IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBrushRef_IsValid(t *testing.T) {
	tests := []struct {
		name string
		ref  BrushRef
		want bool
	}{
		{"valid zero", BrushRef(0), true},
		{"valid non-zero", BrushRef(42), true},
		{"invalid", BrushRef(InvalidRef), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.ref.IsValid(); got != tt.want {
				t.Errorf("BrushRef.IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestImageRef_IsValid(t *testing.T) {
	tests := []struct {
		name string
		ref  ImageRef
		want bool
	}{
		{"valid zero", ImageRef(0), true},
		{"valid non-zero", ImageRef(42), true},
		{"invalid", ImageRef(InvalidRef), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.ref.IsValid(); got != tt.want {
				t.Errorf("ImageRef.IsValid() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDefaultStroke(t *testing.T) {
	s := DefaultStroke()

	if s.Width != 1.0 {
		t.Errorf("DefaultStroke().Width = %v, want 1.0", s.Width)
	}
	if s.Cap != LineCapButt {
		t.Errorf("DefaultStroke().Cap = %v, want LineCapButt", s.Cap)
	}
	if s.Join != LineJoinMiter {
		t.Errorf("DefaultStroke().Join = %v, want LineJoinMiter", s.Join)
	}
	if s.MiterLimit != 4.0 {
		t.Errorf("DefaultStroke().MiterLimit = %v, want 4.0", s.MiterLimit)
	}
	if s.DashPattern != nil {
		t.Errorf("DefaultStroke().DashPattern = %v, want nil", s.DashPattern)
	}
}

func TestStroke_Clone(t *testing.T) {
	original := Stroke{
		Width:       2.0,
		Cap:         LineCapRound,
		Join:        LineJoinRound,
		MiterLimit:  10.0,
		DashPattern: []float64{5, 3, 2, 3},
		DashOffset:  1.5,
	}

	clone := original.Clone()

	// Verify values match
	if clone.Width != original.Width {
		t.Errorf("Clone().Width = %v, want %v", clone.Width, original.Width)
	}
	if clone.Cap != original.Cap {
		t.Errorf("Clone().Cap = %v, want %v", clone.Cap, original.Cap)
	}
	if clone.Join != original.Join {
		t.Errorf("Clone().Join = %v, want %v", clone.Join, original.Join)
	}
	if clone.MiterLimit != original.MiterLimit {
		t.Errorf("Clone().MiterLimit = %v, want %v", clone.MiterLimit, original.MiterLimit)
	}
	if clone.DashOffset != original.DashOffset {
		t.Errorf("Clone().DashOffset = %v, want %v", clone.DashOffset, original.DashOffset)
	}

	// Verify dash pattern is deep copied
	if len(clone.DashPattern) != len(original.DashPattern) {
		t.Fatalf("Clone().DashPattern length = %v, want %v", len(clone.DashPattern), len(original.DashPattern))
	}
	for i := range original.DashPattern {
		if clone.DashPattern[i] != original.DashPattern[i] {
			t.Errorf("Clone().DashPattern[%d] = %v, want %v", i, clone.DashPattern[i], original.DashPattern[i])
		}
	}

	// Verify independence - modify clone shouldn't affect original
	clone.DashPattern[0] = 999
	if original.DashPattern[0] == 999 {
		t.Error("Clone() did not deep copy DashPattern - original was modified")
	}
}

func TestFillPathCommand(t *testing.T) {
	cmd := FillPathCommand{
		Path:  PathRef(1),
		Brush: BrushRef(2),
		Rule:  FillRuleEvenOdd,
	}

	if cmd.Type() != CmdFillPath {
		t.Errorf("FillPathCommand.Type() = %v, want CmdFillPath", cmd.Type())
	}
	if cmd.Path != PathRef(1) {
		t.Errorf("FillPathCommand.Path = %v, want 1", cmd.Path)
	}
	if cmd.Brush != BrushRef(2) {
		t.Errorf("FillPathCommand.Brush = %v, want 2", cmd.Brush)
	}
	if cmd.Rule != FillRuleEvenOdd {
		t.Errorf("FillPathCommand.Rule = %v, want FillRuleEvenOdd", cmd.Rule)
	}
}

func TestStrokePathCommand(t *testing.T) {
	stroke := Stroke{
		Width:       3.0,
		Cap:         LineCapSquare,
		Join:        LineJoinBevel,
		MiterLimit:  8.0,
		DashPattern: []float64{10, 5},
		DashOffset:  2.5,
	}

	cmd := StrokePathCommand{
		Path:   PathRef(3),
		Brush:  BrushRef(4),
		Stroke: stroke,
	}

	if cmd.Type() != CmdStrokePath {
		t.Errorf("StrokePathCommand.Type() = %v, want CmdStrokePath", cmd.Type())
	}
	if cmd.Path != PathRef(3) {
		t.Errorf("StrokePathCommand.Path = %v, want 3", cmd.Path)
	}
	if cmd.Brush != BrushRef(4) {
		t.Errorf("StrokePathCommand.Brush = %v, want 4", cmd.Brush)
	}
	if cmd.Stroke.Width != 3.0 {
		t.Errorf("StrokePathCommand.Stroke.Width = %v, want 3.0", cmd.Stroke.Width)
	}
}

func TestDrawTextCommand(t *testing.T) {
	cmd := DrawTextCommand{
		Text:       "Hello, World!",
		X:          100.5,
		Y:          200.5,
		FontSize:   16.0,
		FontFamily: "Arial",
		Brush:      BrushRef(5),
	}

	if cmd.Type() != CmdDrawText {
		t.Errorf("DrawTextCommand.Type() = %v, want CmdDrawText", cmd.Type())
	}
	if cmd.Text != "Hello, World!" {
		t.Errorf("DrawTextCommand.Text = %v, want Hello, World!", cmd.Text)
	}
	if cmd.X != 100.5 {
		t.Errorf("DrawTextCommand.X = %v, want 100.5", cmd.X)
	}
	if cmd.Y != 200.5 {
		t.Errorf("DrawTextCommand.Y = %v, want 200.5", cmd.Y)
	}
	if cmd.FontSize != 16.0 {
		t.Errorf("DrawTextCommand.FontSize = %v, want 16.0", cmd.FontSize)
	}
	if cmd.FontFamily != "Arial" {
		t.Errorf("DrawTextCommand.FontFamily = %v, want Arial", cmd.FontFamily)
	}
}

func TestDrawImageCommand(t *testing.T) {
	src := image.Rect(0, 0, 50, 50)
	cmd := DrawImageCommand{
		Image: ImageRef(10),
		Opts: render.DrawImageOptions{
			X:             100,
			Y:             100,
			DstWidth:      200,
			DstHeight:     200,
			SrcRect:       &src,
			Interpolation: render.InterpNearest,
			Opacity:       0.8,
		},
	}

	if cmd.Type() != CmdDrawImage {
		t.Errorf("DrawImageCommand.Type() = %v, want CmdDrawImage", cmd.Type())
	}
	if cmd.Image != ImageRef(10) {
		t.Errorf("DrawImageCommand.Image = %v, want 10", cmd.Image)
	}
	if cmd.Opts.SrcRect == nil || cmd.Opts.SrcRect.Dx() != 50 {
		t.Errorf("DrawImageCommand.Opts.SrcRect = %v, want 50 wide", cmd.Opts.SrcRect)
	}
	if cmd.Opts.X != 100 {
		t.Errorf("DrawImageCommand.Opts.X = %v, want 100", cmd.Opts.X)
	}
	if cmd.Opts.Opacity != 0.8 {
		t.Errorf("DrawImageCommand.Opts.Opacity = %v, want 0.8", cmd.Opts.Opacity)
	}
}

// A recording must not alias the caller's option struct: paint code that reuses
// one options value (or edits its SrcRect) after the draw cannot rewrite
// already-recorded commands.
func TestCloneDrawImageOptionsDetachesSrcRect(t *testing.T) {
	src := image.Rect(0, 0, 50, 50)
	cloned := cloneDrawImageOptions(render.DrawImageOptions{SrcRect: &src})
	src.Max = image.Point{9, 9}
	if got := cloned.SrcRect.Dx(); got != 50 {
		t.Errorf("recorded SrcRect changed with the caller's rectangle: width = %d, want 50", got)
	}
}
