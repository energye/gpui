package textinput

import "testing"

func TestR4_PasswordBlocksComposing(t *testing.T) {
	e := New()
	mustSetText(t, e, "hello")
	e.SetPassword(true)
	e.BeginComposing()
	if e.ComposeActive() {
		t.Fatalf("password should block composing")
	}
	e.UpdateComposingText("ni", TextRange{Base: 5, Extent: 7})
	if e.ComposeActive() || e.GetText() != "hello" {
		t.Fatalf("password update should be blocked")
	}
	e.SetPassword(false)
	e.BeginComposing()
	if !e.ComposeActive() {
		t.Fatalf("should allow after password off")
	}
}

func TestR4_ReadOnlyBlocksEdit(t *testing.T) {
	e := New()
	mustSetText(t, e, "hello")
	e.SetReadOnly(true)
	e.AddText("x")
	if e.GetText() != "hello" {
		t.Fatalf("readonly should block AddText")
	}
	e.SetReadOnly(false)
	e.AddText("x")
	if e.GetText() != "hellox" {
		t.Fatalf("should allow after readonly off")
	}
}

func TestR4_SelectWord(t *testing.T) {
	e := New()
	mustSetText(t, e, "hello world")
	if !e.SelectWordAt(1) {
		t.Fatalf("select word failed")
	}
	// selection check via byte offsets
	_ = e.GetText()
	// Check selection is hello
	selText := e.GetText()[byteOffsetForUtf16(e.GetText(), e.selection.Start()):byteOffsetForUtf16(e.GetText(), e.selection.End())]
	if selText != "hello" {
		t.Fatalf("select word got %q", selText)
	}
}

func TestR4_PasswordCustomChar(t *testing.T) {
	e := New()
	mustSetText(t, e, "hello")
	e.SetPassword(true)
	// default is '•' per Flutter
	if e.ObscuringCharacter() != '•' {
		t.Fatalf("default obscuring char = %q want '•'", e.ObscuringCharacter())
	}
	e.SetSelection(TextRange{Base: 0, Extent: 5})
	if got := e.Copy(); got != "•••••" {
		t.Fatalf("default masked copy = %q want •••••", got)
	}
	e.SetObscuringCharacter('*')
	if e.ObscuringCharacter() != '*' {
		t.Fatalf("custom char failed")
	}
	if got := e.Copy(); got != "*****" {
		t.Fatalf("custom masked copy = %q want *****", got)
	}
	e.SetObscuringCharacter('●')
	if got := e.Copy(); got != "●●●●●" {
		t.Fatalf("custom ● copy = %q", got)
	}
	// reset to default
	e.SetObscuringCharacter(0)
	if e.ObscuringCharacter() != '•' {
		t.Fatalf("reset to default failed")
	}
}
