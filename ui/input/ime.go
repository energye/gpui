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
)

func (k IMEKind) String() string {
	switch k {
	case IMECompose:
		return "compose"
	case IMECommit:
		return "commit"
	case IMECaretMove:
		return "caret"
	default:
		return "unknown"
	}
}

// IMEEvent is a normalized IME session event.
type IMEEvent struct {
	Kind  IMEKind
	Text  string // compose pre-edit / committed text
	Start int    // text range start (bytes) affected by this event
	End   int    // text range end (bytes); -1 = whole buffer
}

// TextEvent is committed text input destined for a focused editable control:
// printable keyboard chars, paste, and IME commits all arrive here.
type TextEvent struct {
	Text string
}