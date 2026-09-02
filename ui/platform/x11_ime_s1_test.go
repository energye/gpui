//go:build linux

package platform

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestX11ResolveIbusAddress_SkipPolluted(t *testing.T) {
	dir := t.TempDir()
	orig := os.Getenv("IBUS_BUS_DIR")
	os.Setenv("IBUS_BUS_DIR", dir)
	defer os.Setenv("IBUS_BUS_DIR", orig)

	// valid candidate: current pid alive, clean address
	validAddr := "unix:abstract=/tmp/ibus-test-valid,guid=abc"
	validPath := filepath.Join(dir, "valid-bus-file")
	pid := fmt.Sprintf("%d", os.Getpid())
	if err := os.WriteFile(validPath, []byte("IBUS_ADDRESS="+validAddr+"\nIBUS_DAEMON_PID="+pid+"\n"), 0644); err != nil {
		t.Fatalf("write valid: %v", err)
	}
	// ensure valid is newest
	time.Sleep(10 * time.Millisecond)
	_ = os.Chtimes(validPath, time.Now(), time.Now())

	// polluted: contains fcitx, dead pid, older
	pollutedAddr := "unix:abstract=/tmp/fcitx_random_string,guid=xyz"
	pollutedPath := filepath.Join(dir, "polluted-bus-file")
	if err := os.WriteFile(pollutedPath, []byte("IBUS_ADDRESS="+pollutedAddr+"\nIBUS_DAEMON_PID=999999\n# fcitx\n"), 0644); err != nil {
		t.Fatalf("write polluted: %v", err)
	}
	// make polluted older
	past := time.Now().Add(-10 * time.Second)
	_ = os.Chtimes(pollutedPath, past, past)

	// dead pid but clean address
	deadPath := filepath.Join(dir, "dead-bus-file")
	if err := os.WriteFile(deadPath, []byte("IBUS_ADDRESS=unix:abstract=/tmp/ibus-dead,guid=dead\nIBUS_DAEMON_PID=999999\n"), 0644); err != nil {
		t.Fatalf("write dead: %v", err)
	}
	_ = os.Chtimes(deadPath, past, past)

	addr, err := x11ResolveIbusAddress()
	if err != nil {
		t.Fatalf("resolve failed: %v", err)
	}
	if addr != validAddr {
		t.Fatalf("expected valid addr %q got %q (polluted/dead should be skipped)", validAddr, addr)
	}

	// also test FromDir directly skips polluted
	addr2, err := x11ResolveIbusAddressFromDir(dir)
	if err != nil || addr2 != validAddr {
		t.Fatalf("FromDir failed: %v %q", err, addr2)
	}

	// when only polluted remains, should fail
	dir2 := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir2, "only-polluted"), []byte("IBUS_ADDRESS="+pollutedAddr+"\nIBUS_DAEMON_PID=999999\n"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, err := x11ResolveIbusAddressFromDir(dir2); err == nil {
		t.Fatalf("expected error when only polluted, got addr")
	}
}

func TestX11ProbeOrderStrict(t *testing.T) {
	origGtk := os.Getenv("GTK_IM_MODULE")
	origQt := os.Getenv("QT_IM_MODULE")
	origXmod := os.Getenv("XMODIFIERS")
	defer func() {
		os.Setenv("GTK_IM_MODULE", origGtk)
		os.Setenv("QT_IM_MODULE", origQt)
		os.Setenv("XMODIFIERS", origXmod)
	}()

	cases := []struct {
		gtk, qt, xmod string
		want          string
	}{
		{"ibus", "", "", "ibus"},
		{"IBUS", "", "", "ibus"},
		{"fcitx", "", "", "fcitx5"},
		{"fcitx5", "", "", "fcitx5"},
		{"", "ibus", "", "ibus"},
		{"", "fcitx", "", "fcitx5"},
		{"", "", "@im=ibus", "ibus"},
		{"", "", "@im=fcitx", "fcitx5"},
		{"", "", "", "ibus"},
		{"fcitx", "ibus", "", "fcitx5"}, // GTK priority
	}
	for i, c := range cases {
		os.Setenv("GTK_IM_MODULE", c.gtk)
		os.Setenv("QT_IM_MODULE", c.qt)
		os.Setenv("XMODIFIERS", c.xmod)
		got := x11ProbeOrderStrict()
		if len(got) != 1 {
			t.Fatalf("case %d env gtk=%q qt=%q xmod=%q: expected single element, got %v", i, c.gtk, c.qt, c.xmod, got)
		}
		if got[0] != c.want {
			t.Fatalf("case %d env gtk=%q qt=%q xmod=%q: want %q got %q", i, c.gtk, c.qt, c.xmod, c.want, got[0])
		}
		// ensure FlagNoAutoStart single probe semantics: not fallback double
		if len(got) != 1 || (got[0] != "ibus" && got[0] != "fcitx5") {
			t.Fatalf("case %d: probe should be single ibus or fcitx5, got %v", i, got)
		}
	}
}

func TestX11S1BusSingleton(t *testing.T) {
	// 无守护时降级 nil 不阻塞（验证建窗不阻塞语义）
	// 共享连接单例：多次调用返回同一指针或同为 nil
	c1, err1 := sharedFcitxConn()
	c2, err2 := sharedFcitxConn()
	if (c1 == nil) != (c2 == nil) {
		t.Fatalf("fcitx singleton mismatch nil: %v vs %v err1 %v err2 %v", c1, c2, err1, err2)
	}
	if c1 != nil && c1 != c2 {
		t.Fatalf("fcitx conn not reused %p vs %p", c1, c2)
	}
	// ibus private 在无地址时应快速失败不阻塞（500ms 内）
	start := time.Now()
	// 使用临时空目录确保无地址
	dir := t.TempDir()
	orig := os.Getenv("IBUS_BUS_DIR")
	os.Setenv("IBUS_BUS_DIR", dir)
	defer os.Setenv("IBUS_BUS_DIR", orig)
	resetIbusPrivateForTest()
	_, _ = dialIbusPrivate(false)
	elapsed := time.Since(start)
	if elapsed > 500*time.Millisecond {
		t.Fatalf("ibus dial should not block >500ms, took %v", elapsed)
	}
	resetIbusPrivateForTest()
}
