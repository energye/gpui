package kit

import (
	"sync"

	"github.com/energye/gpui/ui/kit/internal/scope"
)

// ModalHostResult is the promise outcome: true means OK, false means Cancel.
type ModalHostResult struct {
	Confirmed bool
}

// ModalHostEntry is one stacked dialog.
type ModalHostEntry struct {
	ID            uint64
	Kind          ModalHostConfirmKind
	Config        ModalHostConfirmConfig
	ZIndex        int
	Open          bool
	Closing       bool
	OkLoading     bool
	OkClicks      int
	FocusButton   string
	PrevFocusedID uint64
}

// ModalHostHandle is the imperative handle (destroy/update/promise).
type ModalHostHandle struct {
	host *ModalHostInstance
	id   uint64
	done chan ModalHostResult
	once sync.Once
}

// ID returns the entry id.
func (h *ModalHostHandle) ID() uint64 {
	if h == nil {
		return 0
	}
	return h.id
}

// Done returns the promise channel closed exactly once on close.
func (h *ModalHostHandle) Done() <-chan ModalHostResult {
	return h.done
}

// Destroy closes and removes the entry.
func (h *ModalHostHandle) Destroy() {
	if h == nil || h.host == nil {
		return
	}
	h.host.closeEntry(h.id, false)
}

// Update replaces title/content/width for a live entry (memo frozen while closing).
func (h *ModalHostHandle) Update(next ModalHostConfirmConfig) {
	if h == nil || h.host == nil {
		return
	}
	h.host.updateEntry(h.id, next)
}

// ModalHostInstance owns the dialog stack for one host mount.
type ModalHostInstance struct {
	props   ModalHostProps
	ctx     scope.Ctx
	mu      sync.Mutex
	seq     uint64
	stack   []*ModalHostEntry
	handles map[uint64]*ModalHostHandle
	mounted bool
}

// newModalHostInstance builds an unmounted instance.
func newModalHostInstance(ctx scope.Ctx, props ModalHostProps) *ModalHostInstance {
	if props.ZIndexBase == 0 {
		props.ZIndexBase = 1000
	}
	return &ModalHostInstance{
		props:   props,
		ctx:     ctx.Normalize(),
		handles: make(map[uint64]*ModalHostHandle),
	}
}

