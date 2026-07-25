package kit_test

import (
	"strings"
	"testing"

	"github.com/energye/gpui/render"
	"github.com/energye/gpui/ui/core"
	"github.com/energye/gpui/ui/kit"
)

// docs/antd/transfer.md §6.9 — P0 PRD cases (TF-01 … TF-20).
// L3/L4 (TF-21/22) and P1 (TF-23) deferred.

func mockTransferData(n int) []kit.TransferItem {
	out := make([]kit.TransferItem, n)
	for i := 0; i < n; i++ {
		out[i] = kit.TransferItem{
			Key:         itoa(i),
			Title:       "content" + itoa(i+1),
			Description: "description of content" + itoa(i+1),
		}
	}
	return out
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [16]byte
	i := len(b)
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}

func layoutTransfer(t *testing.T, tr *kit.Transfer) *core.Tree {
	t.Helper()
	tree := core.NewTree(tr.Node())
	tree.Layout(core.Size{Width: 640, Height: 360})
	return tree
}

func containsKey(keys []string, k string) bool {
	for _, x := range keys {
		if x == k {
			return true
		}
	}
	return false
}

func TestTransfer_PRD_01_Defaults(t *testing.T) {
	// TF-01
	tr := kit.NewTransfer()
	if tr.Disabled || tr.OneWay || tr.ShowSearch || tr.Controlled {
		t.Fatalf("flags want false: disabled=%v oneWay=%v search=%v controlled=%v",
			tr.Disabled, tr.OneWay, tr.ShowSearch, tr.Controlled)
	}
	if !tr.ShowSelectAll {
		t.Fatal("ShowSelectAll default want true")
	}
	if tr.Status != kit.TransferStatusNone {
		t.Fatalf("Status=%v want none", tr.Status)
	}
	if tr.Node() == nil || tr.Root == nil {
		t.Fatal("nil node")
	}
	if tr.Root.Base().Role != "group" {
		t.Fatalf("role=%q want group", tr.Root.Base().Role)
	}
	if len(tr.TargetKeys()) != 0 {
		t.Fatalf("targetKeys=%v want empty", tr.TargetKeys())
	}
	if tr.ListWidth() != kit.DefaultTransferListWidth {
		t.Fatalf("listWidth=%v want %v", tr.ListWidth(), kit.DefaultTransferListWidth)
	}
	if tr.ListHeight() != kit.DefaultTransferListHeight {
		t.Fatalf("listHeight=%v want %v", tr.ListHeight(), kit.DefaultTransferListHeight)
	}
	r, l := tr.ActionButtons()
	if r == nil {
		t.Fatal("missing toRight button")
	}
	if l == nil {
		t.Fatal("missing toLeft button by default")
	}
}

func TestTransfer_PRD_02_MoveRightOne(t *testing.T) {
	// TF-02 / TF-S1
	tr := kit.NewTransfer()
	tr.SetDataSource(mockTransferData(5))
	var gotKeys []string
	var gotDir kit.TransferDirection
	var gotMove []string
	tr.SetOnChange(func(targetKeys []string, direction kit.TransferDirection, moveKeys []string) {
		gotKeys = append([]string(nil), targetKeys...)
		gotDir = direction
		gotMove = append([]string(nil), moveKeys...)
	})
	tr.SelectItem(kit.TransferLeft, "1", true)
	tr.Move(kit.TransferRight)
	if !containsKey(tr.TargetKeys(), "1") {
		t.Fatalf("targetKeys=%v want contain 1", tr.TargetKeys())
	}
	if gotDir != kit.TransferRight || !containsKey(gotMove, "1") {
		t.Fatalf("onChange dir=%v move=%v", gotDir, gotMove)
	}
	if !containsKey(gotKeys, "1") {
		t.Fatalf("onChange target=%v", gotKeys)
	}
	// left no longer has 1
	for _, it := range tr.FilteredItems(kit.TransferLeft) {
		if it.Key == "1" {
			t.Fatal("key 1 still on left")
		}
	}
}

