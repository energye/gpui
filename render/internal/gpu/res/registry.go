//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package res

import "sync"

// Native abstracts a native GPU resource. Release must be called exactly
// once, at the moment the resource is guaranteed safe to destroy.
type Native interface {
	Release()
}

// OwnerID identifies who owns registry entries. OwnerDevice (0) is the device
// itself — shared pipelines/caches. Every render session allocates its own id
// so a window close can drop exactly its entries from the one device-level
// registry (Skia GrContext: one resource cache, N surfaces).
type OwnerID uint64

// OwnerDevice is the device-wide owner (shared resources).
const OwnerDevice OwnerID = 0

// Owner is an owner-scoped handle over the device-level Registry. All
// registrations made through it are attributed to its id; ReleaseOwned /
// InvalidateOwned drop exactly those entries.
type Owner struct {
	reg *Registry
	id  OwnerID
}

// IsNil reports whether the handle is unbound (zero value).
func (o Owner) IsNil() bool { return o == (Owner{}) }

// ID returns the owner id.
func (o Owner) ID() OwnerID { return o.id }

// Registry returns the underlying registry.
func (o Owner) Registry() *Registry { return o.reg }

// Register adds a native resource owned by this owner and returns a strong
// Ref (refcount = 1).
func (o Owner) Register(res Native) Ref {
	if o.reg == nil {
		return Ref{}
	}
	return o.reg.register(o.id, res, false)
}

// RegisterBorrowed tracks a view owned outside the res system (texture cache
// slots, swapchain views) under this owner. Removal never calls the native
// Release — the external owner owns destruction.
func (o Owner) RegisterBorrowed(res Native) Ref {
	if o.reg == nil {
		return Ref{}
	}
	return o.reg.register(o.id, res, true)
}

// Bind makes ref the current active instance of a role for this owner. The
// previous bound instance (if any) is retired.
func (o Owner) Bind(key SourceKey, ref Ref) {
	if o.reg == nil {
		return
	}
	o.reg.bind(o.id, key, ref)
}

// Resolve returns the current active instance for a role key owned by this
// owner, incrementing its refcount (caller must Release the returned Ref).
func (o Owner) Resolve(key SourceKey) (Ref, bool) {
	if o.reg == nil {
		return Ref{}, false
	}
	return o.reg.resolve(o.id, key)
}

// ReleaseOwned force-releases every native entry owned by this owner and drops
// its role bindings. Used on session teardown while the device is still
// healthy (the GPU must be drained first) — NOT the device-loss path (use
// InvalidateOwned there). Borrowed entries are dropped without Release.
func (o Owner) ReleaseOwned() {
	if o.reg == nil {
		return
	}
	o.reg.releaseOwned(o.id)
}

// InvalidateOwned discards every entry and role owned by this owner WITHOUT
// calling native Release — the device-abandon flow owns native teardown.
// Used on device loss / AutoRecover.
func (o Owner) InvalidateOwned() {
	if o.reg == nil {
		return
	}
	o.reg.invalidateOwned(o.id)
}

// Count returns the number of live entries owned by this owner.
func (o Owner) Count() int {
	if o.reg == nil {
		return 0
	}
	return o.reg.countOwned(o.id, false)
}

// PendingReleaseCount returns how many of this owner's entries are retired but
// still waiting for their last ref / in-flight submission (diagnostics).
func (o Owner) PendingReleaseCount() int {
	if o.reg == nil {
		return 0
	}
	return o.reg.countOwned(o.id, true)
}

// roleKey is the map key for owner+role → current active instance.
type roleKey struct {
	owner OwnerID
	kind  Kind
	role  Role
	idx   uint32
}

// slot is one registry entry: the native resource plus reference state.
type slot struct {
	id       uint32
	owner    OwnerID
	res      Native
	refs     int32 // strong usage refs (Ref / Acquire)
	inflight int32 // referenced by submitted-but-unfinished command buffers
	retire   bool  // marked for destruction (waits for refs==0 && inflight==0)
	// borrowed marks views owned outside the res system (PictureTextureCache
	// slots, swapchain views). The registry tracks them for lifetime/ABA
	// safety but never calls Release — the owner decides when to destroy.
	borrowed bool
	// onIdle runs each time the entry becomes idle (refs==0 && inflight==0,
	// not retired). Cache uses it to re-pool released entries without a
	// separate pending queue.
	onIdle func()
}

