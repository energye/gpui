//go:build linux && !nogpu

package main

import (
	"fmt"

	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/primitive"
)

func (c *catalogCtx) registerForm() {
	// Form — docs/antd/form.md §6.8 P0
	// https://ant.design/components/form
	// demos: basic / control-hooks / layout / layout-multiple / disabled / variant / required-mark / size
	face, th := c.face, c.theme
	status := c.status

	wireFace := func(f *kit.Form) *kit.Form {
		f.SetFace(face)
		if th != nil {
			f.SetTheme(th)
		}
		return f
	}

	// ---------- basic.tsx ----------
	basic := wireFace(kit.NewForm())
	basic.SetName("basic")
	basic.SetLabelCol(kit.FormCol{Span: 8})
	basic.SetWrapperCol(kit.FormCol{Span: 16})
	basic.SetInitialValues(map[string]any{"remember": true})
	basic.SetOnFinish(func(v map[string]any) { *status = fmt.Sprintf("basic ok %v", v) })
	basic.SetOnFinishFailed(func(info kit.FormFailInfo) {
		*status = fmt.Sprintf("basic fail %d fields", len(info.ErrorFields))
	})
	basic.AddItem(kit.NewFormItemName("username", "Username").
		SetRules(kit.FormRule{Required: true, Message: "Please input your username!"}).
		BindInput(kit.NewInput("")))
	basic.AddItem(kit.NewFormItemName("password", "Password").
		SetRules(kit.FormRule{Required: true, Message: "Please input your password!"}).
		BindInput(kit.NewPassword("").Input))
	basic.AddItem(kit.NewFormItemName("remember", "").
		BindCheckbox(kit.NewCheckbox("Remember me")))
	basicSubmit := c.trackBtn(kit.NewButton("Submit"))
	basicSubmit.SetType(kit.ButtonPrimary)
	basicSubmit.SetOnClick(func() { basic.Submit() })
	basic.AddItem(kit.NewFormItem().SetControl(basicSubmit.Node()))
	secBasic := demoSection(face, th, "Basic",
		"基本使用 — username/password/remember + onFinish.",
		basic.Node())

	// ---------- control-hooks.tsx ----------
	hooks := wireFace(kit.NewForm())
	hooks.SetName("control-hooks")
	hooks.SetLabelCol(kit.FormCol{Span: 8})
	hooks.SetWrapperCol(kit.FormCol{Span: 16})
	hooks.SetOnFinish(func(v map[string]any) { *status = fmt.Sprintf("hooks %v", v) })
	noteIn := kit.NewInput("")
	hooks.AddItem(kit.NewFormItemName("note", "Note").
		SetRules(kit.FormRule{Required: true}).
		BindInput(noteIn))
	gender := kit.NewSelect("Select a option and change input text above")
	gender.SetAllowClear(true)
	gender.SetOptions(
		kit.SelectOption{Label: "male", Value: "male"},
		kit.SelectOption{Label: "female", Value: "female"},
		kit.SelectOption{Label: "other", Value: "other"},
	)
	hooks.AddItem(kit.NewFormItemName("gender", "Gender").
		SetRules(kit.FormRule{Required: true}).
		BindSelect(gender))
	// gender → note side-effect (antd control-hooks onGenderChange)
	hooks.SetOnValuesChange(func(changed, all map[string]any) {
		g, ok := changed["gender"]
		if !ok {
			return
		}
		switch g {
		case "male":
			if all["note"] != "Hi, man!" {
				hooks.SetFieldValue(kit.Name("note"), "Hi, man!")
			}
		case "female":
			if all["note"] != "Hi, lady!" {
				hooks.SetFieldValue(kit.Name("note"), "Hi, lady!")
			}
		case "other":
			if all["note"] != "Hi there!" {
				hooks.SetFieldValue(kit.Name("note"), "Hi there!")
			}
		}
	})
	btnSubmit := c.trackBtn(kit.NewButton("Submit"))
	btnSubmit.SetType(kit.ButtonPrimary)
	btnSubmit.SetOnClick(func() { hooks.Submit() })
	btnReset := c.trackBtn(kit.NewButton("Reset"))
	btnReset.SetOnClick(func() { hooks.ResetFields() })
	btnFill := c.trackBtn(kit.NewButton("Fill form"))
	btnFill.SetType(kit.ButtonLink)
	btnFill.SetOnClick(func() {
		hooks.SetFieldsValue(map[string]any{"note": "Hello world!", "gender": "male"})
	})
	hooks.AddChild(spaceWrap(8, btnSubmit.Node(), btnReset.Node(), btnFill.Node()))
	secHooks := demoSection(face, th, "Form methods",
		"表单方法调用 — setFieldsValue / resetFields / submit.",
		hooks.Node())

	// ---------- layout.tsx ----------
	layoutForm := wireFace(kit.NewForm())
	layoutForm.SetLayout(kit.FormHorizontal)
	layoutForm.AddItem(kit.NewFormItemName("a", "Field A").BindInput(kit.NewInput("input placeholder")))
	layoutForm.AddItem(kit.NewFormItemName("b", "Field B").BindInput(kit.NewInput("input placeholder")))
	layoutSubmit := c.trackBtn(kit.NewButton("Submit"))
	layoutSubmit.SetType(kit.ButtonPrimary)
	layoutForm.AddChild(layoutSubmit.Node())
	btnH := c.trackBtn(kit.NewButton("Horizontal"))
	btnV := c.trackBtn(kit.NewButton("Vertical"))
	btnI := c.trackBtn(kit.NewButton("Inline"))
	btnH.SetOnClick(func() { layoutForm.SetLayout(kit.FormHorizontal); *status = "form layout=horizontal" })
	btnV.SetOnClick(func() { layoutForm.SetLayout(kit.FormVertical); *status = "form layout=vertical" })
	btnI.SetOnClick(func() { layoutForm.SetLayout(kit.FormInline); *status = "form layout=inline" })
	secLayout := demoSection(face, th, "Form layout",
		"表单布局 — horizontal / vertical / inline.",
		colForm(spaceWrap(8, btnH.Node(), btnV.Node(), btnI.Node()), layoutForm.Node()))

	// ---------- layout-multiple.tsx ----------
	mixH := wireFace(kit.NewForm())
	mixH.SetName("layout-multiple-horizontal")
	mixH.SetLayout(kit.FormHorizontal)
	mixH.AddItem(kit.NewFormItemName("horizontal", "horizontal").
		SetRules(kit.FormRule{Required: true}).
		SetLabelCol(kit.FormCol{Span: 4}).
		SetWrapperCol(kit.FormCol{Span: 20}).
		BindInput(kit.NewInput("")))
	mixH.AddItem(kit.NewFormItemName("vertical", "vertical").
		SetLayout(kit.FormVertical).
		SetRules(kit.FormRule{Required: true}).
		BindInput(kit.NewInput("")))
	mixV := wireFace(kit.NewForm())
	mixV.SetName("layout-multiple-vertical")
	mixV.SetLayout(kit.FormVertical)
	mixV.AddItem(kit.NewFormItemName("vertical", "vertical").
		SetRules(kit.FormRule{Required: true}).
		BindInput(kit.NewInput("")))
	mixV.AddItem(kit.NewFormItemName("horizontal", "horizontal").
		SetLayout(kit.FormHorizontal).
		SetLabelCol(kit.FormCol{Span: 4}).
		SetWrapperCol(kit.FormCol{Span: 20}).
		SetRules(kit.FormRule{Required: true}).
		BindInput(kit.NewInput("")))
	secMix := demoSection(face, th, "Mixed layout",
		"表单混合布局 — item layout overrides form layout.",
		colForm(mixH.Node(), kit.NewDivider().Node(), mixV.Node()))

	// ---------- disabled.tsx (subset) ----------
	disForm := wireFace(kit.NewForm())
	disForm.SetLayout(kit.FormHorizontal)
	disForm.SetLabelCol(kit.FormCol{Span: 4})
	disForm.SetWrapperCol(kit.FormCol{Span: 14})
	disForm.SetDisabled(true)
	disForm.AddItem(kit.NewFormItemName("cb", "Checkbox").BindCheckbox(kit.NewCheckbox("Checkbox")))
	disForm.AddItem(kit.NewFormItemName("input", "Input").BindInput(kit.NewInput("")))
	disForm.AddItem(kit.NewFormItemName("sw", "Switch").BindSwitch(kit.NewSwitch()))
	disToggle := c.trackBtn(kit.NewButton("Toggle disabled"))
	disToggle.SetType(kit.ButtonPrimary)
	disToggle.SetOnClick(func() {
		disForm.SetDisabled(!disForm.Disabled)
		*status = fmt.Sprintf("form disabled=%v", disForm.Disabled)
	})
	secDis := demoSection(face, th, "Disabled",
		"表单禁用 — Form.disabled 下发子控件.",
		colForm(disToggle.Node(), disForm.Node()))

	// ---------- variant.tsx ----------
	varForm := wireFace(kit.NewForm())
	varForm.SetVariant(kit.FormFilled)
	varForm.SetLabelCol(kit.FormCol{Span: 6})
	varForm.SetWrapperCol(kit.FormCol{Span: 14})
	varIn := kit.NewInput("")
	varForm.AddItem(kit.NewFormItemName("Input", "Input").
		SetRules(kit.FormRule{Required: true, Message: "Please input!"}).
		BindInput(varIn))
	mkVar := func(label string, v kit.FormVariant) *kit.Button {
		b := c.trackBtn(kit.NewButton(label))
		b.SetSize(kit.ButtonSmall)
		b.SetOnClick(func() {
			varForm.SetVariant(v)
			*status = fmt.Sprintf("form variant=%v", v)
		})
		return b
	}
	secVar := demoSection(face, th, "Variant",
		"表单变体 — outlined / filled / borderless / underlined 下发.",
		colForm(
			spaceWrap(8,
				mkVar("outlined", kit.FormOutlined).Node(),
				mkVar("filled", kit.FormFilled).Node(),
				mkVar("borderless", kit.FormBorderless).Node(),
				mkVar("underlined", kit.FormUnderlined).Node(),
			),
			varForm.Node(),
		))

	// ---------- required-mark.tsx ----------
	reqForm := wireFace(kit.NewForm())
	reqForm.SetLayout(kit.FormVertical)
	reqForm.AddItem(kit.NewFormItemName("a", "Field A").SetRequired(true).BindInput(kit.NewInput("input placeholder")))
	reqForm.AddItem(kit.NewFormItemName("b", "Field B").BindInput(kit.NewInput("input placeholder")))
	mkMark := func(label string, m kit.FormRequiredMark) *kit.Button {
		b := c.trackBtn(kit.NewButton(label))
		b.SetSize(kit.ButtonSmall)
		b.SetOnClick(func() {
			reqForm.SetRequiredMark(m)
			*status = fmt.Sprintf("requiredMark=%v", m)
		})
		return b
	}
	secReq := demoSection(face, th, "Required mark",
		"必选样式 — default / optional / hidden.",
		colForm(
			spaceWrap(8,
				mkMark("Default", kit.FormRequiredMarkDefault).Node(),
				mkMark("Optional", kit.FormRequiredMarkOptional).Node(),
				mkMark("Hidden", kit.FormRequiredMarkHidden).Node(),
			),
			reqForm.Node(),
		))

	// ---------- size.tsx ----------
	sizeForm := wireFace(kit.NewForm())
	sizeForm.SetLayout(kit.FormHorizontal)
	sizeForm.SetLabelCol(kit.FormCol{Span: 4})
	sizeForm.SetWrapperCol(kit.FormCol{Span: 14})
	sizeForm.AddItem(kit.NewFormItemName("input", "Input").BindInput(kit.NewInput("")))
	sizeForm.AddItem(kit.NewFormItemName("sw", "Switch").BindSwitch(kit.NewSwitch()))
	mkSize := func(label string, s kit.FormSize) *kit.Button {
		b := c.trackBtn(kit.NewButton(label))
		b.SetSize(kit.ButtonSmall)
		b.SetOnClick(func() {
			sizeForm.SetSize(s)
			*status = fmt.Sprintf("form size=%v", s)
		})
		return b
	}
	secSize := demoSection(face, th, "Size",
		"表单尺寸 — small / middle / large 下发控件高 24/32/40.",
		colForm(
			spaceWrap(8,
				mkSize("Small", kit.FormSmall).Node(),
				mkSize("Medium", kit.FormMiddle).Node(),
				mkSize("Large", kit.FormLarge).Node(),
			),
			sizeForm.Node(),
		))

	c.add("form", "Form", "Data Entry · Form",
		demoPage(face, "Form",
			"High-performance form with data store. P0: layout/size/variant/disabled/requiredMark/rules/List/instance methods + official basic demos.",
			secBasic, secHooks, secLayout, secMix, secDis, secVar, secReq, secSize))
}

func colForm(kids ...core.Node) core.Node {
	f := primitive.Column(kids...)
	f.Gap = 12
	f.CrossAlign = core.CrossStretch
	f.MainAlign = core.MainStart
	return f
}
