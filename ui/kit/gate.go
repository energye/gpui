package kit

import (
	"github.com/energye/gpui/ui/kit/internal/gate"
)

// Public gate re-exports for examples/kit_f0_* windows. Product code
// imports internal/gate directly; examples import kit.

type (
	// GatePropsCoverage is the window-reported Props audit.
	GatePropsCoverage = gate.PropsCoverage
	// GateStateFlow is the window-reported state audit.
	GateStateFlow = gate.StateFlow
	// GateTokenUse is the window-reported theme audit.
	GateTokenUse = gate.TokenUse
)

// GateCheckProps reports whether every documented param is covered.
func GateCheckProps(c GatePropsCoverage) (bool, []string) { return gate.CheckProps(c) }

// GateCheckState reports whether every claimed state resolves.
func GateCheckState(f GateStateFlow) bool { return gate.CheckState(f) }

// GateCheckWiring reports whether imports avoid direct render/gpu.
func GateCheckWiring(imports []string) (bool, []string) { return gate.CheckWiring(imports) }

// GateCheckToken passes only when values come from theme.
func GateCheckToken(u GateTokenUse) bool { return gate.CheckToken(u) }
