package kit

// Overlay ZOrder ladder (higher draws/hits above). Compositor dual-band puts the
// entire OverlayHost above main ScrollViewport layers (MAP §4.1).
//
//	Drawer        400
//	Modal         500
//	ImagePreview  550  (antd zIndexPopupBase+80 ladder)
//	Message       600
//	Tour          700
//
// Anchored popups (Tooltip/Select/Dropdown) use primitive defaults (~200) unless
// overridden — they still sit in the Overlay band above main content.
const (
	OverlayZDrawer       = 400
	OverlayZModal        = 500
	OverlayZImagePreview = 550
	OverlayZMessage      = 600
	OverlayZTour         = 700
)
