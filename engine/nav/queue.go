//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package nav

import (
	"encoding/json"

	"github.com/energye/gpui/engine/core"
)

// Queue amortization (S87 truth): 100 agents at 8 paths/frame drain in
// 13 frames (ceil(100/8)), so one spike frame never pays 100 A* runs.
// The count lives in JSON (per_frame) and is tunable without code.
const (
	// DefaultPerFrame is the paths computed per Update.
	DefaultPerFrame = 8
	// MaxPerFrame caps one Update so a tuned value cannot spike a frame.
	MaxPerFrame = 64
)

// Request is one queued path job: dense FIFO order, IDs rise in enqueue order.
type Request struct {
	ID          int
	Start, Goal Cell
}

// Result is one computed terminal: Path non-nil only on StatusFound.
type Result struct {
	ID     int
	Path   []Cell
	Status Status
}

// Queue holds pending requests plus computed terminals. The caller owns
// grid truth and drains via Update; Queue never draws and never moves.
type Queue struct {
	perFrame int
	nextID   int
	pending  []Request
	results  []Result
	updates  int64
	computed int64
}

// NewQueue builds an empty queue draining DefaultPerFrame paths/Update.
func NewQueue() Queue {
	return Queue{perFrame: DefaultPerFrame}
}

// PerFrame returns the drain count (0 on nil).
func (q *Queue) PerFrame() int {
	if q == nil {
		return 0
	}
	return q.perFrame
}

// SetPerFrame tunes the drain count: 1..MaxPerFrame, else InvalidArg
// keeps the old count.
func (q *Queue) SetPerFrame(n int) error {
	const op = "nav.Queue.SetPerFrame"
	if q == nil {
		return core.InvalidArg(op, "queue")
	}
	if n < 1 || n > MaxPerFrame {
		return core.InvalidArg(op, "per_frame")
	}
	q.perFrame = n
	return nil
}

// Enqueue appends one job and returns its ID. Start/goal validity is
// checked at Update time against the live grid, never here, so clicks
// enqueue fast. Nil queues report -1.
func (q *Queue) Enqueue(start, goal Cell) int {
	if q == nil {
		return -1
	}
	id := q.nextID
	q.nextID++
	q.pending = append(q.pending, Request{ID: id, Start: start, Goal: goal})
	return id
}

// Pending counts queued-but-uncomputed jobs, or 0 on nil.
func (q *Queue) Pending() int {
	if q == nil {
		return 0
	}
	return len(q.pending)
}

// Done counts computed terminals, or 0 on nil.
func (q *Queue) Done() int {
	if q == nil {
		return 0
	}
	return len(q.results)
}

// Updates counts Update calls, or 0 on nil.
func (q *Queue) Updates() int64 {
	if q == nil {
		return 0
	}
	return q.updates
}

// Results returns a fresh copy of every computed terminal in compute order.
func (q *Queue) Results() []Result {
	if q == nil || len(q.results) == 0 {
		return nil
	}
	out := make([]Result, len(q.results))
	copy(out, q.results)
	for i := range out {
		out[i].Path = append([]Cell(nil), q.results[i].Path...)
	}
	return out
}

// Clear forgets pending jobs and computed terminals; tuning and the ID
// ledger stay so IDs never repeat in one session.
func (q *Queue) Clear() {
	if q == nil {
		return
	}
	q.pending = nil
	q.results = nil
}

// Update computes up to perFrame pending jobs against g and appends one
// Result per job (found and no-path alike are terminals). Invalid grids
// are InvalidArg and compute nothing; pending stays queued. Bad cells in
// one job mark that job invalid without stopping the rest of the frame.
func (q *Queue) Update(g Grid) (int, error) {
	const op = "nav.Queue.Update"
	if q == nil {
		return 0, core.InvalidArg(op, "queue")
	}
	if !g.Valid() {
		return 0, core.InvalidArg(op, "grid")
	}
	n := q.perFrame
	if n > len(q.pending) {
		n = len(q.pending)
	}
	for i := 0; i < n; i++ {
		req := q.pending[0]
		q.pending = q.pending[1:]
		path, st, err := FindPath(g, req.Start, req.Goal)
		if err != nil && st != StatusStartBlocked && st != StatusGoalBlocked && st != StatusInvalid {
			st = StatusInvalid
		}
		_ = err
		q.results = append(q.results, Result{ID: req.ID, Path: path, Status: st})
		q.computed++
	}
	q.updates++
	return n, nil
}

// DrainFrames predicts Update calls to finish total jobs at perFrame
// each: ceil(total/perFrame). Bad inputs report 0.
func DrainFrames(total, perFrame int) int {
	if total <= 0 || perFrame <= 0 {
		return 0
	}
	return (total + perFrame - 1) / perFrame
}

// Tuning is the JSON-tunable queue file shape: {"per_frame": 8}.
type Tuning struct {
	PerFrame int `json:"per_frame"`
}

// LoadTuning parses queue tuning: per_frame must sit in 1..MaxPerFrame.
// Syntax faults are BadData; range faults are InvalidArg.
func LoadTuning(raw []byte) (Tuning, error) {
	const op = "nav.LoadTuning"
	var t Tuning
	if err := json.Unmarshal(raw, &t); err != nil {
		return Tuning{}, core.BadData(op, "json")
	}
	if t.PerFrame < 1 || t.PerFrame > MaxPerFrame {
		return Tuning{}, core.InvalidArg(op, "per_frame")
	}
	return t, nil
}