// Mount marks the host live.
func (in *ModalHostInstance) Mount(ctx scope.Ctx) {
	if in == nil {
		return
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	in.ctx = ctx.Normalize()
	in.mounted = true
}

// Update swaps props/ctx snapshots.
func (in *ModalHostInstance) Update(ctx scope.Ctx, next ModalHostProps) {
	if in == nil {
		return
	}
	if next.ZIndexBase == 0 {
		next.ZIndexBase = 1000
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	in.ctx = ctx.Normalize()
	in.props = next
	for i, e := range in.stack {
		e.ZIndex = in.props.ZIndexBase + i*10
	}
}

// Unmount destroys all entries in top-down order.
func (in *ModalHostInstance) Unmount() {
	if in == nil {
		return
	}
	in.DestroyAll()
	in.mu.Lock()
	in.mounted = false
	in.mu.Unlock()
}

// SetState runs f under the host lock (sole state mutation gate).
func (in *ModalHostInstance) SetState(f func(*ModalHostInstance)) {
	if in == nil || f == nil {
		return
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	f(in)
}

// Depth returns open stack depth.
func (in *ModalHostInstance) Depth() int {
	if in == nil {
		return 0
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	n := 0
	for _, e := range in.stack {
		if e.Open {
			n++
		}
	}
	return n
}

// Entries returns a snapshot bottom to top.
func (in *ModalHostInstance) Entries() []*ModalHostEntry {
	if in == nil {
		return nil
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	out := make([]*ModalHostEntry, len(in.stack))
	copy(out, in.stack)
	return out
}

// openLocked pushes one entry; caller holds lock.
func (in *ModalHostInstance) openLocked(cfg ModalHostConfirmConfig) *ModalHostHandle {
	in.seq++
	id := in.seq
	if cfg.Kind == "" {
		cfg.Kind = ModalHostKindConfirm
	}
	e := &ModalHostEntry{
		ID:          id,
		Kind:        cfg.Kind,
		Config:      cfg,
		ZIndex:      in.props.ZIndexBase + len(in.stack)*10,
		Open:        true,
		FocusButton: "ok",
	}
	if len(in.stack) > 0 {
		e.PrevFocusedID = in.stack[len(in.stack)-1].ID
	}
	in.stack = append(in.stack, e)
	h := &ModalHostHandle{host: in, id: id, done: make(chan ModalHostResult, 1)}
	in.handles[id] = h
	return h
}

// Confirm pushes a confirm dialog.
func (in *ModalHostInstance) Confirm(cfg ModalHostConfirmConfig) *ModalHostHandle {
	if in == nil {
		return nil
	}
	cfg.Kind = ModalHostKindConfirm
	in.mu.Lock()
	defer in.mu.Unlock()
	return in.openLocked(cfg)
}

// Info pushes an info dialog.
func (in *ModalHostInstance) Info(cfg ModalHostConfirmConfig) *ModalHostHandle {
	if in == nil {
		return nil
	}
	cfg.Kind = ModalHostKindInfo
	in.mu.Lock()
	defer in.mu.Unlock()
	return in.openLocked(cfg)
}

// Success pushes a success dialog.
func (in *ModalHostInstance) Success(cfg ModalHostConfirmConfig) *ModalHostHandle {
	if in == nil {
		return nil
	}
	cfg.Kind = ModalHostKindSuccess
	in.mu.Lock()
	defer in.mu.Unlock()
	return in.openLocked(cfg)
}

// Error pushes an error dialog.
func (in *ModalHostInstance) Error(cfg ModalHostConfirmConfig) *ModalHostHandle {
	if in == nil {
		return nil
	}
	cfg.Kind = ModalHostKindError
	in.mu.Lock()
	defer in.mu.Unlock()
	return in.openLocked(cfg)
}

// Warning pushes a warning dialog.
func (in *ModalHostInstance) Warning(cfg ModalHostConfirmConfig) *ModalHostHandle {
	if in == nil {
		return nil
	}
	cfg.Kind = ModalHostKindWarning
	in.mu.Lock()
	defer in.mu.Unlock()
	return in.openLocked(cfg)
}

// findLocked locates an entry; caller holds lock.
func (in *ModalHostInstance) findLocked(id uint64) (int, *ModalHostEntry) {
	for i, e := range in.stack {
		if e.ID == id {
			return i, e
		}
	}
	return -1, nil
}

// finishLocked resolves the promise exactly once and removes the entry.
func (in *ModalHostInstance) finishLocked(idx int, confirmed bool) {
	e := in.stack[idx]
	in.stack = append(in.stack[:idx], in.stack[idx+1:]...)
	for i, x := range in.stack {
		x.ZIndex = in.props.ZIndexBase + i*10
	}
	if h, ok := in.handles[e.ID]; ok {
		delete(in.handles, e.ID)
		h.once.Do(func() {
			h.done <- ModalHostResult{Confirmed: confirmed}
			close(h.done)
		})
	}
}

// closeEntry closes one layer only; neighbours keep order.
func (in *ModalHostInstance) closeEntry(id uint64, confirmed bool) {
	if in == nil {
		return
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	idx, e := in.findLocked(id)
	if idx < 0 || !e.Open {
		return
	}
	e.Closing = true
	in.finishLocked(idx, confirmed)
}

// updateEntry replaces config unless closing (memo frozen while closing).
func (in *ModalHostInstance) updateEntry(id uint64, next ModalHostConfirmConfig) {
	if in == nil {
		return
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	_, e := in.findLocked(id)
	if e == nil || e.Closing || !e.Open {
		return
	}
	if next.Kind == "" {
		next.Kind = e.Kind
	}
	e.Config = next
}

// DestroyAll clears the stack top-down (route change path).
func (in *ModalHostInstance) DestroyAll() {
	if in == nil {
		return
	}
	for {
		in.mu.Lock()
		if len(in.stack) == 0 {
			in.mu.Unlock()
			return
		}
		top := in.stack[len(in.stack)-1]
		in.mu.Unlock()
		in.closeEntry(top.ID, false)
	}
}

// ClickOk records one OK; duplicate clicks while loading are swallowed.
func (in *ModalHostInstance) ClickOk(id uint64) bool {
	if in == nil {
		return false
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	_, e := in.findLocked(id)
	if e == nil || !e.Open || e.Closing {
		return false
	}
	if e.OkLoading || e.Config.OkLoading {
		return false
	}
	e.OkClicks++
	if e.Config.Loading {
		return true
	}
	return true
}

// CompleteOk resolves async OK and closes when allow is true.
func (in *ModalHostInstance) CompleteOk(id uint64, allow bool) {
	if in == nil {
		return
	}
	in.mu.Lock()
	_, e := in.findLocked(id)
	needUnlock := true
	defer func() {
		if needUnlock {
			in.mu.Unlock()
		}
	}()
	if e == nil || !e.Open {
		return
	}
	e.OkLoading = false
	in.mu.Unlock()
	needUnlock = false
	if allow {
		in.closeEntry(id, true)
	}
}

// SetOkLoading marks async pending state.
func (in *ModalHostInstance) SetOkLoading(id uint64, loading bool) {
	if in == nil {
		return
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	_, e := in.findLocked(id)
	if e == nil {
		return
	}
	e.OkLoading = loading
}

// ClickCancel closes via cancel path.
func (in *ModalHostInstance) ClickCancel(id uint64) {
	in.closeEntry(id, false)
}

// PressEsc closes the top entry when its keyboard flag is on.
func (in *ModalHostInstance) PressEsc(id uint64) bool {
	if in == nil {
		return false
	}
	in.mu.Lock()
	_, e := in.findLocked(id)
	if e == nil || !e.Open {
		in.mu.Unlock()
		return false
	}
	kb := true
	if e.Config.KeyboardSet {
		kb = e.Config.Keyboard
	}
	in.mu.Unlock()
	if !kb {
		return false
	}
	in.closeEntry(id, false)
	return true
}

// ClickMask closes when merged mask is enabled and closable.
func (in *ModalHostInstance) ClickMask(id uint64) bool {
	if in == nil {
		return false
	}
	in.mu.Lock()
	_, e := in.findLocked(id)
	if e == nil || !e.Open {
		in.mu.Unlock()
		return false
	}
	mask := MergeModalHostMask(e.Config.Mask, e.Config.MaskSet, e.Config.MaskClosable)
	if !e.Config.MaskSet && !in.props.MaskSet {
		hostMask := in.props.Mask
		if !in.props.MaskSet {
			hostMask = DefaultModalHostMask()
		}
		if !hostMask.Enabled {
			mask.Enabled = false
		}
	}
	in.mu.Unlock()
	if !mask.Enabled || !mask.Closable {
		return false
	}
	in.closeEntry(id, false)
	return true
}

// PressTab cycles focus inside the top dialog (trap).
func (in *ModalHostInstance) PressTab(id uint64) string {
	if in == nil {
		return ""
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	_, e := in.findLocked(id)
	if e == nil || !e.Open {
		return ""
	}
	if e.FocusButton == "ok" {
		e.FocusButton = "cancel"
	} else {
		e.FocusButton = "ok"
	}
	return e.FocusButton
}

// ScrollLocked reports body scroll lock while any entry is open.
func (in *ModalHostInstance) ScrollLocked() bool {
	if in == nil {
		return false
	}
	in.mu.Lock()
	defer in.mu.Unlock()
	if !in.props.ScrollLock {
		return false
	}
	for _, e := range in.stack {
		if e.Open {
			return true
		}
	}
	return false
}

// HolderContent renders static-call holder copy through Ctx.
func (in *ModalHostInstance) HolderContent() string {
	if in == nil {
		return ""
	}
	in.mu.Lock()
	ctx := in.ctx
	in.mu.Unlock()
	if ctx.HolderRender == nil {
		return "modal holder"
	}
	return ctx.HolderRender("modal")
}