func TestTransfer_PRD_03_MoveLeft(t *testing.T) {
	// TF-03 / TF-S2
	tr := kit.NewTransfer()
	tr.SetDataSource(mockTransferData(5))
	tr.SetTargetKeys([]string{"2", "3"})
	tr.SelectItem(kit.TransferRight, "2", true)
	tr.Move(kit.TransferLeft)
	if containsKey(tr.TargetKeys(), "2") {
		t.Fatalf("targetKeys still has 2: %v", tr.TargetKeys())
	}
	if !containsKey(tr.TargetKeys(), "3") {
		t.Fatalf("targetKeys lost 3: %v", tr.TargetKeys())
	}
}

func TestTransfer_PRD_04_SearchLeft(t *testing.T) {
	// TF-04 / TF-S3
	tr := kit.NewTransfer()
	tr.SetDataSource(mockTransferData(10))
	tr.SetShowSearch(true)
	var searchDir kit.TransferDirection
	var searchVal string
	tr.SetOnSearch(func(dir kit.TransferDirection, v string) {
		searchDir, searchVal = dir, v
	})
	tr.SetSearch(kit.TransferLeft, "content2")
	if searchDir != kit.TransferLeft || searchVal != "content2" {
		t.Fatalf("onSearch dir=%v val=%q", searchDir, searchVal)
	}
	left := tr.FilteredItems(kit.TransferLeft)
	if len(left) == 0 {
		t.Fatal("filter emptied all")
	}
	for _, it := range left {
		lab := it.Title + " " + it.Description
		if !strings.Contains(strings.ToLower(lab), "content2") {
			t.Fatalf("unfiltered item %v", it)
		}
	}
}

func TestTransfer_PRD_05_SelectAllMoveRight(t *testing.T) {
	// TF-05 / TF-S4
	tr := kit.NewTransfer()
	tr.SetDataSource(mockTransferData(4))
	tr.SelectAllVisible(kit.TransferLeft)
	if len(tr.SourceSelectedKeys()) != 4 {
		t.Fatalf("selected=%v", tr.SourceSelectedKeys())
	}
	tr.Move(kit.TransferRight)
	if len(tr.TargetKeys()) != 4 {
		t.Fatalf("target=%v", tr.TargetKeys())
	}
	if len(tr.FilteredItems(kit.TransferLeft)) != 0 {
		t.Fatalf("left still has %v", tr.FilteredItems(kit.TransferLeft))
	}
}

func TestTransfer_PRD_06_Disabled(t *testing.T) {
	// TF-06 / TF-S5
	tr := kit.NewTransfer()
	tr.SetDataSource(mockTransferData(3))
	tr.SetDisabled(true)
	changed := false
	tr.SetOnChange(func([]string, kit.TransferDirection, []string) { changed = true })
	tr.SelectItem(kit.TransferLeft, "0", true)
	tr.Move(kit.TransferRight)
	if changed || len(tr.TargetKeys()) != 0 {
		t.Fatalf("disabled moved: changed=%v target=%v", changed, tr.TargetKeys())
	}
	// re-enable and ensure works
	tr.SetDisabled(false)
	tr.SelectItem(kit.TransferLeft, "0", true)
	tr.Move(kit.TransferRight)
	if !containsKey(tr.TargetKeys(), "0") {
		t.Fatal("enabled move failed")
	}
}

