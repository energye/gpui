package border_beam

import ()

// BeamSnap is the frozen paint input for one border-beam frame (R2-6,
// button snapshot paradigm). The UI thread refreshes it inside dirty()
// (every paint-affecting setter funnels through dirty); the raster-thread
// paint path reads only this value (plus the atomic phase), never the live
// BorderBeam. Setters may keep mutating the widget while raster paints —
// the pixels always match the last UI refresh, with no cross-thread reads
// or writes of widget state.
type BeamSnap struct {
	// Visible is IsBeamVisible resolved at refresh time.
	Visible bool
	// RTL mirrors the phase direction (EffectivePhase applied on paint
	// against the atomic phase, so RTL is frozen here as a plain bool).
	RTL bool
	// Resolved geometry at refresh time (Resolved* defaults applied).
	Outset    float64
	LineWidth float64
	Radius    float64
	Size      float64
	// Stops is the resolved gradient (explicit copy or theme default,
	// frozen as an owned slice at refresh time).
	Stops []BorderBeamColorStop
}

// refreshSnapshot freezes the current paint inputs (UI thread only; called
// from dirty). Pure value build: no raster-side field is touched, so a
// concurrent paint of the previous snapshot is unaffected.
func (b *BorderBeam) refreshSnapshot() {
	if b == nil {
		return
	}
	s := BeamSnap{
		Visible:   b.IsBeamVisible(),
		RTL:       b.rtl,
		Outset:    b.ResolvedOutset(),
		LineWidth: b.ResolvedLineWidth(),
		Radius:    b.ResolvedBorderRadius(),
		Size:      b.ResolvedSize(),
		Stops:     b.ResolvedColorStops(),
	}
	b.snap.Store(s)
}

// loadSnapshot returns the last frozen snapshot; never nil for a live widget
// (zero value paints nothing: Visible=false, empty stops).
func (b *BorderBeam) loadSnapshot() BeamSnap {
	if b == nil {
		return BeamSnap{}
	}
	if v, ok := b.snap.Load().(BeamSnap); ok {
		return v
	}
	return BeamSnap{}
}
