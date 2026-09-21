package kit_test

import (
	"testing"

	"github.com/energye/gpui/ui/kit"
)

func qrProps(value string) kit.QRCodeProps {
	p := kit.DefaultQRCodeProps()
	p.Value = value
	return p
}

func TestQRCode_StateMachineMatchesCases(t *testing.T) {
	// QR-S1/S2: non-empty value encodes; default size 160.
	host := kit.BuildQRCode(kit.DefaultScopeCtx(), qrProps("https://ant.design"))
	if host.Modules() < 21 {
		t.Fatalf("modules = %d want >= 21", host.Modules())
	}
	if host.Modules() != len(host.Matrix()) {
		t.Fatal("matrix edge must match Modules")
	}
	if kit.ResolveQRCodeSize(kit.DefaultQRCodeProps()) != 160 {
		t.Fatal("default size must be 160")
	}
	// Empty value never crashes and yields no matrix.
	empty := kit.BuildQRCode(kit.DefaultScopeCtx(), kit.DefaultQRCodeProps())
	if empty.Modules() != 0 || empty.Matrix() != nil {
		t.Fatal("empty value must yield no matrix")
	}
	// QR-S3/S4: expired cover + refresh fires once.
	host.SetCoverStatus(kit.QRCodeStatusExpired)
	if !host.HasCover() {
		t.Fatal("expired must show cover")
	}
	fires := 0
	host.SetOnRefresh(func() { fires++ })
	if !host.ClickRefresh() || fires != 1 {
		t.Fatalf("one click must fire exactly once, fires = %d", fires)
	}
	active := kit.BuildQRCode(kit.DefaultScopeCtx(), qrProps("x"))
	if active.ClickRefresh() {
		t.Fatal("active must not refresh")
	}
	// QR-S5: icon flag follows props.
	iprops := qrProps("x")
	iprops.Icon = "https://example.com/logo.png"
	ihost := kit.BuildQRCode(kit.DefaultScopeCtx(), iprops)
	if !ihost.HasIcon() {
		t.Fatal("icon src must report HasIcon")
	}
	// QR-S9/S10: loading spins with Tick; scanned covers.
	lhost := kit.BuildQRCode(kit.DefaultScopeCtx(), qrProps("x"))
	lhost.SetCoverStatus(kit.QRCodeStatusLoading)
	lhost.Tick(0.25)
	if lhost.SpinAngle() != 90 {
		t.Fatalf("spin = %v want 90", lhost.SpinAngle())
	}
	if !lhost.HasCover() || lhost.CoverText() != "" {
		t.Fatal("loading cover must show with empty default text")
	}
	lhost.SetCoverStatus(kit.QRCodeStatusScanned)
	if !lhost.HasCover() || lhost.CoverText() == "" {
		t.Fatal("scanned must cover with copy")
	}
	// QR-S11: statusRender overrides default cover copy.
	sprops := qrProps("x")
	sprops.Status = kit.QRCodeStatusExpired
	sprops.StatusRender = func(kit.QRCodeStatusInfo) string { return "custom cover" }
	shost := kit.BuildQRCode(kit.DefaultScopeCtx(), sprops)
	shost.SetCoverStatus(kit.QRCodeStatusExpired)
	if shost.CoverText() != "custom cover" {
		t.Fatalf("cover = %q want custom", shost.CoverText())
	}
	// Controlled status wins over cover flips.
	chost := kit.BuildQRCode(kit.DefaultScopeCtx(), qrProps("x"))
	chost.SetStatus(kit.QRCodeStatusExpired)
	chost.SetCoverStatus(kit.QRCodeStatusActive)
	if chost.Status() != kit.QRCodeStatusExpired {
		t.Fatal("controlled status must win")
	}
	// SetValue re-encodes live.
	chost.SetValue("hello")
	if chost.Modules() != 21 {
		t.Fatalf("re-encode modules = %d want 21", chost.Modules())
	}
	// Values array resolves as multi input.
	vprops := kit.DefaultQRCodeProps()
	vprops.Values = []string{"a", "b"}
	vprops.ValuesSet = true
	vhost := kit.BuildQRCode(kit.DefaultScopeCtx(), vprops)
	if len(vhost.Values()) != 2 || vhost.Modules() < 21 {
		t.Fatal("string[] must resolve two inputs and encode the first")
	}
	// Update with identical inputs skips re-encode; new value re-encodes.
	uprops := qrProps("hello")
	uhost := kit.BuildQRCode(kit.DefaultScopeCtx(), uprops)
	before := uhost.Modules()
	uhost.Update(kit.DefaultScopeCtx(), uprops)
	if uhost.Modules() != before {
		t.Fatal("same-input Update must keep the matrix")
	}
	uprops.Value = "https://ant.design"
	uhost.Update(kit.DefaultScopeCtx(), uprops)
	if uhost.Modules() == before {
		t.Fatal("new-value Update must re-encode")
	}
}
