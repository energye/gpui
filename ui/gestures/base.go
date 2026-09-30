//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package gestures

// baseRecognizer holds arena back-pointer and win/lose flags shared by Tap/Pan.
type baseRecognizer struct {
	a        *GestureArena
	accepted bool
	rejected bool
	disposed bool
}

func (b *baseRecognizer) setArena(a *GestureArena) { b.a = a }
func (b *baseRecognizer) arena() *GestureArena     { return b.a }

func (b *baseRecognizer) isDead() bool {
	return b == nil || b.disposed || b.rejected
}
