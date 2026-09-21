package kit_test

import (
	"testing"

	"github.com/energye/gpui/ui/kit"
)

func TestUpload_SelectProgressRemoveMaxCount(t *testing.T) {
	cases := loadUploadCases(t)
	uid := cases["uid"].(map[string]any)

	// UPL-S1: select one file lists it with uid fill + onChange.
	changes := 0
	sprops := withUploadOnChange(kit.DefaultUploadProps(), func() { changes++ })
	host := kit.BuildUpload(kit.DefaultScopeCtx(), sprops)
	added := host.SelectFiles([]kit.UploadLocalFile{{Name: "a.png", Type: "image/png", Size: 10}})
	if len(added) != 1 || added[0].UID != uid["first"].(string) {
		t.Fatalf("first uid = %+v", added)
	}
	if len(host.FileList()) != 1 || changes != 1 {
		t.Fatalf("list = %d changes = %d", len(host.FileList()), changes)
	}

	// Default request ticks to done without blocking the caller:
	// 50%/s, so 0.5s is mid-flight and 2s completes.
	host.Tick(0.5)
	if got := host.UploadingCount(); got != 1 {
		t.Fatalf("mid-flight still uploading = %d", got)
	}
	host.Tick(2)
	f, ok := host.Find(added[0].UID)
	if !ok || f.Status != kit.UploadStatusDone || f.Percent != 100 {
		t.Fatalf("after 2s = %+v,%v", f, ok)
	}

	// UPL-S5: remove deletes + notifies; veto blocks.
	second := host.SelectFiles([]kit.UploadLocalFile{{Name: "b.png"}})
	if len(second) != 1 || second[0].UID != uid["second"].(string) {
		t.Fatalf("second uid = %+v", second)
	}
	if !host.Remove(second[0].UID) || len(host.FileList()) != 1 {
		t.Fatal("remove must delete")
	}
	vprops := kit.DefaultUploadProps()
	vprops.OnRemove = func(f kit.UploadFile) bool { return false }
	vhost := kit.BuildUpload(kit.DefaultScopeCtx(), vprops)
	vadded := vhost.SelectFiles([]kit.UploadLocalFile{{Name: "v.png"}})
	if vhost.Remove(vadded[0].UID) || len(vhost.FileList()) != 1 {
		t.Fatal("onRemove false must veto")
	}

	// UPL-S6: maxCount=1 replaces with the latest.
	mprops := kit.DefaultUploadProps()
	mprops.MaxCount = 1
	mhost := kit.BuildUpload(kit.DefaultScopeCtx(), mprops)
	mhost.SelectFiles([]kit.UploadLocalFile{{Name: "old.png"}})
	mhost.SelectFiles([]kit.UploadLocalFile{{Name: "new.png"}})
	list := mhost.FileList()
	if len(list) != 1 || list[0].Name != "new.png" {
		t.Fatalf("maxCount=1 = %+v", list)
	}

	// UPL-S7: disabled selects nothing.
	dprops := kit.DefaultUploadProps()
	dprops.Disabled = true
	dhost := kit.BuildUpload(kit.DefaultScopeCtx(), dprops)
	if len(dhost.SelectFiles([]kit.UploadLocalFile{{Name: "x.png"}})) != 0 {
		t.Fatal("disabled must select nothing")
	}
}

func withUploadOnChange(p kit.UploadProps, fn func()) kit.UploadProps {
	p.OnChange = func(kit.UploadChangeParam) { fn() }
	return p
}
