package core

import "fmt"

// PluginHost is a minimal Register/Replace registry for M0 (empty-run ready).
// Full Control/Skin/TokenSet wiring expands in M1+.
type PluginHost struct {
	controls map[string]any
	skins    map[string]any
	tokens   map[string]any
	services map[string]any
}

// NewPluginHost creates an empty registry.
func NewPluginHost() *PluginHost {
	return &PluginHost{
		controls: make(map[string]any),
		skins:    make(map[string]any),
		tokens:   make(map[string]any),
		services: make(map[string]any),
	}
}

// RegisterControl registers a control factory by typeID. Fails if name exists.
func (h *PluginHost) RegisterControl(typeID string, factory any) error {
	return register(h.controls, typeID, factory, false)
}

// ReplaceControl overwrites a control factory.
func (h *PluginHost) ReplaceControl(typeID string, factory any) error {
	return register(h.controls, typeID, factory, true)
}

// Control returns a registered control factory.
func (h *PluginHost) Control(typeID string) (any, bool) {
	v, ok := h.controls[typeID]
	return v, ok
}

// RegisterSkin registers a skin by name. Fails if name exists.
func (h *PluginHost) RegisterSkin(name string, skin any) error {
	return register(h.skins, name, skin, false)
}

// ReplaceSkin overwrites a skin.
func (h *PluginHost) ReplaceSkin(name string, skin any) error {
	return register(h.skins, name, skin, true)
}

// Skin returns a registered skin.
func (h *PluginHost) Skin(name string) (any, bool) {
	v, ok := h.skins[name]
	return v, ok
}

// RegisterTokenSet registers a token set. Fails if name exists.
func (h *PluginHost) RegisterTokenSet(name string, tokens any) error {
	return register(h.tokens, name, tokens, false)
}

// ReplaceTokenSet overwrites a token set.
func (h *PluginHost) ReplaceTokenSet(name string, tokens any) error {
	return register(h.tokens, name, tokens, true)
}

// TokenSet returns a registered token set.
func (h *PluginHost) TokenSet(name string) (any, bool) {
	v, ok := h.tokens[name]
	return v, ok
}

// RegisterService registers a service. Fails if name exists.
func (h *PluginHost) RegisterService(name string, svc any) error {
	return register(h.services, name, svc, false)
}

// ReplaceService overwrites a service.
func (h *PluginHost) ReplaceService(name string, svc any) error {
	return register(h.services, name, svc, true)
}

// Service returns a registered service.
func (h *PluginHost) Service(name string) (any, bool) {
	v, ok := h.services[name]
	return v, ok
}

func register(m map[string]any, name string, v any, replace bool) error {
	if name == "" {
		return fmt.Errorf("plugin: empty name")
	}
	if !replace {
		if _, exists := m[name]; exists {
			return fmt.Errorf("plugin: %q already registered (use Replace)", name)
		}
	}
	m[name] = v
	return nil
}
