package kit_test

import (
	"testing"

	"github.com/energye/gpui/ui/kit"
	"github.com/energye/gpui/ui/theme"
)

func TestUpload_GeometryThemeMatchesCases(t *testing.T) {
	cases := loadUploadCases(t)
	geo := cases["geometry"].(map[string]any)

	if kit.UploadPictureCardSize != geo["card_cell"].(map[string]any)["size"].(float64) {
		t.Fatalf("card = %v want 102", kit.UploadPictureCardSize)
	}
	if kit.UploadThumbnailSize != geo["thumb"].(float64) {
		t.Fatalf("thumb = %v want 48", kit.UploadThumbnailSize)
	}
	if kit.UploadProgressLine != geo["progress_line"].(float64) {
		t.Fatalf("progress line = %v want 2", kit.UploadProgressLine)
	}
	if kit.UploadItemPadX != geo["item_pad"].(float64) || kit.UploadItemMarginTop != geo["item_margin"].(float64) {
		t.Fatal("item pad/margin must be 4/4")
	}
	if kit.UploadDragPad != geo["drag_pad"].(float64) || kit.UploadDragRadius != geo["drag_radius"].(float64) {
		t.Fatal("drag pad/radius must be 16/8")
	}
	cell := kit.ComputeUploadCardCell(400, 4, 3, 8)
	if cell.W != 102 || cell.X != 110 || cell.Y != 110 {
		t.Fatalf("card cell[4] = %+v want x110 y110", cell)
	}
	row := kit.ComputeUploadTextRow(1, 300)
	if row.Y != 36 || row.H != geo["item_h"].(float64) {
		t.Fatalf("text row[1] = %+v", row)
	}
	if !kit.UploadTriggerVisible(2, 3) || kit.UploadTriggerVisible(3, 3) {
		t.Fatal("trigger hides at maxCount")
	}

	// Static call eats theme; disabled ORs through Ctx.
	base := kit.DefaultScopeCtx()
	skin := base.Theme
	skin.ColorPrimary = theme.Hex("#722ed1")
	skinCtx := base.WithTheme(skin)
	host := kit.BuildUpload(base, kit.DefaultUploadProps())
	host.Update(skinCtx, kit.DefaultUploadProps())
	got := kit.ResolveUpload(skinCtx.Theme)
	if got.Selected != skin.ColorPrimary {
		t.Fatal("selected must follow reskinned primary without code change")
	}
	if got.Error != skin.ColorError {
		t.Fatal("error tint must follow the error token")
	}
	if host.HolderContent() == "" {
		t.Fatal("holderRender must resolve through Ctx")
	}
	disCtx := base
	disCtx.Disabled = true
	dhost := kit.BuildUpload(disCtx, kit.DefaultUploadProps())
	if !dhost.Disabled() {
		t.Fatal("subtree disabled must disable the upload")
	}
}
