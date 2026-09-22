package kit

import (
	"fmt"
	"sync"

	"github.com/energye/gpui/ui/theme"
)

// iconSource holds one offline iconfont source (name -> painter key).
type iconSource struct {
	id    string
	icons map[string]string
}

var iconRegistryMu sync.Mutex
var iconSources []iconSource

// iconGlobalTwoTone defaults to the theme primary seed (antd
// setTwoToneColor default follows colorPrimary); always overridable
// via SetTwoToneColorGlobal, never a second brand source.
var iconGlobalTwoTone = iconDefaultTwoTone()

func iconDefaultTwoTone() string {
	c := theme.DefaultTokens().ColorPrimary
	return fmt.Sprintf("#%02x%02x%02x",
		uint8(c.R*255+0.5), uint8(c.G*255+0.5), uint8(c.B*255+0.5))
}

// RegisterIconSource adds an offline source; later sources override
// same names (matches antd multi scriptUrl order). Same-source
// registers merge (later keys win, earlier keys survive).
func RegisterIconSource(sourceID string, icons map[string]string) {
	iconRegistryMu.Lock()
	defer iconRegistryMu.Unlock()
	for i, s := range iconSources {
		if s.id == sourceID {
			merged := make(map[string]string, len(s.icons)+len(icons))
			for k, v := range s.icons {
				merged[k] = v
			}
			for k, v := range icons {
				merged[k] = v
			}
			iconSources[i] = iconSource{id: sourceID, icons: merged}
			return
		}
	}
	cp := make(map[string]string, len(icons))
	for k, v := range icons {
		cp[k] = v
	}
	iconSources = append(iconSources, iconSource{id: sourceID, icons: cp})
}

// lookupIconSource resolves typeName across sources (last wins).
func lookupIconSource(typeName string) (string, bool) {
	iconRegistryMu.Lock()
	defer iconRegistryMu.Unlock()
	for i := len(iconSources) - 1; i >= 0; i-- {
		if v, ok := iconSources[i].icons[typeName]; ok {
			return v, true
		}
	}
	return "", false
}

// resetIconSources clears offline sources (tests only).
func resetIconSources() {
	iconRegistryMu.Lock()
	defer iconRegistryMu.Unlock()
	iconSources = nil
}

// IconfontOptions carries offline source ids (maps scriptUrl[] order).
type IconfontOptions struct {
	Sources []string
}

// IconfontFamily is the offline family handle.
type IconfontFamily struct {
	options IconfontOptions
}

// CreateFromIconfont builds the offline family (never fetches network).
func CreateFromIconfont(opts IconfontOptions) *IconfontFamily {
	return &IconfontFamily{options: opts}
}

// Register binds typeName to a painter key in the named source.
func (f *IconfontFamily) Register(sourceID, typeName, painterKey string) {
	RegisterIconSource(sourceID, map[string]string{typeName: painterKey})
}

// NewIcon builds props resolving typeName via offline sources.
func (f *IconfontFamily) NewIcon(typeName string) IconProps {
	if key, ok := lookupIconSource(typeName); ok {
		return IconProps{Name: typeName, CustomPainter: key, CustomPainterID: key, Decorative: true}
	}
	return IconProps{Name: typeName, Decorative: true}
}

// SetTwoToneColorGlobal sets the global double-color default.
func SetTwoToneColorGlobal(c string) {
	iconRegistryMu.Lock()
	defer iconRegistryMu.Unlock()
	if c != "" {
		iconGlobalTwoTone = c
	}
}

// GetTwoToneColorGlobal returns the global double-color default.
func GetTwoToneColorGlobal() string {
	iconRegistryMu.Lock()
	defer iconRegistryMu.Unlock()
	return iconGlobalTwoTone
}

// parseIconColor parses #rgb/#rrggbb; invalid yields zero color.
func parseIconColor(s string) theme.Color {
	if s == "" {
		return theme.Color{}
	}
	return theme.Hex(s)
}
