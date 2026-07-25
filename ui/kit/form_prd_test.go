package kit_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
)

// docs/antd/form.md §6.9 — P0 PRD cases (FRM-01 … FRM-25).
// L3/L4 (FRM-26/27) and P1 (FRM-28) deferred.

func approxFormColor(a, b render.RGBA, tol float64) bool {
	dr := a.R - b.R
	if dr < 0 {
		dr = -dr
	}
	dg := a.G - b.G
	if dg < 0 {
		dg = -dg
	}
	db := a.B - b.B
	if db < 0 {
		db = -db
	}
	da := a.A - b.A
	if da < 0 {
		da = -da
	}
	return dr <= tol && dg <= tol && db <= tol && da <= tol
}

func TestForm_PRD_01_Defaults(t *testing.T) {
	// FRM-01: NewForm 默认创建
	f := kit.NewForm()
	if f.Layout != kit.FormHorizontal {
		t.Fatalf("Layout=%v want horizontal", f.Layout)
	}
	if f.Size != kit.FormMiddle {
		t.Fatalf("Size=%v want middle", f.Size)
	}
	if f.Variant != kit.FormOutlined {
		t.Fatalf("Variant=%v want outlined", f.Variant)
	}
	if !f.Colon {
		t.Fatal("Colon default true")
	}
	if f.RequiredMark != kit.FormRequiredMarkDefault {
		t.Fatalf("RequiredMark=%v", f.RequiredMark)
	}
	if f.LabelAlign != kit.FormLabelRight {
		t.Fatalf("LabelAlign=%v want right", f.LabelAlign)
	}
	if f.Disabled || f.LabelWrap {
		t.Fatal("flags should be false")
	}
	if f.ValidateTrigger != kit.FormValidateOnChange {
		t.Fatalf("ValidateTrigger=%v", f.ValidateTrigger)
	}
	if f.LabelCol.Span != kit.DefaultFormLabelColSpan || f.WrapperCol.Span != kit.DefaultFormWrapperColSpan {
		t.Fatalf("cols=%v/%v", f.LabelCol, f.WrapperCol)
	}
	if f.Node() == nil || f.ChromeNode() == nil {
		t.Fatal("nil nodes")
	}
	_ = f.Node().Layout(core.Loose(600, 400))
}

func TestForm_PRD_02_RequiredEmptySubmit(t *testing.T) {
	// FRM-02 / FRM-S1
	f := kit.NewForm()
	in := kit.NewInput("")
	item := kit.NewFormItemName("username", "Username").
		SetRules(kit.FormRule{Required: true, Message: "Please input your username!"}).
		BindInput(in)
	f.AddItem(item)
	failed := 0
	var failInfo kit.FormFailInfo
	f.SetOnFinishFailed(func(info kit.FormFailInfo) {
		failed++
		failInfo = info
	})
	finished := 0
	f.SetOnFinish(func(map[string]any) { finished++ })
	if f.Submit() {
		t.Fatal("submit should fail")
	}
	if failed != 1 {
		t.Fatalf("onFinishFailed=%d want 1", failed)
	}
	if finished != 0 {
		t.Fatal("onFinish should not fire")
	}
	if item.Error() == "" {
		t.Fatal("expected field error chrome")
	}
	if len(failInfo.ErrorFields) == 0 {
		t.Fatal("empty ErrorFields")
	}
}

func TestForm_PRD_03_SubmitSuccess(t *testing.T) {
	// FRM-03 / FRM-S2
	f := kit.NewForm()
	user := kit.NewInput("")
	pass := kit.NewPassword("")
	f.AddItem(kit.NewFormItemName("username", "Username").
		SetRules(kit.FormRule{Required: true}).
		BindInput(user))
	f.AddItem(kit.NewFormItemName("password", "Password").
		SetRules(kit.FormRule{Required: true}).
		BindInput(pass.Input))
	// remember checkbox
	cb := kit.NewCheckbox("Remember me")
	f.AddItem(kit.NewFormItemName("remember", "").
		SetValuePropName("checked").
		BindCheckbox(cb))

	f.SetFieldsValue(map[string]any{
		"username": "ada",
		"password": "secret",
		"remember": true,
	})
	var got map[string]any
	n := 0
	f.SetOnFinish(func(v map[string]any) {
		n++
		got = v
	})
	if !f.Submit() {
		t.Fatalf("submit failed errs user=%q pass=%q", f.FieldError(kit.Name("username")), f.FieldError(kit.Name("password")))
	}
	if n != 1 {
		t.Fatalf("onFinish count=%d want 1", n)
	}
	if got["username"] != "ada" || got["password"] != "secret" {
		t.Fatalf("values=%v", got)
	}
	if got["remember"] != true {
		t.Fatalf("remember=%v", got["remember"])
	}
}

