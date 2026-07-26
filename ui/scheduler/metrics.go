package scheduler

import (
	"encoding/json"
	"sync"
	"time"
)

// HitchThresholdMs counts a frame interval as a hitch when it exceeds this
// wall-time gap (≈2× 60 Hz vsync). Used for soak / jank observation.
const HitchThresholdMs = 33.4

// FrameMetrics is the L1 observability snapshot (F14).
// JSON field names are stable for baseline tooling.
type FrameMetrics struct {
	// Counts
	FrameCount   int64 `json:"frame_count"`
	PresentCount int64 `json:"present_count"`
	MissedVSync  int64 `json:"missed_vsync"`
	HitchCount   int64 `json:"hitch_count"` // intervals > HitchThresholdMs

	// Pipeline
	PipelineDepth int `json:"pipeline_depth"`
	PipelineMax   int `json:"pipeline_max"`

	// Timing (milliseconds)
	LastFrameIntervalMs float64 `json:"last_frame_interval_ms"`
	MaxFrameIntervalMs  float64 `json:"max_frame_interval_ms"`
	AvgFrameIntervalMs  float64 `json:"avg_frame_interval_ms"`
	LastBuildMs         float64 `json:"frame_build_ms"`
	LastRasterMs        float64 `json:"frame_raster_ms"`

	// Optional placeholders filled later phases
	LayoutCount         int64 `json:"layout_count"`
	PaintCount          int64 `json:"paint_count"`
	RasterLayerCount    int64 `json:"raster_layer_count"`
	CompositeLayerCount int64 `json:"composite_layer_count"`
}

// MetricsStore is a concurrency-safe metrics accumulator.
type MetricsStore struct {
	mu          sync.Mutex
	m           FrameMetrics
	lastAt      time.Time
	intervalSum float64
	intervalN   int64
}

// Snapshot returns a copy of current metrics.
func (s *MetricsStore) Snapshot() FrameMetrics {
	if s == nil {
		return FrameMetrics{}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.m
}

// JSON returns metrics as JSON bytes.
func (s *MetricsStore) JSON() ([]byte, error) {
	m := s.Snapshot()
	return json.Marshal(m)
}

// NoteFrameInterval records wall time since previous frame note.
// Tracks max/avg interval and hitch count for soak observation.
func (s *MetricsStore) NoteFrameInterval(now time.Time) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.lastAt.IsZero() {
		ms := now.Sub(s.lastAt).Seconds() * 1000
		s.m.LastFrameIntervalMs = ms
		if ms > s.m.MaxFrameIntervalMs {
			s.m.MaxFrameIntervalMs = ms
		}
		s.intervalSum += ms
		s.intervalN++
		s.m.AvgFrameIntervalMs = s.intervalSum / float64(s.intervalN)
		if ms > HitchThresholdMs {
			s.m.HitchCount++
		}
	}
	s.lastAt = now
	s.m.FrameCount++
}

// NotePresent increments present counter.
func (s *MetricsStore) NotePresent() {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.m.PresentCount++
	s.mu.Unlock()
}

// NoteMissedVSync increments missed vsync counter.
func (s *MetricsStore) NoteMissedVSync() {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.m.MissedVSync++
	s.mu.Unlock()
}

// SetPipeline records current and max pipeline depth.
func (s *MetricsStore) SetPipeline(depth, max int) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.m.PipelineDepth = depth
	s.m.PipelineMax = max
	s.mu.Unlock()
}

// NoteBuildMs records UI frame build duration.
func (s *MetricsStore) NoteBuildMs(ms float64) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.m.LastBuildMs = ms
	s.mu.Unlock()
}

// NoteRasterMs records raster duration.
func (s *MetricsStore) NoteRasterMs(ms float64) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.m.LastRasterMs = ms
	s.mu.Unlock()
}

// SetRasterLayerCount records dirty-layer re-raster count for this frame (F02).
func (s *MetricsStore) SetRasterLayerCount(n int64) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.m.RasterLayerCount = n
	s.mu.Unlock()
}

// SetLayoutCount records layout flush count snapshot.
func (s *MetricsStore) SetLayoutCount(n int64) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.m.LayoutCount = n
	s.mu.Unlock()
}
