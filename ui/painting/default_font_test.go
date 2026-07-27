package painting_test

import (
	"testing"

	"github.com/energye/gpui/ui/painting"
)

func TestTryLoadDefaultFace_Smoke(t *testing.T) {
	face, path, err := painting.TryLoadDefaultFace(14)
	if err != nil {
		t.Skipf("no system font: %v", err)
	}
	if face == nil || path == "" {
		t.Fatal("nil face")
	}
	t.Log("font", path)
}
