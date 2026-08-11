package platform

import "testing"

func TestPlatformKindValuesStable(t *testing.T) {
	if PlatformX11 != 0 {
		t.Fatalf("PlatformX11 = %d, want 0 (stable ABI)", PlatformX11)
	}
	if PlatformWayland != 1 || PlatformWin32 != 2 || PlatformAppKit != 3 {
		t.Fatalf("kind values shifted: %d %d %d", PlatformWayland, PlatformWin32, PlatformAppKit)
	}
	if PlatformNone != -1 {
		t.Fatalf("PlatformNone = %d, want -1", PlatformNone)
	}
}
