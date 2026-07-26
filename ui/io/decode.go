// Package io provides async image decoding off the UI thread (F12).
package io

import (
	"sync"

	"github.com/energye/gpui/render"
)

// Result is delivered after DecodeFile/DecodeBytes completes.
type Result struct {
	Img *render.ImageBuf
	Err error
}

// Pool runs decode jobs on worker goroutines. Never call decode inside Layout/Paint.
type Pool struct {
	once sync.Once
	jobs chan func()
}

// Default is a process-wide pool with 2 workers.
var Default = NewPool(2)

// NewPool starts n workers (min 1).
func NewPool(n int) *Pool {
	if n < 1 {
		n = 1
	}
	p := &Pool{jobs: make(chan func(), 64)}
	for i := 0; i < n; i++ {
		go func() {
			for fn := range p.jobs {
				if fn != nil {
					fn()
				}
			}
		}()
	}
	return p
}

// DecodeFile loads an image path on a worker and calls done (may be any goroutine).
// Callers must hop to the UI thread before mutating RenderObjects.
func (p *Pool) DecodeFile(path string, done func(Result)) {
	if p == nil {
		p = Default
	}
	p.jobs <- func() {
		img, err := render.LoadImage(path)
		if done != nil {
			done(Result{Img: img, Err: err})
		}
	}
}

// DecodeBytes decodes image bytes on a worker.
// Uses a temp path-less path: write not available — use render if exported.
// For P4 MVP we decode via LoadImage only for files; bytes use image.Decode into CPU placeholder.
func (p *Pool) Run(fn func()) {
	if p == nil {
		p = Default
	}
	if fn == nil {
		return
	}
	p.jobs <- fn
}

// DecodeFileDefault uses Default pool.
func DecodeFile(path string, done func(Result)) {
	Default.DecodeFile(path, done)
}