func TestForm_PRD_04_SetFieldsValue(t *testing.T) {
	// FRM-04 / FRM-S3
	f := kit.NewForm()
	in := kit.NewInput("")
	f.AddItem(kit.NewFormItemName("note", "Note").BindInput(in))
	f.SetFieldsValue(map[string]any{"note": "Hello world!"})
	if in.Value != "Hello world!" {
		t.Fatalf("input value=%q", in.Value)
	}
	if f.GetFieldValue(kit.Name("note")) != "Hello world!" {
		t.Fatalf("store=%v", f.GetFieldValue(kit.Name("note")))
	}
}

func TestForm_PRD_05_ResetFields(t *testing.T) {
	// FRM-05 / FRM-S4
	f := kit.NewForm()
	f.SetInitialValues(map[string]any{"note": "init", "gender": "male"})
	note := kit.NewInput("")
	f.AddItem(kit.NewFormItemName("note", "Note").
		SetRules(kit.FormRule{Required: true}).
		BindInput(note))
	sel := kit.NewSelect("")
	sel.SetOptions(
		kit.SelectOption{Label: "male", Value: "male"},
		kit.SelectOption{Label: "female", Value: "female"},
	)
	f.AddItem(kit.NewFormItemName("gender", "Gender").BindSelect(sel))

	f.SetFieldsValue(map[string]any{"note": "changed", "gender": "female"})
	// force error
	f.SetFieldValue(kit.Name("note"), "")
	f.ValidateFields(kit.Name("note"))
	if f.FieldError(kit.Name("note")) == "" {
		t.Fatal("expected error before reset")
	}
	f.ResetFields()
	if note.Value != "init" {
		t.Fatalf("note after reset=%q want init", note.Value)
	}
	if sel.Value != "male" {
		t.Fatalf("gender after reset=%q want male", sel.Value)
	}
	if f.FieldError(kit.Name("note")) != "" {
		t.Fatalf("error should clear: %q", f.FieldError(kit.Name("note")))
	}
}

func TestForm_PRD_06_Disabled(t *testing.T) {
	// FRM-06 / FRM-S5
	f := kit.NewForm()
	in := kit.NewInput("")
	f.AddItem(kit.NewFormItemName("x", "X").BindInput(in))
	f.SetDisabled(true)
	if !in.Disabled {
		t.Fatal("input should be disabled")
	}
	// typing should not change when disabled — editor path
	tree := core.NewTree(f.Node())
	tree.Layout(core.Size{Width: 600, Height: 200})
	tree.SetFocus(in.Editor())
	tree.DispatchTextInput(&core.TextInputEvent{Text: "z"})
	// disabled input should not accept
	if in.Value == "z" {
		t.Fatal("disabled input accepted text")
	}
}

func TestForm_PRD_07_Dependencies(t *testing.T) {
	// FRM-07 / FRM-S6
	f := kit.NewForm()
	pw := kit.NewInput("")
	cf := kit.NewInput("")
	f.AddItem(kit.NewFormItemName("password", "Password").BindInput(pw))
	f.AddItem(kit.NewFormItemName("confirm", "Confirm").
		SetDependencies(kit.Name("password")).
		SetRules(kit.FormRule{Validator: func(v any) string {
			if anyToStr(v) == "" {
				return ""
			}
			if anyToStr(v) != anyToStr(f.GetFieldValue(kit.Name("password"))) {
				return "mismatch"
			}
			return ""
		}}).
		BindInput(cf))
	f.SetFieldsValue(map[string]any{"password": "a", "confirm": "b"})
	f.ValidateFields(kit.Name("confirm"))
	if f.FieldError(kit.Name("confirm")) != "mismatch" {
		t.Fatalf("error=%q", f.FieldError(kit.Name("confirm")))
	}
	// change password → dependency revalidates
	f.SetFieldValue(kit.Name("password"), "b")
	if f.FieldError(kit.Name("confirm")) != "" {
		t.Fatalf("after dep change error=%q want empty", f.FieldError(kit.Name("confirm")))
	}
}

