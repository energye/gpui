package kit

// QRErrorLevel selects the error-correction level.
type QRErrorLevel string

const (
	QRErrorLevelL QRErrorLevel = "L"
	QRErrorLevelM QRErrorLevel = "M"
	QRErrorLevelQ QRErrorLevel = "Q"
	QRErrorLevelH QRErrorLevel = "H"
)

// QRCodeStatus selects the cover state.
type QRCodeStatus string

const (
	QRCodeStatusActive  QRCodeStatus = "active"
	QRCodeStatusExpired QRCodeStatus = "expired"
	QRCodeStatusLoading QRCodeStatus = "loading"
	QRCodeStatusScanned QRCodeStatus = "scanned"
)

// QRCodeType selects the render backend label. Both backends draw the
// same module matrix; the type only changes the semantic/export label.
type QRCodeType string

const (
	QRCodeTypeCanvas QRCodeType = "canvas"
	QRCodeTypeSVG    QRCodeType = "svg"
)

// QRCodeStatusInfo feeds the statusRender hook.
type QRCodeStatusInfo struct {
	Status    QRCodeStatus
	Expired   string
	Refresh   string
	Scanned   string
	OnRefresh func()
}

// QRCodeStatusRender customizes the cover content (P0 string hook;
// the full ReactNode visual lands with the F component).
type QRCodeStatusRender func(info QRCodeStatusInfo) string

// QRCodeProps configures one QR code host.
type QRCodeProps struct {
	Value         string
	Values        []string
	ValuesSet     bool
	Type          QRCodeType
	TypeSet       bool
	Size          float64
	SizeSet       bool
	Icon          string
	IconSize      float64
	IconSizeSet   bool
	IconSizeW     float64
	IconSizeH     float64
	IconSizeWHSet bool
	Color         string
	ColorSet      bool
	BgColor       string
	BgColorSet    bool
	MarginSize    int
	Bordered      bool
	BorderedSet   bool
	ErrorLevel    QRErrorLevel
	BoostLevel    bool
	BoostLevelSet bool
	Status        QRCodeStatus
	OnRefresh     func()
	StatusRender  QRCodeStatusRender
	AriaLabel     string
}

// DefaultQRCodeProps returns antd 6.5.1 aligned defaults.
func DefaultQRCodeProps() QRCodeProps {
	return QRCodeProps{
		Type:          QRCodeTypeCanvas,
		TypeSet:       false,
		Size:          160,
		SizeSet:       false,
		IconSize:      40,
		IconSizeSet:   false,
		Color:         "",
		ColorSet:      false,
		BgColor:       "",
		BgColorSet:    false,
		MarginSize:    0,
		Bordered:      true,
		BorderedSet:   false,
		ErrorLevel:    QRErrorLevelM,
		BoostLevel:    true,
		BoostLevelSet: false,
		Status:        QRCodeStatusActive,
	}
}

// ResolveQRCodeSize returns explicit size else 160.
func ResolveQRCodeSize(props QRCodeProps) float64 {
	if props.SizeSet && props.Size > 0 {
		return props.Size
	}
	if props.Size > 0 {
		return props.Size
	}
	return 160
}

// ResolveQRCodeIconSize returns the icon box (default 40x40).
func ResolveQRCodeIconSize(props QRCodeProps) (w, h float64) {
	if props.IconSizeWHSet && props.IconSizeW > 0 && props.IconSizeH > 0 {
		return props.IconSizeW, props.IconSizeH
	}
	if props.IconSizeSet && props.IconSize > 0 {
		return props.IconSize, props.IconSize
	}
	if props.IconSize > 0 {
		return props.IconSize, props.IconSize
	}
	return 40, 40
}

// ResolveQRCodeValues returns the encode inputs: Values wins when set,
// otherwise the single Value. Empty means no matrix (antd renders null).
func ResolveQRCodeValues(props QRCodeProps) []string {
	if props.ValuesSet {
		return props.Values
	}
	if props.Value != "" {
		return []string{props.Value}
	}
	return nil
}

// ResolveQRCodeLocale returns the status copy (en default, zh override).
func ResolveQRCodeLocale(locale string) (expired, refresh, scanned string) {
	expired, refresh, scanned = "QR code expired", "Refresh", "Scanned"
	if len(locale) >= 2 && (locale[0] == 'z' || locale[0] == 'Z') {
		return "二维码已过期", "刷新", "已扫描"
	}
	return expired, refresh, scanned
}
