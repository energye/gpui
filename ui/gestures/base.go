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
