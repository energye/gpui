package kit_test

import (
	"testing"

	"github.com/energye/gpui/ui/kit"
)

func TestUpload_BeforeUploadCustomRequestControlled(t *testing.T) {
	// UPL-S2: beforeUpload false keeps the file listed without uploading.
	sprops := kit.DefaultUploadProps()
	sprops.BeforeUpload = func(f kit.UploadLocalFile, b []kit.UploadLocalFile) kit.UploadBeforeAction {
		return kit.UploadBeforeSkip
	}
	shost := kit.BuildUpload(kit.DefaultScopeCtx(), sprops)
	sadded := shost.SelectFiles([]kit.UploadLocalFile{{Name: "manual.png"}})
	if len(sadded) != 1 {
		t.Fatal("skip must still list the file")
	}
	shost.Tick(10)
	f, _ := shost.Find(sadded[0].UID)
	if f.Status == kit.UploadStatusDone {
		t.Fatal("skipped file must never auto-upload")
	}

	// LIST_IGNORE drops the file entirely.
	iprops := kit.DefaultUploadProps()
	iprops.BeforeUpload = func(f kit.UploadLocalFile, b []kit.UploadLocalFile) kit.UploadBeforeAction {
		return kit.UploadBeforeIgnore
	}
	ihost := kit.BuildUpload(kit.DefaultScopeCtx(), iprops)
	if len(ihost.SelectFiles([]kit.UploadLocalFile{{Name: "ignore.png"}})) != 0 {
		t.Fatal("ignore must list nothing")
	}
	if len(ihost.FileList()) != 0 {
		t.Fatal("ignored file must not enter the list")
	}

	// UPL-S3/S4: business customRequest drives progress/success/error.
	var captured kit.UploadRequestOptions
	cprops := kit.DefaultUploadProps()
	cprops.CustomRequest = func(o kit.UploadRequestOptions) { captured = o }
	chost := kit.BuildUpload(kit.DefaultScopeCtx(), cprops)
	cadded := chost.SelectFiles([]kit.UploadLocalFile{{Name: "biz.png"}})
	if captured.File.UID != cadded[0].UID {
		t.Fatal("customRequest must receive the file")
	}
	chost.Tick(100)
	if got, _ := chost.Find(cadded[0].UID); got.Percent != 0 {
		t.Fatal("business files must not move on Tick")
	}
	captured.OnProgress(40)
	if got, _ := chost.Find(cadded[0].UID); got.Percent != 40 {
		t.Fatalf("progress = %v want 40", got.Percent)
	}
	captured.OnSuccess(`{"status":"success"}`)
	if got, _ := chost.Find(cadded[0].UID); got.Status != kit.UploadStatusDone {
		t.Fatal("success must mark done")
	}
	eadded := chost.SelectFiles([]kit.UploadLocalFile{{Name: "bad.png"}})
	captured.OnError("timeout")
	if got, _ := chost.Find(eadded[0].UID); got.Status != kit.UploadStatusError || got.Error != "timeout" {
		t.Fatalf("error = %+v", got)
	}

	// Controlled list: outside drives, non-list events ignored (FAQ).
	cprops2 := kit.DefaultUploadProps()
	cprops2.Controlled = true
	chost2 := kit.BuildUpload(kit.DefaultScopeCtx(), cprops2)
	keep := []kit.UploadFile{{UID: "k1", Name: "keep.png", Status: kit.UploadStatusDone, Percent: 100}}
	chost2.SetFileList(keep)
	fired := false
	cprops2.OnChange = func(kit.UploadChangeParam) { fired = true }
	chost2.Update(kit.DefaultScopeCtx(), cprops2)
	chost2.SelectFiles([]kit.UploadLocalFile{{Name: "new.png"}})
	_ = fired
	chost2.ReportProgress("ghost", 50)
	chost2.ReportSuccess("ghost", "")
	chost2.ReportError("ghost", "x")
	if len(chost2.FileList()) < 1 {
		t.Fatal("controlled list must survive ghost events")
	}
}
