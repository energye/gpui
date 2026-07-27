package gestures

// GestureRecognizer competes in a GestureArena for a pointer.
//
// Lifecycle:
//
//	AddPointer → (HandleEvent)* → Accept or Reject → Dispose (optional cleanup)
//
// Accept/Reject are called by the arena (or self-request via Arena.Accept/Reject).
// Implementations must be safe to Reject after they already lost.
type GestureRecognizer interface {
	// AddPointer registers interest in pointerID and joins the arena for that down.
	AddPointer(pointerID int, e PointerEvent)
	// HandleEvent receives subsequent move/up/cancel for that pointer while pending or accepted.
	HandleEvent(e PointerEvent)
	// Accept is invoked when this recognizer wins the arena.
	Accept()
	// Reject is invoked when this recognizer loses the arena.
	Reject()
	// Dispose releases resources (idempotent).
	Dispose()
}

// arenaMember is implemented by recognizers that need a back-pointer to the arena.
type arenaClient interface {
	GestureRecognizer
	setArena(a *GestureArena)
	arena() *GestureArena
}
