package kit

// Mentions is a simple @-mention input (value + options list).
// https://ant.design/components/mentions
type Mentions struct {
	*AutoComplete
}

// NewMentions creates a mentions field.
func NewMentions(placeholder string, users ...string) *Mentions {
	opts := make([]string, len(users))
	for i, u := range users {
		opts[i] = "@" + u
	}
	return &Mentions{AutoComplete: NewAutoComplete(placeholder, opts...)}
}