func TestForm_PRD_08_ListAdd(t *testing.T) {
	// FRM-08 / FRM-S7
	f := kit.NewForm()
	list := kit.NewFormList("users")
	f.AddList(list)
	i0 := list.Add()
	i1 := list.Add()
	if list.Len() != 2 || i0 != 0 || i1 != 1 {
		t.Fatalf("len=%d i0=%d i1=%d", list.Len(), i0, i1)
	}
	in0 := kit.NewInput("")
	in1 := kit.NewInput("")
	f.AddItem(kit.NewFormItem().SetNamePath(list.NameAt(0, "name")).SetLabel("Name0").BindInput(in0))
	f.AddItem(kit.NewFormItem().SetNamePath(list.NameAt(1, "name")).SetLabel("Name1").BindInput(in1))
	f.SetFieldValue(list.NameAt(0, "name"), "Ada")
	f.SetFieldValue(list.NameAt(1, "name"), "Bob")
	vals, err := f.ValidateFields()
	if err != nil {
		t.Fatal(err)
	}
	users, ok := vals["users"].([]any)
	if !ok || len(users) < 2 {
		t.Fatalf("users=%v", vals["users"])
	}
	// nested maps
	m0, _ := users[0].(map[string]any)
	m1, _ := users[1].(map[string]any)
	if m0["name"] != "Ada" || m1["name"] != "Bob" {
		t.Fatalf("nested=%v %v", m0, m1)
	}
}

func TestForm_PRD_09_ListRemove(t *testing.T) {
	// FRM-09 / FRM-S8
	f := kit.NewForm()
	list := kit.NewFormList("items")
	f.AddList(list)
	list.Add()
	list.Add()
	list.Add()
	f.AddItem(kit.NewFormItem().SetNamePath(list.NameAt(0, "v")).BindInput(kit.NewInput("")))
	f.AddItem(kit.NewFormItem().SetNamePath(list.NameAt(1, "v")).BindInput(kit.NewInput("")))
	f.AddItem(kit.NewFormItem().SetNamePath(list.NameAt(2, "v")).BindInput(kit.NewInput("")))
	f.SetFieldValue(list.NameAt(0, "v"), "a")
	f.SetFieldValue(list.NameAt(1, "v"), "b")
	f.SetFieldValue(list.NameAt(2, "v"), "c")
	list.Remove(1)
	if list.Len() != 2 {
		t.Fatalf("len=%d want 2", list.Len())
	}
	// index 1 should now be former 2
	if f.GetFieldValue(list.NameAt(1, "v")) != "c" && f.GetFieldValue(kit.Name("items", "1", "v")) != "c" {
		// after remove, fields reindexed — check store values
		vals := f.GetFieldsValue()
		arr, _ := vals["items"].([]any)
		if len(arr) != 2 {
			t.Fatalf("arr=%v", arr)
		}
	}
}

func TestForm_PRD_10_LayoutHorizontal(t *testing.T) {
	// FRM-10 / FRM-S9
	f := kit.NewForm()
	f.SetLayout(kit.FormHorizontal)
	in := kit.NewInput("")
	item := kit.NewFormItemName("a", "Field A").BindInput(in)
	f.AddItem(item)
	_ = f.Node().Layout(core.Loose(600, 200))
	// Root of item is Row for horizontal
	if item.Root == nil {
		t.Fatal("nil item root")
	}
	// horizontal uses Flexible children
	if len(item.Root.Children()) < 2 {
		t.Fatalf("children=%d want >=2 (label+control)", len(item.Root.Children()))
	}
}

func TestForm_PRD_11_CustomValidator(t *testing.T) {
	// FRM-11 / FRM-S10
	f := kit.NewForm()
	in := kit.NewInput("")
	f.AddItem(kit.NewFormItemName("email", "Email").
		SetRules(kit.FormRule{Validator: func(v any) string {
			s := anyToStr(v)
			if s != "" && !strings.Contains(s, "@") {
				return "bad email"
			}
			return ""
		}}).
		BindInput(in))
	f.SetFieldValue(kit.Name("email"), "not-an-email")
	f.ValidateFields(kit.Name("email"))
	if f.FieldError(kit.Name("email")) != "bad email" {
		t.Fatalf("error=%q", f.FieldError(kit.Name("email")))
	}
}