func TestTransfer_PRD_07_OneWay(t *testing.T) {
	// TF-07 / TF-S6
	tr := kit.NewTransfer()
	tr.SetDataSource(mockTransferData(4))
	tr.SetOneWay(true)
	tr.SetTargetKeys([]string{"1"})
	_, leftBtn := tr.ActionButtons()
	if leftBtn != nil {
		t.Fatal("oneWay should hide toLeft button")
	}
	// programmatic left move blocked
	tr.SelectItem(kit.TransferRight, "1", true)
	tr.Move(kit.TransferLeft)
	if !containsKey(tr.TargetKeys(), "1") {
		t.Fatal("oneWay Move(left) should no-op")
	}
	// remove via RemoveTargetItem
	tr.RemoveTargetItem("1")
	if containsKey(tr.TargetKeys(), "1") {
		t.Fatal("RemoveTargetItem failed")
	}
}

func TestTransfer_PRD_08_ControlledTargetKeys(t *testing.T) {
	// TF-08 / TF-S7
	tr := kit.NewTransfer()
	tr.SetDataSource(mockTransferData(5))
	tr.SetControlled(true)
	tr.SetTargetKeys([]string{"0"})
	var notified []string
	tr.SetOnChange(func(targetKeys []string, _ kit.TransferDirection, _ []string) {
		notified = append([]string(nil), targetKeys...)
	})
	tr.SelectItem(kit.TransferLeft, "1", true)
	tr.Move(kit.TransferRight)
	// local targetKeys unchanged until parent SetTargetKeys
	if len(tr.TargetKeys()) != 1 || tr.TargetKeys()[0] != "0" {
		t.Fatalf("controlled local target=%v", tr.TargetKeys())
	}
	if !containsKey(notified, "0") || !containsKey(notified, "1") {
		t.Fatalf("onChange notified=%v", notified)
	}
	tr.SetTargetKeys(notified)
	if !containsKey(tr.TargetKeys(), "1") {
		t.Fatalf("after parent write target=%v", tr.TargetKeys())
	}
}

func TestTransfer_PRD_09_BasicDemo(t *testing.T) {
	// TF-09 basic.tsx
	data := mockTransferData(20)
	var initial []string
	for _, it := range data {
		if n := it.Key; n > "10" || len(n) > 2 { // keys "11".."19"
			// number > 10
		}
	}
	for i := 11; i < 20; i++ {
		initial = append(initial, itoa(i))
	}
	tr := kit.NewTransfer()
	tr.SetDataSource(data)
	tr.SetTitles("Source", "Target")
	tr.SetControlled(true)
	tr.SetTargetKeys(initial)
	tr.SetRender(func(it kit.TransferItem) string { return it.Title })
	tr.SetOnChange(func(next []string, _ kit.TransferDirection, _ []string) {
		tr.SetTargetKeys(next)
	})
	layoutTransfer(t, tr)
	if len(tr.TargetKeys()) != len(initial) {
		t.Fatalf("target=%d want %d", len(tr.TargetKeys()), len(initial))
	}
	// move one more
	tr.SelectItem(kit.TransferLeft, "0", true)
	tr.Move(kit.TransferRight)
	if !containsKey(tr.TargetKeys(), "0") {
		t.Fatal("basic move failed")
	}
}

func TestTransfer_PRD_10_OneWayDemo(t *testing.T) {
	// TF-10 oneWay.tsx
	data := mockTransferData(20)
	for i := range data {
		data[i].Disabled = i%3 < 1
	}
	tr := kit.NewTransfer()
	tr.SetDataSource(data)
	tr.SetOneWay(true)
	tr.SetTitles("Source", "Target")
	layoutTransfer(t, tr)
	_, left := tr.ActionButtons()
	if left != nil {
		t.Fatal("oneWay demo should hide left action")
	}
}

func TestTransfer_PRD_11_SearchDemo(t *testing.T) {
	// TF-11 search.tsx
	data := mockTransferData(20)
	var targets []string
	for i, it := range data {
		if i%2 == 0 {
			targets = append(targets, it.Key)
		}
	}
	tr := kit.NewTransfer()
	tr.SetDataSource(data)
	tr.SetTargetKeys(targets)
	tr.SetShowSearch(true)
	tr.SetFilterOption(func(input string, option kit.TransferItem, _ kit.TransferDirection) bool {
		return strings.Contains(option.Description, input)
	})
	tr.SetRender(func(it kit.TransferItem) string { return it.Title })
	layoutTransfer(t, tr)
	tr.SetSearch(kit.TransferLeft, "content3")
	left := tr.FilteredItems(kit.TransferLeft)
	for _, it := range left {
		if !strings.Contains(it.Description, "content3") {
			t.Fatalf("filter miss: %v", it)
		}
	}
}

