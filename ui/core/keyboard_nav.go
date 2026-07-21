package core

// KeyboardNavMode for arrow-key navigation within a list/menu.
type KeyboardNavMode int

const (
	NavVertical KeyboardNavMode = iota
	NavHorizontal
	NavBoth
)

// KeyboardNav tracks an active index among focusable/selectable items (C-KeyboardNav).
type KeyboardNav struct {
	Mode  KeyboardNavMode
	Index int
	Count int
	// Wrap around ends.
	Wrap bool
	// OnIndexChange fires when Index changes via keys.
	OnIndexChange func(index int)
}

// NewKeyboardNav creates a nav controller.
func NewKeyboardNav(mode KeyboardNavMode, count int) *KeyboardNav {
	return &KeyboardNav{Mode: mode, Count: count, Wrap: true, Index: 0}
}

// SetCount updates item count and clamps index.
func (k *KeyboardNav) SetCount(n int) {
	k.Count = n
	if k.Index >= n {
		k.Index = n - 1
	}
	if k.Index < 0 {
		k.Index = 0
	}
}

// Move applies a delta (+1/-1) with optional wrap.
func (k *KeyboardNav) Move(delta int) {
	if k == nil || k.Count <= 0 {
		return
	}
	next := k.Index + delta
	if k.Wrap {
		if next < 0 {
			next = k.Count - 1
		} else if next >= k.Count {
			next = 0
		}
	} else {
		if next < 0 {
			next = 0
		}
		if next >= k.Count {
			next = k.Count - 1
		}
	}
	if next != k.Index {
		k.Index = next
		if k.OnIndexChange != nil {
			k.OnIndexChange(k.Index)
		}
	}
}

// HandleKey processes arrow keys; returns true if handled.
func (k *KeyboardNav) HandleKey(key string) bool {
	if k == nil || k.Count <= 0 {
		return false
	}
	switch key {
	case "ArrowDown", "Down":
		if k.Mode == NavVertical || k.Mode == NavBoth {
			k.Move(1)
			return true
		}
	case "ArrowUp", "Up":
		if k.Mode == NavVertical || k.Mode == NavBoth {
			k.Move(-1)
			return true
		}
	case "ArrowRight", "Right":
		if k.Mode == NavHorizontal || k.Mode == NavBoth {
			k.Move(1)
			return true
		}
	case "ArrowLeft", "Left":
		if k.Mode == NavHorizontal || k.Mode == NavBoth {
			k.Move(-1)
			return true
		}
	case "Home":
		if k.Index != 0 {
			k.Index = 0
			if k.OnIndexChange != nil {
				k.OnIndexChange(0)
			}
		}
		return true
	case "End":
		last := k.Count - 1
		if k.Index != last {
			k.Index = last
			if k.OnIndexChange != nil {
				k.OnIndexChange(last)
			}
		}
		return true
	}
	return false
}
