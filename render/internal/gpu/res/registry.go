package res

// Native abstracts a native GPU resource. Release must be called exactly
// once, at the moment the resource is guaranteed safe to destroy.
type Native interface {
	Release()
}

// roleKey is the map key for role → current active instance.
type roleKey struct {
	kind Kind
	role Role
	idx  uint32
}

// slot is one registry entry: the native resource plus reference state.
type slot struct {
	id       uint32
	res      Native
	refs     int32 // strong usage refs (Ref / Acquire)
	inflight int32 // referenced by submitted-but-unfinished command buffers
	retire   bool  // marked for destruction (waits for refs==0 && inflight==0)
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

// Registry maps ids to resources with reference counts. Ids are monotonic and
// never reused (uint64 semantics with uint32, no ABA problem). Actual native
// destruction is deferred until refs==0 && inflight==0 && retire, mirroring
// Skia command-buffer refs.
type Registry struct {
	slots  map[uint32]*slot
	nextID uint32
	roles  map[roleKey]uint32 // role → current active instance id
}

// NewRegistry creates an empty registry.
func NewRegistry() *Registry {
	return &Registry{
		slots: make(map[uint32]*slot),
		roles: make(map[roleKey]uint32),
	}
}

// Register adds a native resource and returns a strong Ref (refcount = 1).
func (r *Registry) Register(res Native) Ref {
	if r == nil || res == nil {
		return Ref{}
	}
	r.nextID++
	s := &slot{id: r.nextID, res: res, refs: 1}
	r.slots[s.id] = s
	return Ref{reg: r, id: s.id}
}

// Acquire increments the refcount of an existing entry and returns a new Ref.
// Returns a nil Ref if the id is unknown.
func (r *Registry) Acquire(id uint32) Ref {
	if r == nil || id == 0 {
		return Ref{}
	}
	s, ok := r.slots[id]
	if !ok {
		return Ref{}
	}
	s.refs++
	return Ref{reg: r, id: id}
}

// Release decrements the refcount. The native resource is released only when
// refs==0 && inflight==0 && retire (see removeIfReady).
func (r *Registry) Release(ref Ref) {
	if r == nil || ref.reg != r || ref.id == 0 {
		return
	}
	s, ok := r.slots[ref.id]
	if !ok {
		return
	}
	if s.refs > 0 {
		s.refs--
	}
	r.removeIfReady(s)
}

// Retire marks an entry for destruction. The native resource is released only
// once no usage refs and no in-flight submissions remain.
func (r *Registry) Retire(ref Ref) {
	if r == nil || ref.reg != r || ref.id == 0 {
		return
	}
	s, ok := r.slots[ref.id]
	if !ok {
		return
	}
	s.retire = true
	r.removeIfReady(s)
}

// Bind makes ref the current active instance of a role. The previous bound
// instance (if any) is retired — it will be destroyed once unused.
func (r *Registry) Bind(key SourceKey, ref Ref) {
	if r == nil || ref.reg != r || ref.id == 0 {
		return
	}
	k := roleKey{kind: key.Kind, role: key.Role, idx: key.Index}
	if oldID, ok := r.roles[k]; ok && oldID != ref.id {
		if old, ok := r.slots[oldID]; ok {
			old.retire = true
			r.removeIfReady(old)
		}
	}
	r.roles[k] = ref.id
}

// Resolve returns the current active instance for a role key, incrementing
// its refcount (caller must Release the returned Ref). Resolution happens at
// flush time; the resolved instance is by construction the current one.
func (r *Registry) Resolve(key SourceKey) (Ref, bool) {
	if r == nil {
		return Ref{}, false
	}
	k := roleKey{kind: key.Kind, role: key.Role, idx: key.Index}
	id, ok := r.roles[k]
	if !ok || id == 0 {
		return Ref{}, false
	}
	s, ok := r.slots[id]
	if !ok {
		return Ref{}, false
	}
	// A bound instance must not be retired-while-bound: Bind retires the
	// predecessor, and Resolve runs before any rebuild in the same flush.
	if s.retire {
		return Ref{}, false
	}
	s.refs++
	return Ref{reg: r, id: id}, true
}

// markInflight adjusts the in-flight counter of an entry. Used by Submission.
// When the counter drops to zero and the entry is retired with no refs, the
// native resource is released.
func (r *Registry) markInflight(id uint32, delta int32) {
	if r == nil || id == 0 {
		return
	}
	s, ok := r.slots[id]
	if !ok {
		return
	}
	s.inflight += delta
	if s.inflight < 0 {
		s.inflight = 0
	}
	if s.inflight == 0 {
		r.removeIfReady(s)
	}
}

// InvalidateAll discards every entry WITHOUT calling native Release — native
// teardown is handled by the device-abandon flow (abandonDeviceOwnedLocked),
// keeping a single release outlet. Used on device loss / AutoRecover.
func (r *Registry) InvalidateAll() {
	if r == nil {
		return
	}
	r.slots = make(map[uint32]*slot)
	r.roles = make(map[roleKey]uint32)
}

// ReleaseAll force-releases every registered native resource and clears the
// registry. Used on session teardown while the device is still healthy —
// NOT the device-loss path (use InvalidateAll there). Safe to call with
// outstanding Refs: native Release is idempotent at the facade layer.
func (r *Registry) ReleaseAll() {
	if r == nil {
		return
	}
	for _, s := range r.slots {
		if s != nil && s.res != nil {
			s.res.Release()
		}
	}
	r.slots = make(map[uint32]*slot)
	r.roles = make(map[roleKey]uint32)
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
	return len(r.slots)
}

// removeIfReady releases the native resource when the entry is retired and
// has no usage refs and no in-flight submissions.
func (r *Registry) removeIfReady(s *slot) {
	if s == nil || !s.retire || s.refs != 0 || s.inflight != 0 {
		return
	}
	if s.res != nil {
		s.res.Release()
	}
	delete(r.slots, s.id)
	for k, id := range r.roles {
		if id == s.id {
			delete(r.roles, k)
		}
	}
}