package input

// IMEKind classifies an in-progress input-method session event.
type IMEKind int

const (
	// IMECompose: the pre-edit text changed (pinyin romanization, candidates
	// still pending). Text holds the current pre-edit string.
	IMECompose IMEKind = iota
	// IMECommit: a composition finished; Text holds the committed string.
	IMECommit
	// IMECaretMove: the text-editing cursor/selection moved (from on-screen
	// keyboards / IME UI), Start/End delimit the affected range.
	IMECaretMove
	// IMEDeleteSurrounding: the IME asks to remove text around the caret
	// (zwp_text_input_v3 delete_surrounding_text). Start carries -before and
	// End carries +after, both byte counts relative to the caret.
	IMEDeleteSurrounding
)

func (k IMEKind) String() string {
	switch k {
	case IMECompose:
		return "compose"
	case IMECommit:
		return "commit"
	case IMECaretMove:
		return "caret"
	case IMEDeleteSurrounding:
		return "delete-surrounding"
	default:
		return "unknown"
	}
}

// IMEEvent is a normalized IME session event.
type IMEEvent struct {
	Kind  IMEKind
	Text  string // compose pre-edit / committed text
	Start int    // compose: caret byte offset within Text (<0 = end)
	End   int    // delete-surrounding: +after bytes (D3: no negative magic)
}

// PreeditEvent is the rich composition update from the platform adapter
// path (design §4.1). Segments carry IME attributes in display offsets.
type PreeditEvent struct {
	Text     string
	Cursor   int       // caret byte offset within Text (<0 = end)
	Segments []Segment // nil = default underline styling
}

// Segment marks a styled range within pre-edit text.
type Segment struct {
	Start, End int // byte offsets into the pre-edit text
	Attr       uint8
}

// Session event kind (design §4.1): engine-side activation news.
const (
	// IMESession reports session activation; Start=1 active / 0 inactive.
	IMESession IMEKind = iota + 100
)

// TextEvent is committed text input destined for a focused editable control:
// printable keyboard chars, paste, and IME commits all arrive here.
type TextEvent struct {
	Text string
}