func TestTransfer_PRD_12_AdvancedDemo(t *testing.T) {
	// TF-12 advanced.tsx — showSearch + actions + footer + custom size
	tr := kit.NewTransfer()
	tr.SetDataSource(mockTransferData(20))
	tr.SetShowSearch(true)
	tr.SetActions("to right", "to left")
	tr.SetListWidth(250)
	tr.SetListHeight(300)
	tr.SetFooter(func(dir kit.TransferDirection) core.Node {
		lab := "Left button reload"
		if dir == kit.TransferRight {
			lab = "Right button reload"
		}
		btn := kit.NewButton(lab)
		btn.SetSize(kit.ButtonSmall)
		return btn.Node()
	})
	tr.SetRender(func(it kit.TransferItem) string {
		return it.Title + "-" + it.Description
	})
	layoutTransfer(t, tr)
	r, l := tr.ActionButtons()
	if r == nil || l == nil {
		t.Fatal("actions missing")
	}
	if r.Label != "to right" || l.Label != "to left" {
		t.Fatalf("labels %q %q", r.Label, l.Label)
	}
	if tr.ListWidth() != 250 || tr.ListHeight() != 300 {
		t.Fatalf("size %v×%v", tr.ListWidth(), tr.ListHeight())
	}
}

func TestTransfer_PRD_13_CustomItemDemo(t *testing.T) {
	// TF-13 custom-item.tsx
	tr := kit.NewTransfer()
	tr.SetDataSource(mockTransferData(8))
	tr.SetRender(func(it kit.TransferItem) string {
		return it.Title + " - " + it.Description
	})
	layoutTransfer(t, tr)
	// ensure render used in filter default path
	tr.SetSearch(kit.TransferLeft, "description of content1")
	if len(tr.FilteredItems(kit.TransferLeft)) == 0 {
		t.Fatal("custom render text should still be searchable via description default")
	}
}

func TestTransfer_PRD_14_ActionsDemo(t *testing.T) {
	// TF-14 actions.tsx — custom actions + loading ticker path
	tr := kit.NewTransfer()
	tr.SetDataSource(mockTransferData(10))
	tr.SetTargetKeys([]string{"8", "9"})
	tr.SetActions("to right", "to left")
	tr.SetActionLoading(true, false)
	r, l := tr.ActionButtons()
	if r == nil || !r.Loading {
		t.Fatal("right action should be loading")
	}
	if l == nil || l.Loading {
		t.Fatal("left action should not be loading")
	}
	// Move right blocked while loading
	tr.SelectItem(kit.TransferLeft, "0", true)
	tr.Move(kit.TransferRight)
	if containsKey(tr.TargetKeys(), "0") {
		t.Fatal("move while loading should no-op")
	}
	tr.SetActionLoading(false, false)
	tr.SelectItem(kit.TransferLeft, "0", true)
	tr.Move(kit.TransferRight)
	if !containsKey(tr.TargetKeys(), "0") {
		t.Fatal("move after loading clear failed")
	}
}

func TestTransfer_PRD_15_PaginationDemo(t *testing.T) {
	// TF-15 large-data.tsx
	tr := kit.NewTransfer()
	data := mockTransferData(50)
	tr.SetDataSource(data)
	tr.SetPagination(true)
	if tr.ListWidth() != kit.DefaultTransferListWidthLG {
		t.Fatalf("pagination width=%v want %v", tr.ListWidth(), kit.DefaultTransferListWidthLG)
	}
	page := tr.PageItems(kit.TransferLeft)
	if len(page) != kit.DefaultTransferPageSize {
		t.Fatalf("page len=%d want %d", len(page), kit.DefaultTransferPageSize)
	}
	// total left = 50
	if n := len(tr.FilteredItems(kit.TransferLeft)); n != 50 {
		t.Fatalf("filtered left=%d", n)
	}
}