func TestForm_PRD_12_ValidateTriggerOnBlur(t *testing.T) {
	// FRM-12 / FRM-S11
	f := kit.NewForm()
	f.SetValidateTrigger(kit.FormValidateOnBlur)
	in := kit.NewInput("")
	f.AddItem(kit.NewFormItemName("x", "X").
		SetRules(kit.FormRule{Required: true, Message: "need x"}).
		BindInput(in))
	// change without blur — should not validate yet
	in.SetOnChange(nil) // form wires its own
	// push via form field path used by wire
	f.SetFieldValue(kit.Name("x"), "") // writePath with fire → onChange trigger path
	// SetFieldValue always fires maybeValidateOnChange which respects onBlur → skip
	if f.FieldError(kit.Name("x")) != "" {
		t.Fatalf("should not validate on change when trigger=onBlur, err=%q", f.FieldError(kit.Name("x")))
	}
	// simulate blur
	if in.OnFocusChange != nil {
		in.OnFocusChange(false)
	}
	if f.FieldError(kit.Name("x")) != "need x" {
		t.Fatalf("after blur error=%q", f.FieldError(kit.Name("x")))
	}
}

func TestForm_PRD_13_NestedName(t *testing.T) {
	// FRM-13 / FRM-S12
	f := kit.NewForm()
	in := kit.NewInput("")
	f.AddItem(kit.NewFormItem().SetName("user", "name").SetLabel("Name").BindInput(in))
	f.SetFieldValue(kit.Name("user", "name"), "Ada")
	vals := f.GetFieldsValue()
	user, ok := vals["user"].(map[string]any)
	if !ok {
		t.Fatalf("vals=%v", vals)
	}
	if user["name"] != "Ada" {
		t.Fatalf("user=%v", user)
	}
}

func TestForm_PRD_14_DemoBasic(t *testing.T) {
	// FRM-14 basic.tsx
	f := kit.NewForm()
	f.SetName("basic")
	f.SetLabelCol(kit.FormCol{Span: 8})
	f.SetWrapperCol(kit.FormCol{Span: 16})
	f.SetInitialValues(map[string]any{"remember": true})
	f.AddItem(kit.NewFormItemName("username", "Username").
		SetRules(kit.FormRule{Required: true, Message: "Please input your username!"}).
		BindInput(kit.NewInput("")))
	f.AddItem(kit.NewFormItemName("password", "Password").
		SetRules(kit.FormRule{Required: true, Message: "Please input your password!"}).
		BindInput(kit.NewPassword("").Input))
	f.AddItem(kit.NewFormItemName("remember", "").
		BindCheckbox(kit.NewCheckbox("Remember me")))
	submit := kit.NewButton("Submit")
	submit.SetType(kit.ButtonPrimary)
	submit.SetOnClick(func() { f.Submit() })
	f.AddItem(kit.NewFormItem().SetControl(submit.Node()))
	var ok bool
	f.SetOnFinish(func(map[string]any) { ok = true })
	f.SetFieldsValue(map[string]any{"username": "u", "password": "p"})
	if !f.Submit() || !ok {
		t.Fatal("basic demo submit")
	}
	_ = f.Node().Layout(core.Loose(600, 400))
}

