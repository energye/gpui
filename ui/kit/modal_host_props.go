package kit

// ModalHostConfirmKind selects the imperative confirm family.
type ModalHostConfirmKind string

const (
	ModalHostKindConfirm ModalHostConfirmKind = "confirm"
	ModalHostKindInfo    ModalHostConfirmKind = "info"
	ModalHostKindSuccess ModalHostConfirmKind = "success"
	ModalHostKindError   ModalHostConfirmKind = "error"
	ModalHostKindWarning ModalHostConfirmKind = "warning"
)

// ModalHostMask carries mask.enabled/blur/closable (antd 6.5.1 mask object).
type ModalHostMask struct {
	Enabled  bool
	Blur     bool
	Closable bool
}

// DefaultModalHostMask returns mask=true semantics.
func DefaultModalHostMask() ModalHostMask {
	return ModalHostMask{Enabled: true, Blur: false, Closable: true}
}

// MergeModalHostMask resolves mask object over deprecated maskClosable.
// maskClosable nil means unset; explicit mask.Closable wins when set.
func MergeModalHostMask(mask ModalHostMask, maskSet bool, maskClosable *bool) ModalHostMask {
	if !maskSet {
		mask = DefaultModalHostMask()
	}
	if maskClosable != nil {
		mask.Closable = *maskClosable
	}
	return mask
}

// ModalHostProps configures the App-level dialog stack host.
type ModalHostProps struct {
	ZIndexBase int
	Mask       ModalHostMask
	MaskSet    bool
	ScrollLock bool
}

// DefaultModalHostProps returns zIndex 1000 with mask and scroll lock on.
func DefaultModalHostProps() ModalHostProps {
	return ModalHostProps{
		ZIndexBase: 1000,
		Mask:       DefaultModalHostMask(),
		MaskSet:    false,
		ScrollLock: true,
	}
}

// ModalHostConfirmConfig is one imperative dialog (confirm/info/...).
type ModalHostConfirmConfig struct {
	Kind            ModalHostConfirmKind
	Title           string
	Content         string
	OkText          string
	CancelText      string
	Width           float64
	WidthSet        bool
	Centered        bool
	Mask            ModalHostMask
	MaskSet         bool
	MaskClosable    *bool
	Keyboard        bool
	KeyboardSet     bool
	Closable        bool
	ClosableSet     bool
	Loading         bool
	DestroyOnHidden bool
	OkLoading       bool
}

// DefaultModalHostConfirmConfig returns method defaults (width 416).
func DefaultModalHostConfirmConfig() ModalHostConfirmConfig {
	return ModalHostConfirmConfig{
		Kind:        ModalHostKindConfirm,
		Width:       416,
		WidthSet:    false,
		Centered:    false,
		Mask:        DefaultModalHostMask(),
		MaskSet:     false,
		Keyboard:    true,
		KeyboardSet: false,
		Closable:    false,
		ClosableSet: false,
	}
}

// ResolveModalHostConfirmText fills ok/cancel copy from locale when empty.
func ResolveModalHostConfirmText(cfg ModalHostConfirmConfig, locale string) (ok, cancel string) {
	ok, cancel = cfg.OkText, cfg.CancelText
	if ok == "" || cancel == "" {
		defOk, defCancel := "OK", "Cancel"
		if len(locale) >= 2 && (locale[0] == 'z' || locale[0] == 'Z') {
			defOk, defCancel = "确定", "取消"
		}
		if ok == "" {
			ok = defOk
		}
		if cancel == "" {
			cancel = defCancel
		}
	}
	return ok, cancel
}
