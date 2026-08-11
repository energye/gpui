package res

// Submission models one Queue.Submit lifecycle (command-buffer refs).
// Begin frames a submission; Track marks the resources referenced by the
// submitted command buffers as in-flight; SubmitDone (reached via
// OnSubmittedWorkDone callback or a synchronous readback point) releases the
// in-flight count, which is what finally allows retired resources to be
// destroyed. Invalidate handles device loss where the fence never arrives.
type Submission struct {
	reg  *Registry
	jobs []uint32 // registry ids tracked by the current submission
}

// NewSubmission creates a submission bound to reg.
func NewSubmission(reg *Registry) *Submission {
	return &Submission{reg: reg}
}

// Begin starts a new submission window. Any prior un-resolved jobs are
// dropped (callers must ensure the previous submission completed or was
// invalidated first).
func (s *Submission) Begin() {
	if s == nil {
		return
	}
	s.jobs = s.jobs[:0]
}

// Track marks ref as referenced by this submission: the underlying entry is
// placed in-flight until SubmitDone.
func (s *Submission) Track(ref Ref) {
	if s == nil || ref.IsNil() || ref.reg != s.reg {
		return
	}
	s.reg.markInflight(ref.id, 1)
	s.jobs = append(s.jobs, ref.id)
}

// SubmitDone resolves the submission: every tracked entry leaves the
// in-flight state, allowing retired resources to be released.
func (s *Submission) SubmitDone() {
	if s == nil {
		return
	}
	for _, id := range s.jobs {
		s.reg.markInflight(id, -1)
	}
	s.jobs = s.jobs[:0]
}

// Invalidate force-resolves the submission on device loss, where no fence will
// arrive. Native resources are NOT released here (the abandon flow owns that);
// the in-flight state is simply cleared so further bookkeeping is consistent.
func (s *Submission) Invalidate() {
	if s == nil {
		return
	}
	for _, id := range s.jobs {
		if e, ok := s.reg.slots[id]; ok {
			e.inflight = 0
		}
	}
	s.jobs = s.jobs[:0]
}

// InFlight returns the set of registry ids currently tracked (diagnostics).
func (s *Submission) InFlight() []uint32 {
	if s == nil {
		return nil
	}
	out := make([]uint32, len(s.jobs))
	copy(out, s.jobs)
	return out
}