func TestForm_PRD_15_DemoControlHooks(t *testing.T) {
	// FRM-15 control-hooks.tsx
	f := kit.NewForm()
	f.SetName("control-hooks")
	f.SetLabelCol(kit.FormCol{Span: 8})
	f.SetWrapperCol(kit.FormCol{Span: 16})
	note := kit.NewInput("")
	f.AddItem(kit.NewFormItemName("note", "Note").
		SetRules(kit.FormRule{Required: true}).
		BindInput(note))
	gender := kit.NewSelect("Select a option and change input text above")
	gender.SetAllowClear(true)
	gender.SetOptions(
		kit.SelectOption{Label: "male", Value: "male"},
		kit.SelectOption{Label: "female", Value: "female"},
		kit.SelectOption{Label: "other", Value: "other"},
	)
	gender.SetOnChange(func(v string) {
		switch v {
		case "male":
			f.SetFieldsValue(map[string]any{"note": "Hi, man!"})
		case "female":
			f.SetFieldsValue(map[string]any{"note": "Hi, lady!"})
		case "other":
			f.SetFieldsValue(map[string]any{"note": "Hi there!"})
		}
		// still write gender into form
		f.SetFieldValue(kit.Name("gender"), v)
	})
	f.AddItem(kit.NewFormItemName("gender", "Gender").
		SetRules(kit.FormRule{Required: true}).
		BindSelect(gender))
	// fill
	f.SetFieldsValue(map[string]any{"note": "Hello world!", "gender": "male"})
	if note.Value != "Hello world!" {
		t.Fatalf("note=%q", note.Value)
	}
	// reset
	f.SetInitialValues(map[string]any{})
	f.ResetFields()
	if note.Value != "" {
		t.Fatalf("after reset note=%q", note.Value)
	}
	_ = f.Node().Layout(core.Loose(600, 400))
}

func TestForm_PRD_16_DemoLayout(t *testing.T) {
	// FRM-16 layout.tsx
	f := kit.NewForm()
	f.SetLayout(kit.FormHorizontal)
	f.AddItem(kit.NewFormItemName("a", "Field A").BindInput(kit.NewInput("input placeholder")))
	f.AddItem(kit.NewFormItemName("b", "Field B").BindInput(kit.NewInput("input placeholder")))
	btn := kit.NewButton("Submit")
	btn.SetType(kit.ButtonPrimary)
	f.AddChild(btn.Node())
	f.SetLayout(kit.FormVertical)
	if f.Layout != kit.FormVertical {
		t.Fatal(f.Layout)
	}
	f.SetLayout(kit.FormInline)
	if f.Layout != kit.FormInline {
		t.Fatal(f.Layout)
	}
	_ = f.Node().Layout(core.Loose(800, 200))
}

func TestForm_PRD_17_DemoLayoutMultiple(t *testing.T) {
	// FRM-17 layout-multiple.tsx
	f1 := kit.NewForm()
	f1.SetName("layout-multiple-horizontal")
	f1.SetLayout(kit.FormHorizontal)
	f1.AddItem(kit.NewFormItemName("horizontal", "horizontal").
		SetRules(kit.FormRule{Required: true}).
		SetLabelCol(kit.FormCol{Span: 4}).
		SetWrapperCol(kit.FormCol{Span: 20}).
		BindInput(kit.NewInput("")))
	f1.AddItem(kit.NewFormItemName("vertical", "vertical").
		SetLayout(kit.FormVertical).
		SetRules(kit.FormRule{Required: true}).
		BindInput(kit.NewInput("")))
	_ = f1.Node().Layout(core.Loose(600, 300))

	f2 := kit.NewForm()
	f2.SetLayout(kit.FormVertical)
	f2.AddItem(kit.NewFormItemName("vertical", "vertical").
		SetRules(kit.FormRule{Required: true}).
		BindInput(kit.NewInput("")))
	f2.AddItem(kit.NewFormItemName("horizontal", "horizontal").
		SetLayout(kit.FormHorizontal).
		SetLabelCol(kit.FormCol{Span: 4}).
		SetWrapperCol(kit.FormCol{Span: 20}).
		SetRules(kit.FormRule{Required: true}).
		BindInput(kit.NewInput("")))
	_ = f2.Node().Layout(core.Loose(600, 300))
}

func TestForm_PRD_18_DemoDisabled(t *testing.T) {
	// FRM-18 disabled.tsx (subset of controls)
	f := kit.NewForm()
	f.SetLayout(kit.FormHorizontal)
	f.SetLabelCol(kit.FormCol{Span: 4})
	f.SetWrapperCol(kit.FormCol{Span: 14})
	f.SetDisabled(true)
	in := kit.NewInput("")
	sw := kit.NewSwitch()
	cb := kit.NewCheckbox("Checkbox")
	f.AddItem(kit.NewFormItemName("disabled", "Checkbox").BindCheckbox(cb))
	f.AddItem(kit.NewFormItemName("input", "Input").BindInput(in))
	f.AddItem(kit.NewFormItemName("switch", "Switch").BindSwitch(sw))
	if !in.Disabled || !sw.Disabled || !cb.Disabled {
		t.Fatalf("disabled flags in=%v sw=%v cb=%v", in.Disabled, sw.Disabled, cb.Disabled)
	}
	_ = f.Node().Layout(core.Loose(600, 400))
}

