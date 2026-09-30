//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

// TrueType bytecode interpreter — function/instruction definitions.
//
// Manages FDEF/IDEF function and instruction definitions.
package text

// ttDefinition describes a function or instruction definition.
// The key is either a function number (FDEF) or opcode (IDEF).
type ttDefinition struct {
	start    int32 // start of code range
	end      int32 // end of code range
	key      int32 // function number or opcode
	prog     ttProgramType
	isActive bool
}

// program returns the program that contains this definition.
func (d *ttDefinition) program() ttProgramType {
	return d.prog
}

// ttDefinitionMap maps function numbers or opcodes to definitions.
type ttDefinitionMap struct {
	defs     []ttDefinition
	readonly bool
}

// newTTDefinitionMap creates a mutable definition map with the given capacity.
func newTTDefinitionMap(capacity int) ttDefinitionMap {
	return ttDefinitionMap{
		defs:     make([]ttDefinition, capacity),
		readonly: false,
	}
}

// newTTDefinitionMapReadonly creates a read-only definition map.
func newTTDefinitionMapReadonly(defs []ttDefinition) ttDefinitionMap {
	return ttDefinitionMap{
		defs:     defs,
		readonly: true,
	}
}

// allocate finds or creates a definition slot for the given key.
// Returns the index into the defs slice.
//
// For well-behaved fonts, the key directly maps to the index.
// For IDEF or out-of-range keys, a linear search is used.
func (m *ttDefinitionMap) allocate(key int32) (int, error) {
	if m.readonly {
		return 0, ttErrDefinitionInGlyphProgram
	}
	// Fast path: key fits as direct index.
	if key >= 0 && int(key) < len(m.defs) {
		d := &m.defs[key]
		if !d.isActive || d.key == key {
			d.key = key
			d.isActive = true
			d.prog = ttProgramFont
			d.start = 0
			d.end = 0
			return int(key), nil
		}
	}
	// Slow path: linear search backward for matching key or inactive slot.
	lastInactiveIdx := -1
	for i := len(m.defs) - 1; i >= 0; i-- {
		d := &m.defs[i]
		if d.isActive {
			if d.key == key {
				d.start = 0
				d.end = 0
				d.prog = ttProgramFont
				return i, nil
			}
		} else if lastInactiveIdx == -1 {
			lastInactiveIdx = i
		}
	}
	if lastInactiveIdx == -1 {
		return 0, ttErrTooManyDefinitions
	}
	d := &m.defs[lastInactiveIdx]
	d.key = key
	d.isActive = true
	d.prog = ttProgramFont
	d.start = 0
	d.end = 0
	return lastInactiveIdx, nil
}

// get returns the definition for the given key.
func (m *ttDefinitionMap) get(key int32) (ttDefinition, error) {
	// Fast path: use key as index.
	if key >= 0 && int(key) < len(m.defs) {
		d := &m.defs[key]
		if d.isActive && d.key == key {
			return *d, nil
		}
	}
	// Slow path: linear search.
	for i := len(m.defs) - 1; i >= 0; i-- {
		d := &m.defs[i]
		if d.isActive && d.key == key {
			return *d, nil
		}
	}
	return ttDefinition{}, ttErrInvalidDefinition
}

// reset clears all definitions (if mutable).
func (m *ttDefinitionMap) reset() {
	if m.readonly {
		return
	}
	for i := range m.defs {
		m.defs[i] = ttDefinition{}
	}
}

// ttDefinitionState contains function and instruction definition maps.
type ttDefinitionState struct {
	functions    ttDefinitionMap
	instructions ttDefinitionMap
}
