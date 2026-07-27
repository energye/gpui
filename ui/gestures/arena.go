package gestures

import "github.com/energye/gpui/ui/platform"

// entryState is the resolution state of one arena member.
type entryState int

const (
	entryPending entryState = iota
	entryAccepted
	entryRejected
)

type arenaEntry struct {
	rec   GestureRecognizer
	state entryState
}

// GestureArena is one competition for a single pointer (Flutter GestureArena subset).
//
// MVP: single winner. When a member Accepts, all others are Rejected.
// When all but one have Rejected and the arena is closed, the last pending
// member is auto-accepted (sweep).
type GestureArena struct {
	PointerID int
	entries   []*arenaEntry
	open      bool // still accepting Add
	resolved  bool
	winner    GestureRecognizer
}

func newArena(pointerID int) *GestureArena {
	return &GestureArena{PointerID: pointerID, open: true}
}

// Add registers a recognizer as pending. No-op if resolved or rec nil.
func (a *GestureArena) Add(rec GestureRecognizer) {
	if a == nil || rec == nil || a.resolved {
		return
	}
	for _, e := range a.entries {
		if e.rec == rec {
			return
		}
	}
	a.entries = append(a.entries, &arenaEntry{rec: rec, state: entryPending})
	if c, ok := rec.(arenaClient); ok {
		c.setArena(a)
	}
}

// Close stops accepting new members. If exactly one pending remains, it wins.
func (a *GestureArena) Close() {
	if a == nil || a.resolved {
		return
	}
	a.open = false
	a.sweep()
}

// Accept resolves the arena in favor of rec.
func (a *GestureArena) Accept(rec GestureRecognizer) {
	if a == nil || rec == nil || a.resolved {
		return
	}
	var found *arenaEntry
	for _, e := range a.entries {
		if e.rec == rec {
			found = e
			break
		}
	}
	if found == nil || found.state == entryRejected {
		return
	}
	a.resolved = true
	a.open = false
	a.winner = rec
	found.state = entryAccepted
	for _, e := range a.entries {
		if e.rec == rec {
			continue
		}
		if e.state == entryPending {
			e.state = entryRejected
			e.rec.Reject()
		}
	}
	rec.Accept()
}

// Reject marks rec as rejected and may auto-accept the last pending member.
func (a *GestureArena) Reject(rec GestureRecognizer) {
	if a == nil || rec == nil || a.resolved {
		return
	}
	for _, e := range a.entries {
		if e.rec != rec {
			continue
		}
		if e.state != entryPending {
			return
		}
		e.state = entryRejected
		e.rec.Reject()
		a.sweep()
		return
	}
}

func (a *GestureArena) sweep() {
	if a == nil || a.resolved {
		return
	}
	var pending []*arenaEntry
	for _, e := range a.entries {
		if e.state == entryPending {
			pending = append(pending, e)
		}
	}
	if len(pending) == 1 && !a.open {
		a.Accept(pending[0].rec)
	}
	if len(pending) == 0 {
		a.resolved = true
		a.open = false
	}
}

// Winner returns the accepted recognizer, or nil.
func (a *GestureArena) Winner() GestureRecognizer {
	if a == nil {
		return nil
	}
	return a.winner
}

// Resolved reports whether the arena has a final outcome.
func (a *GestureArena) Resolved() bool {
	return a != nil && a.resolved
}

// MemberCount returns how many recognizers joined.
func (a *GestureArena) MemberCount() int {
	if a == nil {
		return 0
	}
	return len(a.entries)
}

// PendingCount returns pending members.
func (a *GestureArena) PendingCount() int {
	if a == nil {
		return 0
	}
	n := 0
	for _, e := range a.entries {
		if e.state == entryPending {
			n++
		}
	}
	return n
}

// GestureArenaManager owns arenas keyed by pointer id.
type GestureArenaManager struct {
	arenas map[int]*GestureArena
}

// NewManager creates an empty manager.
func NewManager() *GestureArenaManager {
	return &GestureArenaManager{arenas: make(map[int]*GestureArena)}
}

// Arena returns the open or active arena for pointerID, creating one if needed.
func (m *GestureArenaManager) Arena(pointerID int) *GestureArena {
	if m == nil {
		return nil
	}
	if pointerID == 0 {
		pointerID = PrimaryPointerID
	}
	if m.arenas == nil {
		m.arenas = make(map[int]*GestureArena)
	}
	if a, ok := m.arenas[pointerID]; ok && a != nil && !a.resolved {
		return a
	}
	a := newArena(pointerID)
	m.arenas[pointerID] = a
	return a
}

// Get returns the arena for pointerID if present.
func (m *GestureArenaManager) Get(pointerID int) *GestureArena {
	if m == nil || m.arenas == nil {
		return nil
	}
	if pointerID == 0 {
		pointerID = PrimaryPointerID
	}
	return m.arenas[pointerID]
}

// CloseArena closes the arena for pointerID (end of down routing).
func (m *GestureArenaManager) CloseArena(pointerID int) {
	if a := m.Get(pointerID); a != nil {
		a.Close()
	}
}

// Route delivers e to all relevant members of the pointer's arena.
func (m *GestureArenaManager) Route(e PointerEvent) {
	if m == nil {
		return
	}
	id := e.EffectivePointerID()
	a := m.Get(id)
	if a == nil {
		return
	}
	ents := append([]*arenaEntry(nil), a.entries...)
	for _, ent := range ents {
		if ent.state == entryRejected {
			continue
		}
		if a.resolved && ent.rec != a.winner {
			continue
		}
		ent.rec.HandleEvent(e)
	}
	if e.Kind == platform.PointerUp {
		m.finish(id)
	}
}

// finish removes a finished arena so members do not leak.
func (m *GestureArenaManager) finish(pointerID int) {
	if m == nil || m.arenas == nil {
		return
	}
	a := m.arenas[pointerID]
	if a == nil {
		return
	}
	if !a.resolved {
		a.open = false
		for _, e := range a.entries {
			if e.state == entryPending {
				e.state = entryRejected
				e.rec.Reject()
			}
		}
		a.resolved = true
	}
	delete(m.arenas, pointerID)
}

// ActiveCount is the number of live arenas (test helper).
func (m *GestureArenaManager) ActiveCount() int {
	if m == nil || m.arenas == nil {
		return 0
	}
	return len(m.arenas)
}