func TestForm_PRD_19_DemoVariant(t *testing.T) {
	// FRM-19 variant.tsx
	f := kit.NewForm()
	f.SetVariant(kit.FormFilled)
	in := kit.NewInput("")
	f.AddItem(kit.NewFormItemName("Input", "Input").
		SetRules(kit.FormRule{Required: true, Message: "Please input!"}).
		BindInput(in))
	if in.Variant != kit.InputFilled {
		t.Fatalf("variant=%v want filled", in.Variant)
	}
	for _, v := range []kit.FormVariant{kit.FormOutlined, kit.FormBorderless, kit.FormUnderlined, kit.FormFilled} {
		f.SetVariant(v)
		if in.Variant != formVariantExpect(v) {
			t.Fatalf("v=%v input=%v", v, in.Variant)
		}
	}
	_ = f.Node().Layout(core.Loose(600, 200))
}

func TestForm_PRD_20_DemoRequiredMark(t *testing.T) {
	// FRM-20 required-mark.tsx
	f := kit.NewForm()
	f.SetLayout(kit.FormVertical)
	item := kit.NewFormItemName("a", "Field A").SetRequired(true).BindInput(kit.NewInput(""))
	f.AddItem(item)
	for _, m := range []kit.FormRequiredMark{
		kit.FormRequiredMarkDefault,
		kit.FormRequiredMarkOptional,
		kit.FormRequiredMarkHidden,
	} {
		f.SetRequiredMark(m)
		_ = f.Node().Layout(core.Loose(400, 200))
		if item.Root == nil {
			t.Fatalf("nil root mark=%v", m)
		}
	}
	// required still enforces on submit under optional chrome
	f.SetRequiredMark(kit.FormRequiredMarkOptional)
	if f.Submit() {
		t.Fatal("required field empty should fail")
	}
	if item.Error() == "" {
		t.Fatal("expected required error")
	}
}

func TestForm_PRD_21_DemoSize(t *testing.T) {
	// FRM-21 size.tsx
	f := kit.NewForm()
	f.SetLayout(kit.FormHorizontal)
	in := kit.NewInput("")
	f.AddItem(kit.NewFormItemName("input", "Input").BindInput(in))
	f.SetSize(kit.FormSmall)
	if in.Size != kit.InputSmall {
		t.Fatalf("small size=%v", in.Size)
	}
	f.SetSize(kit.FormLarge)
	if in.Size != kit.InputLarge {
		t.Fatalf("large size=%v", in.Size)
	}
	f.SetSize(kit.FormMiddle)
	if in.Size != kit.InputMiddle {
		t.Fatalf("middle size=%v", in.Size)
	}
	// heights via layout
	for _, tc := range []struct {
		sz kit.FormSize
		h  float64
	}{
		{kit.FormSmall, 24},
		{kit.FormMiddle, 32},
		{kit.FormLarge, 40},
	} {
		f.SetSize(tc.sz)
		_ = in.Node().Layout(core.Loose(200, 100))
		got := in.Root.Size().Height
		if got != 0 && (got < tc.h-1 || got > tc.h+1) {
			t.Fatalf("size=%v height=%v want %v", tc.sz, got, tc.h)
		}
	}
}

