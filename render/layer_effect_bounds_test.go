//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package render

import (
	"image"
	"testing"
)

// markLayerEffect bounds a filter layer's composite to its content damage
// expanded by the filter's spatial spread. A full-surface composite would
// paint stale pixels from a pooled layer RT at the filter layer's opacity
// (the blur -> opacity A -> B -> C bug: B became opaque because the blur's
// full blit carried a previous layer's pixels at B's rect).
func TestMarkLayerEffect_BoundsDamage(t *testing.T) {
	c := &Context{layerStack: newLayerStack()}
	layer := &Layer{damage: image.Rect(10, 10, 30, 30)}
	c.layerStack.layers = append(c.layerStack.layers, layer)

	c.markLayerEffect(6)
	if layer.fullComposite {
		t.Fatal("non-empty damage must not force a full-surface composite")
	}
	if want := image.Rect(4, 4, 36, 36); layer.damage != want {
		t.Fatalf("damage=%v want %v (pad 6)", layer.damage, want)
	}
}

// Unknown damage keeps the conservative full-surface composite.
func TestMarkLayerEffect_EmptyDamageFallsBackFull(t *testing.T) {
	c := &Context{layerStack: newLayerStack()}
	layer := &Layer{}
	c.layerStack.layers = append(c.layerStack.layers, layer)

	c.markLayerEffect(6)
	if !layer.fullComposite {
		t.Fatal("empty damage must fall back to a full-surface composite")
	}
}
