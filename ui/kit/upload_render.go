package kit

// Fixed upload geometry aligned to upload.md §6.2.1.
// Painting stays in prim when F0-2 lands; this file only computes rects.
const (
	// UploadPictureCardSize is the photo-wall cell edge (~102).
	UploadPictureCardSize = 102.0
	// UploadThumbnailSize is the picture-list thumbnail edge (48).
	UploadThumbnailSize = 48.0
	// UploadProgressLine is the list progress line width (2).
	UploadProgressLine = 2.0
	// UploadItemPadX is the list item horizontal padding (4).
	UploadItemPadX = 4.0
	// UploadItemMarginTop is the list item top margin (4).
	UploadItemMarginTop = 4.0
	// UploadDragPad is the drag area inner padding (16).
	UploadDragPad = 16.0
	// UploadDragRadius is the drag area corner radius (8).
	UploadDragRadius = 8.0
	// UploadItemH is the text list row height.
	UploadItemH = 32.0
)

// UploadRect is one list/trigger rect in logical pixels.
type UploadRect struct {
	X, Y, W, H float64
}

// ComputeUploadCardCell places one photo-wall cell in a wrapping row.
// viewportW is reserved for responsive columns; callers currently pass
// an explicit column count.
func ComputeUploadCardCell(viewportW float64, index, columns int, gap float64) UploadRect {
	_ = viewportW
	if columns <= 0 {
		columns = 3
	}
	if gap < 0 {
		gap = 8
	}
	col := index % columns
	row := index / columns
	return UploadRect{
		X: float64(col) * (UploadPictureCardSize + gap),
		Y: float64(row) * (UploadPictureCardSize + gap),
		W: UploadPictureCardSize,
		H: UploadPictureCardSize,
	}
}

// ComputeUploadTextRow stacks one text list row.
func ComputeUploadTextRow(index int, width float64) UploadRect {
	return UploadRect{
		X: 0,
		Y: float64(index) * (UploadItemH + UploadItemMarginTop),
		W: width,
		H: UploadItemH,
	}
}

// UploadTriggerVisible reports whether the card trigger shows:
// maxCount reached hides it.
func UploadTriggerVisible(count, maxCount int) bool {
	if maxCount <= 0 {
		return true
	}
	return count < maxCount
}
