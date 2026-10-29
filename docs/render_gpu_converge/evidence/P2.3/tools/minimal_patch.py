#!/usr/bin/env python3
"""P2.3 minimal-survival patch, applied AFTER deleting the 10 cluster files.

Reproducible record of the deletion experiment (2026-10-29). Usage:

    python3 minimal_patch.py <repo-or-worktree-root>

Effects (all behaviour-neutral):
  1. device.go      + slim Backend (device/queue/initialized + IsInitialized/Device/Queue/Close)
  2. commands.go    + StubPipelineID / StubComputePipelineID / StubBindGroupID / StubBufferID
  3. gpu_texture.go - GPUTexture.manager field, SetMemoryManager, Close() manager branch
"""
import sys
import os

root = sys.argv[1]
pkg = os.path.join(root, "render/internal/gpu")

# 1) device.go: sync import + slim Backend
p = os.path.join(pkg, "device.go")
s = open(p, encoding="utf-8").read()
s = s.replace('import (\n\t"fmt"\n\t"time"\n', 'import (\n\t"fmt"\n\t"sync"\n\t"time"\n')
s += '''
// Backend holds the device/queue pair shared by the legacy core wrappers
// (command encoder, pixmap texture, atlas). Device creation itself lives in
// the accelerator paths; this type never creates an instance.
type Backend struct {
	mu sync.RWMutex

	device      hal.Device
	queue       hal.Queue
	initialized bool
}

// IsInitialized returns true if the backend holds a live device.
func (b *Backend) IsInitialized() bool {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.initialized
}

// Device returns the GPU device.
func (b *Backend) Device() hal.Device {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.device
}

// Queue returns the GPU queue.
func (b *Backend) Queue() hal.Queue {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.queue
}

// Close releases the device and clears the backend state.
func (b *Backend) Close() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if !b.initialized {
		return
	}
	if b.device != nil {
		if err := releaseDevice(b.device); err != nil {
			slogger().Warn("error releasing device", "err", err)
		}
		b.device = nil
	}
	b.queue = nil
	b.initialized = false
}
'''
open(p, "w", encoding="utf-8").write(s)

# 2) commands.go: stub id types
p = os.path.join(pkg, "commands.go")
s = open(p, encoding="utf-8").read()
anchor = '''// StubCommandBufferID is a legacy logical marker for command-buffer tests.
type StubCommandBufferID uint64
'''
add = '''
// StubPipelineID is a legacy logical marker for render-pipeline tests.
type StubPipelineID uint64

// StubComputePipelineID is a legacy logical marker for compute-pipeline tests.
type StubComputePipelineID uint64

// StubBindGroupID is a legacy logical marker for bind-group tests.
type StubBindGroupID uint64

// StubBufferID is a legacy logical marker for buffer tests.
type StubBufferID uint64
'''
assert anchor in s, "commands.go anchor not found"
open(p, "w", encoding="utf-8").write(s.replace(anchor, anchor + add))

# 3) gpu_texture.go: drop memory manager hook
p = os.path.join(pkg, "gpu_texture.go")
s = open(p, encoding="utf-8").read()
pairs = [
    ('\t// Memory tracking\n\tsizeBytes uint64\n\tmanager   *MemoryManager // optional, for memory tracking\n',
     '\t// Memory tracking\n\tsizeBytes uint64\n'),
    ('// SetMemoryManager sets the memory manager for tracking.\n'
     '// This is called internally when allocating through MemoryManager.\n'
     'func (t *GPUTexture) SetMemoryManager(m *MemoryManager) {\n'
     '\tt.mu.Lock()\n\tdefer t.mu.Unlock()\n\tt.manager = m\n}\n\n', ''),
    ('\tt.mu.Lock()\n\tmanager := t.manager\n\tt.mu.Unlock()\n\n'
     '\t// Notify memory manager if present\n'
     '\tif manager != nil {\n\t\tmanager.unregisterTexture(t)\n\t}\n\n\tt.mu.Lock()',
     '\tt.mu.Lock()'),
    ('\tt.device = nil\n\tt.queue = nil\n\tt.manager = nil\n\tt.mu.Unlock()',
     '\tt.device = nil\n\tt.queue = nil\n\tt.mu.Unlock()'),
]
for old, new in pairs:
    assert old in s, "gpu_texture.go fragment not found"
    s = s.replace(old, new)
open(p, "w", encoding="utf-8").write(s)

print("minimal patch applied to", pkg)
