package painting_test

import (
	"testing"

	"github.com/energye/gpui/ui/painting"
)

func TestSaveLayerBudget(t *testing.T) {
	b := &painting.SaveLayerBudget{MaxOps: 2, MaxArea: 1000}
	if !b.Allow(10, 10) {
		t.Fatal("first")
	}
	if !b.Allow(10, 10) {
		t.Fatal("second")
	}
	if b.Allow(10, 10) {
		t.Fatal("third op should fail")
	}
	b.Reset()
	if !b.Allow(10, 10) {
		t.Fatal("after reset")
	}
	b2 := &painting.SaveLayerBudget{MaxArea: 50}
	if b2.Allow(10, 10) { // 100 > 50
		t.Fatal("area")
	}
}