func TestTransfer_PRD_16_TableTransferDemo(t *testing.T) {
	// TF-16 table-transfer.tsx — ListBody children
	tr := kit.NewTransfer()
	tr.SetDataSource(mockTransferData(8))
	tr.SetShowSearch(true)
	tr.SetShowSelectAll(false)
	var bodyCalls int
	tr.SetListBody(func(p kit.TransferListBodyProps) core.Node {
		bodyCalls++
		// minimal table body: show count text
		tx := kit.NewText("rows=" + itoa(len(p.FilteredItems)))
		return tx.Node()
	})
	layoutTransfer(t, tr)
	if bodyCalls < 2 {
		t.Fatalf("ListBody calls=%d want ≥2 (left+right)", bodyCalls)
	}
	// select via ListBody API
	tr.SelectItem(kit.TransferLeft, "1", true)
	tr.Move(kit.TransferRight)
	if !containsKey(tr.TargetKeys(), "1") {
		t.Fatal("table-transfer move failed")
	}
}

func TestTransfer_PRD_17_Metrics(t *testing.T) {
	// TF-17 §6.2
	tr := kit.NewTransfer()
	th := kit.DefaultTheme()
	if tr.ListWidth() != 180 {
		t.Fatalf("listWidth=%v", tr.ListWidth())
	}
	if tr.ListHeight() != 200 {
		t.Fatalf("listHeight=%v", tr.ListHeight())
	}
	hh := tr.HeaderHeight()
	wantH := th.SizeOr(core.TokenControlHeightLG, 40)
	if hh < wantH-0.5 || hh > wantH+0.5 {
		t.Fatalf("headerHeight=%v want %v", hh, wantH)
	}
	ih := tr.ItemHeight()
	wantI := th.SizeOr(core.TokenControlHeight, 32)
	if ih < wantI-0.5 || ih > wantI+0.5 {
		t.Fatalf("itemHeight=%v want %v", ih, wantI)
	}
	tr.SetPagination(true)
	if tr.ListWidth() != 250 {
		t.Fatalf("paginated listWidth=%v want 250", tr.ListWidth())
	}
}

func TestTransfer_PRD_18_ThemeColors(t *testing.T) {
	// TF-18 no hard-coded brand as sole skin — sections use theme tokens
	tr := kit.NewTransfer()
	tr.SetDataSource(mockTransferData(2))
	th := kit.DefaultTheme()
	tr.SetTheme(th)
	layoutTransfer(t, tr)
	// Status error / warning rebuild without panic
	tr.SetStatus(kit.TransferStatusError)
	left, right := tr.SectionNodes()
	if left == nil || right == nil {
		t.Fatal("nil sections")
	}
	tr.SetStatus(kit.TransferStatusWarning)
	// Token primary exists on theme (Transfer actions use Button primary → Token)
	c := th.Color(core.TokenColorPrimary)
	if c.A == 0 {
		t.Fatal("primary token missing")
	}
	errC := th.Color(core.TokenColorError)
	warnC := th.Color(core.TokenColorWarning)
	if approxTFColor(errC, warnC, 0.01) {
		t.Fatal("error and warning tokens should differ")
	}
}

func TestTransfer_PRD_19_DisabledChrome(t *testing.T) {
	// TF-19
	tr := kit.NewTransfer()
	tr.SetDataSource(mockTransferData(3))
	tr.SetDisabled(true)
	layoutTransfer(t, tr)
	left, _ := tr.SectionNodes()
	if left == nil {
		t.Fatal("nil section")
	}
	r, l := tr.ActionButtons()
	if r == nil || !r.Disabled {
		t.Fatal("toRight should be disabled")
	}
	if l == nil || !l.Disabled {
		t.Fatal("toLeft should be disabled")
	}
}

