package kit

// TreeSelect is Select-like using tree node labels (flat list of paths).
// https://ant.design/components/tree-select
type TreeSelect struct {
	*Select
}

// NewTreeSelect creates a select from path labels.
func NewTreeSelect(placeholder string, paths ...string) *TreeSelect {
	opts := make([]SelectOption, len(paths))
	for i, p := range paths {
		opts[i] = SelectOption{Value: p, Label: p}
	}
	return &TreeSelect{Select: NewSelect(placeholder, opts...)}
}
