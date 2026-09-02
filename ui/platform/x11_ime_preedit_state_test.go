//go:build linux

package platform

// B21/B22 回归单测：preedit 样式段解析 + 守护抖动时保持 IC（输入法状态不丢）。
//
// B21：preedit 无下划线/选中背景——IBus AttrList 按线上 (sa{sv}av) 结构下发，
//      解析层须还原出下划线(type=1)/前景(type=2)/背景(type=3)段，不得退化为 singleSegment。
// B22：切回程序任务栏显示英文——实测 Destroy 后重建 IC 会让 fcitx5 状态回落到
//      keyboard-us；守护抖动时须复用现有 IC，不得销毁重建。

import (
	"context"
	"testing"
	"time"

	"github.com/godbus/dbus/v5"
)

// buildIBusTextVariant 复刻真机 fcitx5/ibus 下发的 IBusText 结构：
// (sa{sv}sv) ["IBusText", {}, text, <(sa{sv}av) ["IBusAttrList", {}, [IBusAttribute...]>]]
func buildIBusTextVariant(t *testing.T, text string, attrs []ibusAttr) dbus.Variant {
	t.Helper()
	attrVariants := make([]dbus.Variant, 0, len(attrs))
	for _, a := range attrs {
		attrVariants = append(attrVariants, dbus.MakeVariant(
			struct {
				Name  string
				Props map[string]dbus.Variant
				Type  uint32
				Value uint32
				Start uint32
				End   uint32
			}{"IBusAttribute", map[string]dbus.Variant{}, a.Type, a.Value, a.Start, a.End},
		))
	}
	attrList := dbus.MakeVariant(
		struct {
			Name  string
			Props map[string]dbus.Variant
			Attrs []dbus.Variant
		}{"IBusAttrList", map[string]dbus.Variant{}, attrVariants},
	)
	ibusText := struct {
		Name  string
		Props map[string]dbus.Variant
		Text  string
		Attrs dbus.Variant
	}{"IBusText", map[string]dbus.Variant{}, text, attrList}
	return dbus.MakeVariant(ibusText)
}

// TestX11PreeditAttrsParsed 锁死 B21：属性须被还原，不得退化为整段单一样式。
func TestX11PreeditAttrsParsed(t *testing.T) {
	// 真机实测 fcitx5 拼音 preedit "ni" 下发的 4 条属性
	attrs := []ibusAttr{
		{Type: 1, Value: 1, Start: 0, End: 2},        // 下划线 single [0,2)
		{Type: 2, Value: 0xffffff, Start: 0, End: 2}, // 前景色
		{Type: 3, Value: 0, Start: 0, End: 2},        // 背景色（选中反显）
		{Type: 1, Value: 1, Start: 2, End: 2},        // 光标段，空区间应被丢弃
	}
	v := buildIBusTextVariant(t, "ni", attrs)

	gotText, segs := DecodeIBusVariant(v)
	if gotText != "ni" {
		t.Fatalf("text = %q, want \"ni\"", gotText)
	}

	var hasUnderline, hasSelected bool
	for _, s := range segs {
		switch s.Attr {
		case ImeAttrUnderline:
			hasUnderline = true
		case ImeAttrSelected:
			hasSelected = true
		}
	}
	if len(segs) == 1 && segs[0].Attr == ImeAttrHighlight {
		t.Fatalf("B21 回归：属性被丢弃，退化为 singleSegment %+v", segs)
	}
	if !hasUnderline {
		t.Fatalf("B21 回归：未还原下划线段，segs=%+v", segs)
	}
	if !hasSelected {
		t.Fatalf("B21 回归：未还原选中背景段，segs=%+v", segs)
	}
	t.Logf("B21 OK: segs=%+v", segs)
}

// TestX11PreeditNoAttrsFallsBack 无属性时退化为整段高亮（不崩、不空）。
func TestX11PreeditNoAttrsFallsBack(t *testing.T) {
	v := buildIBusTextVariant(t, "ni", nil)
	text, segs := DecodeIBusVariant(v)
	if text != "ni" {
		t.Fatalf("text = %q, want \"ni\"", text)
	}
	if len(segs) == 0 {
		t.Fatalf("无属性时也应给出整段默认样式，segs 为空")
	}
	t.Logf("fallback segs=%+v", segs)
}

// TestX11FcitxPreeditUnderline 五笔等只下发下划线的场景。
func TestX11FcitxPreeditUnderline(t *testing.T) {
	attrs := []ibusAttr{{Type: 1, Value: 1, Start: 0, End: 2}}
	v := buildIBusTextVariant(t, "wt", attrs)
	_, segs := DecodeIBusVariant(v)
	found := false
	for _, s := range segs {
		if s.Attr == ImeAttrUnderline {
			found = true
		}
	}
	if !found {
		t.Fatalf("应还原出下划线段，segs=%+v", segs)
	}
	t.Logf("wbx segs=%+v", segs)
}

// TestX11OwnerChangeKeepsInputContext 锁死 B22：
// 守护「有→空」时不得销毁 IC、不得清空 objectPath、不得发 FocusOut。
// 否则重建 IC 会让输入法状态回落英文。
func TestX11OwnerChangeKeepsInputContext(t *testing.T) {
	conn, err := sharedFcitxConn()
	if err != nil || conn == nil {
		t.Skipf("无会话总线：%v", err)
	}
	if has, _ := dbusHasOwner(conn, dbusServiceIBus); !has {
		t.Skip("本机无 org.freedesktop.IBus，跳过")
	}

	host := &x11Host{st: &x11State{}}
	im := &x11Ime{conn: conn, host: host}
	x11RegisterIme(im)
	defer func() {
		x11UnregisterIme(im)
		im.Close()
	}()

	// 建立 IC
	if err := im.ensureReprobeReady(2 * time.Second); err != nil {
		t.Fatalf("建 IC 失败：%v", err)
	}
	before := im.ObjectPath()
	if before == "" {
		t.Fatalf("IC 未建立")
	}

	// 模拟守护「有→空」抖动
	x11MarkDirtyForOwnerChange(dbusServiceIBus, ":1.123", "")

	if got := im.ObjectPath(); got != before {
		t.Fatalf("B22 回归：守护抖动后 objectPath 被清空/重建（%q -> %q），会导致输入法状态回落英文", before, got)
	}
	if !im.imeDirty {
		t.Fatalf("B22：抖动后应标记 imeDirty 以便下次重探")
	}

	// 守护仍在时，ensureReprobe 应复用 IC 而非重建
	im.ensureReprobe()
	if got := im.ObjectPath(); got != before {
		t.Fatalf("B22 回归：守护存活时不应重建 IC（%q -> %q）", before, got)
	}
	if im.imeDirty {
		t.Fatalf("B22：复用 IC 后应清除脏标记")
	}
	t.Logf("B22 OK: IC %s 在守护抖动后保持不变", before)
}

// ensureReprobeReady 等待 asyncProbe 建好 IC，便于测试断言。
func (im *x11Ime) ensureReprobeReady(timeout time.Duration) error {
	go im.asyncProbe()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if im.ObjectPath() != "" {
			return nil
		}
		time.Sleep(20 * time.Millisecond)
	}
	return context.DeadlineExceeded
}