func TestTransfer_PRD_20_A11yFocus(t *testing.T) {
	// TF-20
	tr := kit.NewTransfer()
	tr.SetDataSource(mockTransferData(3))
	tr.SetAriaLabel("employee transfer")
	layoutTransfer(t, tr)
	if tr.Root.Base().Role != "group" {
		t.Fatalf("role=%q", tr.Root.Base().Role)
	}
	if tr.Root.Base().Label != "employee transfer" {
		t.Fatalf("label=%q", tr.Root.Base().Label)
	}
	left, right := tr.SectionNodes()
	if left.Base().Role != "listbox" || right.Base().Role != "listbox" {
		t.Fatalf("section roles %q %q", left.Base().Role, right.Base().Role)
	}
	r, _ := tr.ActionButtons()
	if r == nil {
		t.Fatal("nil action")
	}
	// action button focusable chrome exists
	tree := core.NewTree(tr.Node())
	tree.Layout(core.Size{Width: 640, Height: 360})
	// Space/Enter on action with selection
	tr.SelectItem(kit.TransferLeft, "0", true)
	tr.Move(kit.TransferRight)
	if !containsKey(tr.TargetKeys(), "0") {
		t.Fatal("keyboard-equivalent Move failed")
	}
}

func TestTransfer_PRD_StatusWarning(t *testing.T) {
	// status.tsx companion (P0 status)
	tr := kit.NewTransfer()
	tr.SetStatus(kit.TransferStatusWarning)
	tr.SetShowSearch(true)
	layoutTransfer(t, tr)
	if tr.Status != kit.TransferStatusWarning {
		t.Fatal(tr.Status)
	}
}

func TestTransfer_PRD_SelectChange(t *testing.T) {
	tr := kit.NewTransfer()
	tr.SetDataSource(mockTransferData(4))
	var src, tgt []string
	tr.SetOnSelectChange(func(s, t []string) {
		src, tgt = append([]string(nil), s...), append([]string(nil), t...)
	})
	tr.SelectItem(kit.TransferLeft, "1", true)
	if !containsKey(src, "1") {
		t.Fatalf("sourceSelected=%v", src)
	}
	tr.SetTargetKeys([]string{"2"})
	tr.SelectItem(kit.TransferRight, "2", true)
	if !containsKey(tgt, "2") {
		t.Fatalf("targetSelected=%v", tgt)
	}
}

func TestTransfer_PRD_ItemDisabled(t *testing.T) {
	// TF-S8
	tr := kit.NewTransfer()
	data := mockTransferData(3)
	data[1].Disabled = true
	tr.SetDataSource(data)
	tr.SelectItem(kit.TransferLeft, "1", true)
	if containsKey(tr.SourceSelectedKeys(), "1") {
		t.Fatal("disabled item selected")
	}
	tr.SelectAllVisible(kit.TransferLeft)
	if containsKey(tr.SourceSelectedKeys(), "1") {
		t.Fatal("select all included disabled")
	}
	if len(tr.SourceSelectedKeys()) != 2 {
		t.Fatalf("selected=%v", tr.SourceSelectedKeys())
	}
}

func approxTFColor(a, b render.RGBA, tol float64) bool {
	dr := float64(a.R) - float64(b.R)
	dg := float64(a.G) - float64(b.G)
	db := float64(a.B) - float64(b.B)
	da := float64(a.A) - float64(b.A)
	if dr < 0 {
		dr = -dr
	}
	if dg < 0 {
		dg = -dg
	}
	if db < 0 {
		db = -db
	}
	if da < 0 {
		da = -da
	}
	return dr <= tol && dg <= tol && db <= tol && da <= tol
}