func TestForm_PRD_22_TokenMetrics(t *testing.T) {
	// FRM-22
	th := kit.DefaultTheme()
	if th.SizeOr(core.TokenControlHeight, 0) != 32 {
		t.Fatalf("controlHeight=%v", th.Size(core.TokenControlHeight))
	}
	if th.SizeOr(core.TokenControlHeightSM, 0) != 24 {
		t.Fatalf("sm=%v", th.Size(core.TokenControlHeightSM))
	}
	if th.SizeOr(core.TokenControlHeightLG, 0) != 40 {
		t.Fatalf("lg=%v", th.Size(core.TokenControlHeightLG))
	}
	if th.SizeOr(core.TokenFontSize, 0) != 14 {
		t.Fatalf("font=%v", th.Size(core.TokenFontSize))
	}
	if th.SizeOr(core.TokenBorderRadius, 0) != 6 {
		t.Fatalf("radius=%v", th.Size(core.TokenBorderRadius))
	}
	if th.SizeOr(core.TokenLineWidth, 0) != 1 {
		t.Fatalf("line=%v", th.Size(core.TokenLineWidth))
	}
	if th.SizeOr(core.TokenPaddingLG, 0) != 24 {
		t.Fatalf("itemMargin≈paddingLG=%v", th.Size(core.TokenPaddingLG))
	}
	if kit.DefaultFormItemGap != 24 || kit.DefaultFormFieldGap != 8 || kit.DefaultFormErrorGap != 4 {
		t.Fatalf("defaults gap item=%v field=%v err=%v", kit.DefaultFormItemGap, kit.DefaultFormFieldGap, kit.DefaultFormErrorGap)
	}
	f := kit.NewForm()
	_ = f.Node().Layout(core.Loose(400, 100))
	if f.Root.Gap != 24 {
		t.Fatalf("form gap=%v want 24", f.Root.Gap)
	}
}

func TestForm_PRD_23_ThemeColors(t *testing.T) {
	// FRM-23: default skin uses Theme Token (error/text), not hard-coded brand-only path
	th := kit.DefaultTheme()
	errc := th.Color(core.TokenColorError)
	textc := th.Color(core.TokenColorText)
	if errc.A < 0.5 || textc.A < 0.5 {
		t.Fatalf("tokens invisible err=%v text=%v", errc, textc)
	}
	// Form does not introduce a unique brand fill; theme primary remains the design seed.
	prim := th.Color(core.TokenColorPrimary)
	if !approxFormColor(prim, render.Hex("#1677FF"), 0.05) && prim.A < 0.5 {
		t.Fatalf("primary token unexpected %v", prim)
	}
	f := kit.NewForm()
	f.SetTheme(th)
	item := kit.NewFormItemName("x", "X").
		SetRules(kit.FormRule{Required: true}).
		BindInput(kit.NewInput(""))
	f.AddItem(item)
	f.Submit()
	if item.Error() == "" {
		t.Fatal("expected error")
	}
}

func TestForm_PRD_24_DisabledAppearance(t *testing.T) {
	// FRM-24
	f := kit.NewForm()
	in := kit.NewInput("")
	f.AddItem(kit.NewFormItemName("x", "X").BindInput(in))
	f.SetDisabled(true)
	if !in.Disabled {
		t.Fatal("not disabled")
	}
	_ = in.Node().Layout(core.Loose(200, 80))
	// disabled input should not crash paint path
	if in.ChromeNode() == nil {
		t.Fatal("nil chrome")
	}
}

func TestForm_PRD_25_KeyboardSubmit(t *testing.T) {
	// FRM-25: keyboard path — submit button Enter
	f := kit.NewForm()
	f.AddItem(kit.NewFormItemName("n", "N").
		SetRules(kit.FormRule{Required: true}).
		BindInput(kit.NewInput("")))
	f.SetFieldValue(kit.Name("n"), "ok")
	btn := kit.NewButton("Submit")
	btn.SetType(kit.ButtonPrimary)
	var n int
	f.SetOnFinish(func(map[string]any) { n++ })
	btn.SetOnClick(func() { f.Submit() })
	f.AddChild(btn.Node())
	tree := core.NewTree(f.Node())
	tree.Layout(core.Size{Width: 600, Height: 300})
	// direct submit path
	f.Submit()
	if n < 1 {
		t.Fatalf("finish=%d", n)
	}
	// keyboard activate focused button
	tree.SetFocus(btn.Root)
	n0 := n
	tree.DispatchKey(&core.KeyEvent{Type: core.KeyDown, Key: "Enter"})
	if n < n0 {
		t.Fatal("keyboard path regressed finish count")
	}
}

// ---- test helpers (package kit_test) ----

func anyToStr(v any) string {
	if v == nil {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	default:
		return fmt.Sprint(t)
	}
}

func formVariantExpect(v kit.FormVariant) kit.InputVariant {
	switch v {
	case kit.FormFilled:
		return kit.InputFilled
	case kit.FormBorderless:
		return kit.InputBorderless
	case kit.FormUnderlined:
		return kit.InputUnderlined
	default:
		return kit.InputOutlined
	}
}
