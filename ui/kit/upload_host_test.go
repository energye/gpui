package kit_test

import (
	"testing"

	"github.com/energye/gpui/ui/kit"
)

func TestUpload_HostTrioAcceptPreview(t *testing.T) {
	cases := loadUploadCases(t)
	acc := cases["accept"].(map[string]any)

	// UPL-S9: drag area drops like a select.
	dhost := kit.BuildUploadDragger(kit.DefaultScopeCtx(), kit.DefaultUploadDraggerProps())
	dropped := false
	dp := kit.DefaultUploadDraggerProps()
	dp.OnDrop = func([]kit.UploadLocalFile) { dropped = true }
	dhost.Update(kit.DefaultScopeCtx(), dp)
	dadded := dhost.DropFiles([]kit.UploadLocalFile{{Name: "d.png"}})
	if len(dadded) != 1 || !dropped {
		t.Fatal("drop must list one file and fire onDrop")
	}

	// Paste gated by pastable.
	pprops := kit.DefaultUploadProps()
	pprops.Pastable = true
	phost := kit.BuildUpload(kit.DefaultScopeCtx(), pprops)
	if len(phost.PasteFiles([]kit.UploadLocalFile{{Name: "c.png"}})) != 1 {
		t.Fatal("pastable must accept paste")
	}
	nphost := kit.BuildUpload(kit.DefaultScopeCtx(), kit.DefaultUploadProps())
	if len(nphost.PasteFiles([]kit.UploadLocalFile{{Name: "c.png"}})) != 0 {
		t.Fatal("non-pastable must refuse paste")
	}

	// Picker + paste provider injection.
	hprops := kit.DefaultUploadProps()
	hprops.Picker = func() ([]kit.UploadLocalFile, bool) {
		return []kit.UploadLocalFile{{Name: "picked.png"}}, true
	}
	hprops.PasteProvider = func() []kit.UploadLocalFile {
		return []kit.UploadLocalFile{{Name: "clip.png"}}
	}
	hprops.Pastable = true
	hhost := kit.BuildUpload(kit.DefaultScopeCtx(), hprops)
	if len(hhost.PickFromHost()) != 1 {
		t.Fatal("picker must feed select")
	}
	if len(hhost.PasteFromHost()) != 1 {
		t.Fatal("paste provider must feed paste")
	}

	// UPL-S11/UPL-11: accept filter from the case file.
	accept := ".png,image/*"
	if !kit.MatchUploadAccept(accept, acc["ext_png_match"].(string), "image/png") {
		t.Fatal("a.PNG must pass .png")
	}
	if !kit.MatchUploadAccept(accept, "photo.jpg", "image/jpeg") {
		t.Fatal("mime prefix image/* must pass jpg")
	}
	if kit.MatchUploadAccept(accept, "doc.pdf", "application/pdf") {
		t.Fatal("pdf must not pass .png,image/*")
	}
	if !kit.MatchUploadAccept("", "anything.exe", "") {
		t.Fatal("empty accept must pass everything")
	}
	// Accept filters at select time.
	aprops := kit.DefaultUploadProps()
	aprops.Accept = ".png"
	ahost := kit.BuildUpload(kit.DefaultScopeCtx(), aprops)
	if len(ahost.SelectFiles([]kit.UploadLocalFile{{Name: "a.bmp"}})) != 0 {
		t.Fatal("bmp must not enter a png-only list")
	}

	// Preview pipeline: previewFile then onPreview.
	vprops := kit.DefaultUploadProps()
	vprops.PreviewFile = func(l kit.UploadLocalFile) (string, bool) {
		return "data:image/png;base64,xx", true
	}
	previewed := ""
	vprops.OnPreview = func(f kit.UploadFile) { previewed = f.Name }
	vhost := kit.BuildUpload(kit.DefaultScopeCtx(), vprops)
	vadded := vhost.SelectFiles([]kit.UploadLocalFile{{Name: "p.png"}})
	url, ok := vhost.Preview(vadded[0].UID)
	if !ok || url != "data:image/png;base64,xx" || previewed != "p.png" {
		t.Fatalf("preview = %q,%v,%q", url, ok, previewed)
	}
}
