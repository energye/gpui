package semantics

// Role is a coarse widget role (ARIA-inspired subset, not full ARIA).
type Role string

const (
	RoleNone       Role = ""
	RoleButton     Role = "button"
	RoleText       Role = "text"
	RoleImage      Role = "image"
	RoleList       Role = "list"
	RoleListItem   Role = "listitem"
	RoleTextField  Role = "textfield"
	RoleCheckbox   Role = "checkbox"
	RoleSwitch     Role = "switch"
	RoleDialog     Role = "dialog"
	RoleNavigation Role = "navigation"
	RoleGeneric    Role = "generic"
)

// Node is one semantics record (parallel to, or embedded by, a widget).
type Node struct {
	Role  Role
	Label string
	// Value optional current value (switch on/off, progress, …).
	Value string
	// Focusable hints that the node participates in focus (informational).
	Focusable bool
	// Children for a lightweight parallel tree.
	Children []*Node
}

// New creates a node with role and label.
func New(role Role, label string) *Node {
	return &Node{Role: role, Label: label}
}

// Add appends a child and returns the child.
func (n *Node) Add(child *Node) *Node {
	if n == nil || child == nil {
		return child
	}
	n.Children = append(n.Children, child)
	return child
}

// Flat is one row of a flattened export (debug / tests).
type Flat struct {
	Depth int
	Role  Role
	Label string
	Value string
}

// Flatten walks the tree pre-order into a flat list.
func Flatten(root *Node) []Flat {
	if root == nil {
		return nil
	}
	var out []Flat
	var walk func(n *Node, depth int)
	walk = func(n *Node, depth int) {
		if n == nil {
			return
		}
		out = append(out, Flat{Depth: depth, Role: n.Role, Label: n.Label, Value: n.Value})
		for _, ch := range n.Children {
			walk(ch, depth+1)
		}
	}
	walk(root, 0)
	return out
}