// Ref is a strong usage reference (GrGpuResource::ref semantics). While a Ref
// exists the underlying resource is guaranteed alive; a ref>0 resource is
// never destroyed.
type Ref struct {
	reg *Registry
	id  uint32
}

// IsNil reports whether the Ref is the zero value.
func (r Ref) IsNil() bool { return r.reg == nil || r.id == 0 }

// Registry is one device's resource state machine, shared by every window
// session on that device (Skia GrResourceCache). Ids are monotonic and never
// reused (no ABA problem). Actual native destruction is deferred until
// refs==0 && inflight==0 && retire, mirroring Skia command-buffer refs.
//
// All methods are safe for concurrent use: window sessions on the same device
// may retire resources from different raster threads. Idle callbacks fire
// outside the lock so they can safely re-enter the registry.
type Registry struct {
	mu sync.Mutex

	slots     map[uint32]*slot
	nextID    uint32
	nextOwner uint64
	roles     map[roleKey]uint32 // role → current active instance id

	resolveHits   uint64
	resolveMisses uint64
}

// NewRegistry creates an empty registry.
func NewRegistry() *Registry {
	return &Registry{
		slots: make(map[uint32]*slot),
		roles: make(map[roleKey]uint32),
	}
}

// NewOwner allocates a fresh owner id and returns its handle.
func (r *Registry) NewOwner() Owner {
	if r == nil {
		return Owner{}
	}
	r.mu.Lock()
	r.nextOwner++
	id := OwnerID(r.nextOwner)
	r.mu.Unlock()
	return Owner{reg: r, id: id}
}

// DeviceOwner returns the device-wide owner handle (shared resources).
func (r *Registry) DeviceOwner() Owner {
	if r == nil {
		return Owner{}
	}
	return Owner{reg: r, id: OwnerDevice}
}

