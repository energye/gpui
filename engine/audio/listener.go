//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package audio

import (
	"sync"

	"github.com/energye/gpui/engine/core"
)

// Listener2D is the 2D hear point (Godot AudioListener2D replay).
//
// Frozen 2026-10-01: NewListener2D, Pos, SetPos, FollowCamera,
// MakeCurrent, ClearCurrent, IsCurrent, CurrentListener,
// MixSource, MixCurrent, MixStereo. Additive changes only.
//
// Only core numbers are used; nothing is played here. The listener
// holds a position; MakeCurrent publishes it as the single current
// hear point (Godot make_current singleton). Positional mixes read
// the current position, so the window keeps one listener glued to
// the camera and mixes through it. Nil receivers never panic.
type Listener2D struct {
	pos core.Vec2
	id  uint64
}

var (
	listenerMu      sync.RWMutex
	listenerSeq     uint64
	listenerCurrent uint64
	listenerPos     core.Vec2
	listenerHas     bool
)

// NewListener2D builds a listener at pos. Non-finite pos is InvalidArg.
func NewListener2D(pos core.Vec2) (Listener2D, error) {
	if !finiteVec(pos) {
		return Listener2D{}, core.InvalidArg("audio.NewListener2D", "pos")
	}
	listenerMu.Lock()
	listenerSeq++
	id := listenerSeq
	listenerMu.Unlock()
	return Listener2D{pos: pos, id: id}, nil
}

// Pos returns the listener position.
func (l Listener2D) Pos() core.Vec2 { return l.pos }

// SetPos moves the listener. Non-finite input is InvalidArg and moves
// nothing. When this listener is current, the published hear point
// moves with it so the next mix hears the new spot.
func (l *Listener2D) SetPos(p core.Vec2) error {
	if l == nil {
		return core.InvalidArg("audio.SetPos", "listener")
	}
	if !finiteVec(p) {
		return core.InvalidArg("audio.SetPos", "pos")
	}
	l.pos = p
	listenerMu.Lock()
	if l.id != 0 && listenerCurrent == l.id {
		listenerPos = p
	}
	listenerMu.Unlock()
	return nil
}

// FollowCamera glues the listener to the camera center: pos = cameraPos.
// Non-finite input is InvalidArg and moves nothing. The caller passes
// the camera effective center each frame; no camera type is imported.
func (l *Listener2D) FollowCamera(cameraPos core.Vec2) error {
	if l == nil {
		return core.InvalidArg("audio.FollowCamera", "listener")
	}
	if !finiteVec(cameraPos) {
		return core.InvalidArg("audio.FollowCamera", "cameraPos")
	}
	return l.SetPos(cameraPos)
}

// MakeCurrent publishes this listener as the single hear point.
// The previous current listener drops out silently. Nil is a no-op.
// Zero-value listeners take an id on first publish so copies share it.
func (l *Listener2D) MakeCurrent() {
	if l == nil {
		return
	}
	listenerMu.Lock()
	if l.id == 0 {
		listenerSeq++
		l.id = listenerSeq
	}
	listenerCurrent = l.id
	listenerPos = l.pos
	listenerHas = true
	listenerMu.Unlock()
}

// ClearCurrent withdraws this listener when it is current. Other
// listeners are untouched. Nil is a no-op.
func (l *Listener2D) ClearCurrent() {
	if l == nil {
		return
	}
	listenerMu.Lock()
	if listenerCurrent == l.id {
		listenerCurrent = 0
		listenerHas = false
	}
	listenerMu.Unlock()
}

// IsCurrent reports whether this listener is the published hear point.
func (l Listener2D) IsCurrent() bool {
	listenerMu.RLock()
	defer listenerMu.RUnlock()
	return l.id != 0 && listenerCurrent == l.id
}

// CurrentListener returns the published hear point copy plus true,
// or zero plus false when nobody is current.
func CurrentListener() (Listener2D, bool) {
	listenerMu.RLock()
	defer listenerMu.RUnlock()
	if !listenerHas || listenerCurrent == 0 {
		return Listener2D{}, false
	}
	return Listener2D{pos: listenerPos, id: listenerCurrent}, true
}

// MixSource mixes one source heard at this listener position.
func (l Listener2D) MixSource(s PosSound) (Mix, bool) {
	return s.Mix(l.pos)
}

// MixCurrent mixes every source heard at the current listener in order.
// Nobody current returns nil,false. Bad slots stay silent per MixAll.
func MixCurrent(sources []PosSound) ([]Mix, bool) {
	listenerMu.RLock()
	pos, has := listenerPos, listenerHas && listenerCurrent != 0
	listenerMu.RUnlock()
	if !has {
		return nil, false
	}
	return MixAll(pos, sources)
}

// MixStereo mixes one source at this listener and spreads one mono
// sample into left/right. ok reports the mix validity, not audibility:
// far-but-valid stays ok=true with 0,0 samples.
func (l Listener2D) MixStereo(s PosSound, mono float64) (left, right float64, ok bool) {
	m, ok := s.Mix(l.pos)
	if !ok {
		return 0, 0, false
	}
	left, right = Stereo(mono, m)
	return left, right, true
}