func (r *Registry) register(owner OwnerID, res Native, borrowed bool) Ref {
	if r == nil || res == nil {
		return Ref{}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.nextID++
	s := &slot{id: r.nextID, owner: owner, res: res, refs: 1, borrowed: borrowed}
	r.slots[s.id] = s
	return Ref{reg: r, id: s.id}
}

// Acquire increments the refcount of an existing entry and returns a new Ref.
// Returns a nil Ref if the id is unknown.
func (r *Registry) Acquire(id uint32) Ref {
	if r == nil || id == 0 {
		return Ref{}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.slots[id]
	if !ok {
		return Ref{}
	}
	s.refs++
	return Ref{reg: r, id: id}
}

// acquireIfIdle refs an idle entry atomically: pooled entries may only be
// lent out when no usage ref and no submission still references them.
func (r *Registry) acquireIfIdle(id uint32) (Ref, bool) {
	if r == nil || id == 0 {
		return Ref{}, false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	s, ok := r.slots[id]
	if !ok || s.retire || s.refs != 0 || s.inflight != 0 {
		return Ref{}, false
	}
	s.refs++
	return Ref{reg: r, id: id}, true
}

// hasEntry reports whether the id is still registered.
func (r *Registry) hasEntry(id uint32) bool {
	if r == nil || id == 0 {
		return false
	}
	r.mu.Lock()
	_, ok := r.slots[id]
	r.mu.Unlock()
	return ok
}

// idleEntry reports whether the entry exists and is reusable (refs==0,
// inflight==0, not retired).
func (r *Registry) idleEntry(id uint32) bool {
	if r == nil || id == 0 {
		return false
	}
	r.mu.Lock()
	s, ok := r.slots[id]
	idle := ok && !s.retire && s.refs == 0 && s.inflight == 0
	r.mu.Unlock()
	return idle
}

// retireEntry marks an entry for destruction and releases it when idle.
// Returns false when the id is unknown.
func (r *Registry) retireEntry(id uint32) bool {
	if r == nil || id == 0 {
		return false
	}
	r.mu.Lock()
	s, ok := r.slots[id]
	if !ok {
		r.mu.Unlock()
		return false
	}
	s.retire = true
	r.removeIfReady(s)
	r.mu.Unlock()
	return true
}

// Release decrements the refcount. The native resource is released only when
// refs==0 && inflight==0 && retire (see removeIfReady).
func (r *Registry) Release(ref Ref) {
	if r == nil || ref.reg != r || ref.id == 0 {
		return
	}
	var cb func()
	r.mu.Lock()
	if s, ok := r.slots[ref.id]; ok {
		if s.refs > 0 {
			s.refs--
		}
		r.removeIfReady(s)
		cb = idleCallbackOf(s)
	}
	r.mu.Unlock()
	if cb != nil {
		cb()
	}
}

// Retire marks an entry for destruction. The native resource is released only
// once no usage refs and no in-flight submissions remain.
func (r *Registry) Retire(ref Ref) {
	if r == nil || ref.reg != r || ref.id == 0 {
		return
	}
	r.mu.Lock()
	if s, ok := r.slots[ref.id]; ok {
		s.retire = true
		r.removeIfReady(s)
	}
	r.mu.Unlock()
}

func (r *Registry) bind(owner OwnerID, key SourceKey, ref Ref) {
	if r == nil || ref.reg != r || ref.id == 0 {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	k := roleKey{owner: owner, kind: key.Kind, role: key.Role, idx: key.Index}
	if oldID, ok := r.roles[k]; ok && oldID != ref.id {
		if old, ok := r.slots[oldID]; ok {
			old.retire = true
			r.removeIfReady(old)
		}
	}
	r.roles[k] = ref.id
}

func (r *Registry) resolve(owner OwnerID, key SourceKey) (Ref, bool) {
	if r == nil {
		return Ref{}, false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	k := roleKey{owner: owner, kind: key.Kind, role: key.Role, idx: key.Index}
	id, ok := r.roles[k]
	if !ok || id == 0 {
		r.resolveMisses++
		return Ref{}, false
	}
	s, ok := r.slots[id]
	if !ok {
		r.resolveMisses++
		return Ref{}, false
	}
	// A bound instance must not be retired-while-bound: Bind retires the
	// predecessor, and Resolve runs before any rebuild in the same flush.
	if s.retire {
		r.resolveMisses++
		return Ref{}, false
	}
	s.refs++
	r.resolveHits++
	return Ref{reg: r, id: id}, true
}

// The frame gate is: ResolveMiss must stay 0 under resize — a miss means a
// command resolved to a role whose active instance was unavailable.
func (r *Registry) ResolveStats() (hits, misses uint64) {
	if r == nil {
		return 0, 0
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.resolveHits, r.resolveMisses
}

// markInflight adjusts the in-flight counter of an entry. Used by Submission.
// When the counter drops to zero and the entry is retired with no refs, the
// native resource is released.
func (r *Registry) markInflight(id uint32, delta int32) {
	if r == nil || id == 0 {
		return
	}
	var cb func()
	r.mu.Lock()
	if s, ok := r.slots[id]; ok {
		s.inflight += delta
		if s.inflight < 0 {
			s.inflight = 0
		}
		if s.inflight == 0 {
			r.removeIfReady(s)
			cb = idleCallbackOf(s)
		}
	}
	r.mu.Unlock()
	if cb != nil {
		cb()
	}
}

// clearInflight drops the in-flight count without releasing (device loss path:
// no fence will arrive, and the abandon flow owns native teardown).
func (r *Registry) clearInflight(id uint32) {
	if r == nil || id == 0 {
		return
	}
	r.mu.Lock()
	if s, ok := r.slots[id]; ok {
		s.inflight = 0
	}
	r.mu.Unlock()
}

// SetIdleCallback registers fn to run whenever the entry transitions to idle
// (refs==0 && inflight==0 and not retired). Replaces the per-cache pending
// queue: the callback runs from Release or from the submission completion
// that clears the in-flight count, whichever comes last.
func (r *Registry) SetIdleCallback(id uint32, fn func()) {
	if r == nil || id == 0 {
		return
	}
	r.mu.Lock()
	if s, ok := r.slots[id]; ok {
		s.onIdle = fn
	}
	r.mu.Unlock()
}

// idleCallbackOf returns the idle callback to run after unlocking, or nil when
// the entry is not idle or not callable.
func idleCallbackOf(s *slot) func() {
	if s == nil || s.retire || s.refs != 0 || s.inflight != 0 {
		return nil
	}
	return s.onIdle
}

// InvalidateAll discards every entry WITHOUT calling native Release — native
// teardown is handled by the device-abandon flow (abandonDeviceOwnedLocked),
// keeping a single release outlet. Used on device loss / AutoRecover.
func (r *Registry) InvalidateAll() {
	if r == nil {
		return
	}
	r.mu.Lock()
	r.slots = make(map[uint32]*slot)
	r.roles = make(map[roleKey]uint32)
	r.mu.Unlock()
}

// ReleaseAll force-releases every registered native resource and clears the
// registry. Used on device teardown while the device is still healthy —
// NOT the device-loss path (use InvalidateAll there). Safe to call with
// outstanding Refs: native Release is idempotent at the facade layer.
// Borrowed entries are dropped without Release (their owner destroys them).
func (r *Registry) ReleaseAll() {
	if r == nil {
		return
	}
	var natives []Native
	r.mu.Lock()
	for _, s := range r.slots {
		if s != nil && s.res != nil && !s.borrowed {
			natives = append(natives, s.res)
		}
	}
	r.slots = make(map[uint32]*slot)
	r.roles = make(map[roleKey]uint32)
	r.mu.Unlock()
	for _, n := range natives {
		n.Release()
	}
}

func (r *Registry) releaseOwned(owner OwnerID) {
	if r == nil {
		return
	}
	var natives []Native
	r.mu.Lock()
	for id, s := range r.slots {
		if s == nil || s.owner != owner {
			continue
		}
		if s.res != nil && !s.borrowed {
			natives = append(natives, s.res)
		}
		delete(r.slots, id)
	}
	for k := range r.roles {
		if k.owner == owner {
			delete(r.roles, k)
		}
	}
	r.mu.Unlock()
	for _, n := range natives {
		n.Release()
	}
}

// releaseIDs force-releases the given entries and drops their roles, ignoring
// refcounts. Device-teardown helper for Cache.ReleaseAll (single release
// outlet for cache-owned natives).
func (r *Registry) releaseIDs(ids []uint32) {
	if r == nil || len(ids) == 0 {
		return
	}
	var natives []Native
	r.mu.Lock()
	for _, id := range ids {
		s, ok := r.slots[id]
		if !ok {
			continue
		}
		if s.res != nil && !s.borrowed {
			natives = append(natives, s.res)
		}
		delete(r.slots, id)
		for k, rid := range r.roles {
			if rid == id {
				delete(r.roles, k)
			}
		}
	}
	r.mu.Unlock()
	for _, n := range natives {
		n.Release()
	}
}

func (r *Registry) invalidateOwned(owner OwnerID) {
	if r == nil {
		return
	}
	r.mu.Lock()
	for id, s := range r.slots {
		if s == nil || s.owner != owner {
			continue
		}
		delete(r.slots, id)
	}
	for k := range r.roles {
		if k.owner == owner {
			delete(r.roles, k)
		}
	}
	r.mu.Unlock()
}

func (r *Registry) countOwned(owner OwnerID, retiredOnly bool) int {
	if r == nil {
		return 0
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for _, s := range r.slots {
		if s == nil || s.owner != owner {
			continue
		}
		if retiredOnly && !s.retire {
			continue
		}
		n++
	}
	return n
}

// ID returns the registry id carried by the Ref (diagnostics / cache keys).
func (r *Registry) ID(ref Ref) uint32 {
	if r == nil || ref.reg != r {
		return 0
	}
	return ref.id
}

// NativeOf returns the native resource behind a Ref (diagnostics / tests).
// Returns nil if the id is unknown or has been released.
func (r *Registry) NativeOf(ref Ref) Native {
	if r == nil || ref.reg != r || ref.id == 0 {
		return nil
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if s, ok := r.slots[ref.id]; ok {
		return s.res
	}
	return nil
}

// Count returns the number of live entries (diagnostics).
func (r *Registry) Count() int {
	if r == nil {
		return 0
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.slots)
}

// PendingReleaseCount returns how many live entries are retired but still
// waiting for their last ref / in-flight submission (diagnostics).
func (r *Registry) PendingReleaseCount() int {
	if r == nil {
		return 0
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	n := 0
	for _, s := range r.slots {
		if s != nil && s.retire {
			n++
		}
	}
	return n
}

// removeIfReady releases the native resource when the entry is retired and
// has no usage refs and no in-flight submissions. Caller holds r.mu.
func (r *Registry) removeIfReady(s *slot) {
	if s == nil || !s.retire || s.refs != 0 || s.inflight != 0 {
		return
	}
	if s.res != nil && !s.borrowed {
		s.res.Release()
	}
	delete(r.slots, s.id)
	for k, id := range r.roles {
		if id == s.id {
			delete(r.roles, k)
		}
	}
}
