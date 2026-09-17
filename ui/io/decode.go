// Package io provides async image decoding off the UI thread (F12).
//
// T4 image lane: the pool keeps 2 small-image workers and adds one
// dedicated large-image worker (single flight). Pictures larger than
// LargeImagePixels never run on the small workers, so a burst of big
// pictures cannot stall small UI images. Window close cancels via
// context: jobs whose context is done before or after decoding are
// dropped without invoking the callback. Decoders live behind a small
// registry (G8): PNG/JPEG/WebP are registered in init, tests register
// stubs by name. Callers must hop to the UI thread before mutating
// RenderObjects.
package io

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/energye/gpui/render"
	"golang.org/x/image/webp"
)

// LargeImagePixels routes pictures above this pixel count to the single
// large-image worker (T4: big pictures >2MP fly solo).
const LargeImagePixels = 2000000

// Result is delivered after DecodeFile/DecodeBytes completes.
// Img/Err are the original fields; Format/Width/Height/Large describe
// what was decoded (zero when Err != nil).
type Result struct {
	Img    *render.ImageBuf
	Err    error
	Format string
	Width  int
	Height int
	Large  bool
}

// Decoder is one registered image format (G8). Decode turns a byte
// stream into a buffer; Config reports dimensions without full decode
// so the pool can route big pictures before paying for pixels.
type Decoder struct {
	Decode func(r io.Reader) (*render.ImageBuf, error)
	Config func(r io.Reader) (image.Config, error)
}

var (
	regMu    sync.RWMutex
	registry = map[string]Decoder{}
)

// RegisterDecoder plugs a new image format in (G8). Registering an
// existing name replaces it (tests use this to prove the table is
// consulted). Empty name or nil Decode/Config is ignored.
func RegisterDecoder(name string, d Decoder) {
	if name == "" || d.Decode == nil || d.Config == nil {
		return
	}
	regMu.Lock()
	defer regMu.Unlock()
	registry[strings.ToLower(name)] = d
}

// SupportedDecoders lists registered format names in sorted order.
func SupportedDecoders() []string {
	regMu.RLock()
	defer regMu.RUnlock()
	out := make([]string, 0, len(registry))
	for k := range registry {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// lookupDecoder returns the registered decoder for name (case-insensitive).
func lookupDecoder(name string) (Decoder, bool) {
	regMu.RLock()
	defer regMu.RUnlock()
	d, ok := registry[strings.ToLower(name)]
	return d, ok
}

// formatForExt maps a file extension to a registry name.
// Unknown extensions return "" so the caller falls back to sniffing.
func formatForExt(path string) string {
	switch strings.ToLower(strings.TrimPrefix(filepath.Ext(path), ".")) {
	case "png":
		return "png"
	case "jpg", "jpeg":
		return "jpeg"
	case "webp":
		return "webp"
	default:
		return ""
	}
}

// sniffFormat runs Config of every registered decoder over data and
// returns the first that recognizes it (sorted order, deterministic).
func sniffFormat(data []byte) (string, image.Config, bool) {
	for _, name := range SupportedDecoders() {
		d, ok := lookupDecoder(name)
		if !ok {
			continue
		}
		cfg, err := d.Config(bytes.NewReader(data))
		if err == nil && cfg.Width > 0 && cfg.Height > 0 {
			return name, cfg, true
		}
	}
	return "", image.Config{}, false
}

// PoolStats is a point-in-time copy of pool counters.
type PoolStats struct {
	Submitted   int64 `json:"submitted"`
	Completed   int64 `json:"completed"`
	Cancelled   int64 `json:"cancelled"`
	LargeRouted int64 `json:"large_routed"`
}

// decodeReq is one unit of pool work. fn != nil is a bare func (Run);
// otherwise it is a file/bytes decode with an optional cancel context.
type decodeReq struct {
	ctx     context.Context
	path    string
	data    []byte
	hasData bool
	fn      func()
	done    func(Result)
}

// Pool runs decode jobs on worker goroutines. Never call decode inside Layout/Paint.
type Pool struct {
	jobs       chan decodeReq
	largeJobs  chan decodeReq
	closeOnce  sync.Once
	wg         sync.WaitGroup
	submitted  atomic.Int64
	completed  atomic.Int64
	cancelled  atomic.Int64
	largeJobsN atomic.Int64
}

// Default is a process-wide pool with 2 workers. Never Close it:
// windows share it, per-window cancel uses request contexts.
var Default = NewPool(2)

// NewPool starts n small-image workers (min 1) plus one dedicated
// large-image worker that drains largeJobs serially (single flight).
func NewPool(n int) *Pool {
	if n < 1 {
		n = 1
	}
	p := &Pool{
		jobs:      make(chan decodeReq, 64),
		largeJobs: make(chan decodeReq, 64),
	}
	for i := 0; i < n; i++ {
		p.wg.Add(1)
		go func() {
			defer p.wg.Done()
			for req := range p.jobs {
				p.serveSmall(req)
			}
		}()
	}
	p.wg.Add(1)
	go func() {
		defer p.wg.Done()
		for req := range p.largeJobs {
			p.serveDecode(req)
		}
	}()
	return p
}

// Close drains the pool and waits for workers. Only close pools you
// created (never Default); in-flight request contexts still suppress
// their callbacks. Double Close is safe.
func (p *Pool) Close() {
	if p == nil {
		return
	}
	p.closeOnce.Do(func() {
		close(p.jobs)
		close(p.largeJobs)
	})
	p.wg.Wait()
}

// Stats snapshots the pool counters.
func (p *Pool) Stats() PoolStats {
	if p == nil {
		return PoolStats{}
	}
	return PoolStats{
		Submitted:   p.submitted.Load(),
		Completed:   p.completed.Load(),
		Cancelled:   p.cancelled.Load(),
		LargeRouted: p.largeJobsN.Load(),
	}
}

// cancelled reports whether ctx is done (nil ctx never cancels).
func cancelled(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	select {
	case <-ctx.Done():
		return true
	default:
		return false
	}
}

// deliver invokes done unless the request was cancelled. Cancelled
// jobs are counted and never call back (the window is gone).
func (p *Pool) deliver(req decodeReq, res Result) {
	if req.done == nil {
		return
	}
	if cancelled(req.ctx) {
		p.cancelled.Add(1)
		return
	}
	p.completed.Add(1)
	req.done(res)
}

// serveSmall runs on a small worker: bare funcs execute inline,
// decodes peek at dimensions first and forward big pictures to the
// large lane so small images never queue behind a big decode.
func (p *Pool) serveSmall(req decodeReq) {
	if req.fn != nil {
		if cancelled(req.ctx) {
			p.cancelled.Add(1)
			return
		}
		req.fn()
		return
	}
	if cancelled(req.ctx) {
		p.cancelled.Add(1)
		return
	}
	if p.isLarge(req) {
		p.largeJobsN.Add(1)
		if req.ctx != nil {
			select {
			case p.largeJobs <- req:
			case <-req.ctx.Done():
				p.cancelled.Add(1)
			}
			return
		}
		p.largeJobs <- req
		return
	}
	p.serveDecode(req)
}

// serveDecode runs the full decode on whichever lane owns the request.
func (p *Pool) serveDecode(req decodeReq) {
	if req.fn != nil {
		if cancelled(req.ctx) {
			p.cancelled.Add(1)
			return
		}
		req.fn()
		return
	}
	if cancelled(req.ctx) {
		p.cancelled.Add(1)
		return
	}
	var res Result
	if req.hasData {
		res = decodeBytes(req.data)
	} else {
		res = decodeFile(req.path)
	}
	p.deliver(req, res)
}

// isLarge peeks at dimensions only (header decode, no pixels).
// Peek failures return false so the full decode reports the error.
func (p *Pool) isLarge(req decodeReq) bool {
	var w, h int
	if req.hasData {
		_, cfg, ok := sniffFormat(req.data)
		if !ok {
			return false
		}
		w, h = cfg.Width, cfg.Height
	} else {
		w, h, _ = configFile(req.path)
	}
	if w <= 0 || h <= 0 {
		return false
	}
	return int64(w)*int64(h) > LargeImagePixels
}

// submit enqueues req on the small lane (routing happens in the worker
// so the caller never blocks on file IO).
func (p *Pool) submit(req decodeReq) {
	if p == nil {
		p = Default
	}
	if cancelled(req.ctx) {
		p.cancelled.Add(1)
		return
	}
	p.submitted.Add(1)
	if req.ctx != nil {
		select {
		case p.jobs <- req:
		case <-req.ctx.Done():
			p.cancelled.Add(1)
		}
		return
	}
	p.jobs <- req
}

// DecodeFile loads an image path on a worker and calls done (may be any goroutine).
// Callers must hop to the UI thread before mutating RenderObjects.
func (p *Pool) DecodeFile(path string, done func(Result)) {
	if p == nil {
		p = Default
	}
	p.submit(decodeReq{path: path, done: done})
}

// DecodeFileWithContext is DecodeFile bound to ctx (window close cancels):
// jobs dropped before or after decoding never invoke done.
func (p *Pool) DecodeFileWithContext(ctx context.Context, path string, done func(Result)) {
	if p == nil {
		p = Default
	}
	p.submit(decodeReq{ctx: ctx, path: path, done: done})
}

// DecodeBytes decodes in-memory bytes on a worker and calls done
// (may be any goroutine). Format is sniffed via the registry.
func (p *Pool) DecodeBytes(data []byte, done func(Result)) {
	if p == nil {
		p = Default
	}
	p.submit(decodeReq{data: data, hasData: true, done: done})
}

// DecodeBytesWithContext is DecodeBytes bound to ctx (window close cancels).
func (p *Pool) DecodeBytesWithContext(ctx context.Context, data []byte, done func(Result)) {
	if p == nil {
		p = Default
	}
	p.submit(decodeReq{ctx: ctx, data: data, hasData: true, done: done})
}

// Run enqueues fn onto the decode worker pool for asynchronous execution.
// A nil fn is a no-op. Results are delivered via the caller's own channel.
func (p *Pool) Run(fn func()) {
	if p == nil {
		p = Default
	}
	if fn == nil {
		return
	}
	p.submitted.Add(1)
	p.jobs <- decodeReq{fn: fn}
}

// RunWithContext enqueues fn unless ctx is already done; a func that
// waits in queue past cancel is dropped without running.
func (p *Pool) RunWithContext(ctx context.Context, fn func()) {
	if p == nil {
		p = Default
	}
	if fn == nil {
		return
	}
	if cancelled(ctx) {
		p.cancelled.Add(1)
		return
	}
	p.submitted.Add(1)
	if ctx != nil {
		select {
		case p.jobs <- decodeReq{ctx: ctx, fn: fn}:
		case <-ctx.Done():
			p.cancelled.Add(1)
		}
		return
	}
	p.jobs <- decodeReq{fn: fn}
}

// DecodeFileDefault uses Default pool.
func DecodeFile(path string, done func(Result)) {
	Default.DecodeFile(path, done)
}

// DecodeBytes uses Default pool.
func DecodeBytes(data []byte, done func(Result)) {
	Default.DecodeBytes(data, done)
}

// DecodeFileWithContext uses Default pool bound to ctx.
func DecodeFileWithContext(ctx context.Context, path string, done func(Result)) {
	Default.DecodeFileWithContext(ctx, path, done)
}

// DecodeBytesWithContext uses Default pool bound to ctx.
func DecodeBytesWithContext(ctx context.Context, data []byte, done func(Result)) {
	Default.DecodeBytesWithContext(ctx, data, done)
}

// configFile reports dimensions of path via the registry (header only).
func configFile(path string) (int, int, error) {
	if name := formatForExt(path); name != "" {
		if d, ok := lookupDecoder(name); ok {
			f, err := os.Open(filepath.Clean(path))
			if err != nil {
				return 0, 0, err
			}
			cfg, cerr := d.Config(f)
			_ = f.Close()
			if cerr != nil {
				return 0, 0, cerr
			}
			return cfg.Width, cfg.Height, nil
		}
	}
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return 0, 0, err
	}
	_, cfg, ok := sniffFormat(data)
	if !ok {
		return 0, 0, errors.New("io: unsupported image format")
	}
	return cfg.Width, cfg.Height, nil
}

// decodeFile runs the full decode of path via the registry.
// Reported dimensions come from the header peek (the routing view),
// falling back to the decoded buffer when the peek is unavailable.
func decodeFile(path string) Result {
	if name := formatForExt(path); name != "" {
		if d, ok := lookupDecoder(name); ok {
			w, h := peekFileDims(path, name, d)
			f, err := os.Open(filepath.Clean(path))
			if err != nil {
				return Result{Err: err}
			}
			img, derr := d.Decode(f)
			_ = f.Close()
			if derr != nil {
				return Result{Format: name, Err: derr}
			}
			return resultOfDims(name, img, w, h)
		}
	}
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return Result{Err: err}
	}
	return decodeBytes(data)
}

// decodeBytes runs the full decode of data via content sniffing.
func decodeBytes(data []byte) Result {
	if len(data) == 0 {
		return Result{Err: errors.New("io: empty image data")}
	}
	name, cfg, ok := sniffFormat(data)
	if !ok {
		return Result{Err: errors.New("io: unsupported image format")}
	}
	d, found := lookupDecoder(name)
	if !found {
		return Result{Format: name, Err: errors.New("io: unsupported image format")}
	}
	img, err := d.Decode(bytes.NewReader(data))
	if err != nil {
		return Result{Format: name, Err: err}
	}
	return resultOfDims(name, img, cfg.Width, cfg.Height)
}

// peekFileDims reports path dimensions via d (header only, no pixels).
func peekFileDims(path, name string, d Decoder) (int, int) {
	f, err := os.Open(filepath.Clean(path))
	if err != nil {
		return 0, 0
	}
	cfg, cerr := d.Config(f)
	_ = f.Close()
	if cerr != nil {
		return 0, 0
	}
	return cfg.Width, cfg.Height
}

// resultOfDims fills the dimension/large fields from the header peek
// (the routing view), falling back to the decoded buffer when the
// peek is unavailable (0 dimensions).
func resultOfDims(name string, img *render.ImageBuf, w, h int) Result {
	if img == nil {
		return Result{Format: name}
	}
	if w <= 0 || h <= 0 {
		w, h = img.Bounds()
	}
	return Result{
		Img:    img,
		Format: name,
		Width:  w,
		Height: h,
		Large:  int64(w)*int64(h) > LargeImagePixels,
	}
}

func init() {
	RegisterDecoder("png", Decoder{
		Decode: func(r io.Reader) (*render.ImageBuf, error) {
			img, err := png.Decode(r)
			if err != nil {
				return nil, err
			}
			return render.ImageBufFromImage(img), nil
		},
		Config: func(r io.Reader) (image.Config, error) {
			return png.DecodeConfig(r)
		},
	})
	RegisterDecoder("jpeg", Decoder{
		Decode: func(r io.Reader) (*render.ImageBuf, error) {
			img, err := jpeg.Decode(r)
			if err != nil {
				return nil, err
			}
			return render.ImageBufFromImage(img), nil
		},
		Config: func(r io.Reader) (image.Config, error) {
			return jpeg.DecodeConfig(r)
		},
	})
	RegisterDecoder("webp", Decoder{
		Decode: func(r io.Reader) (*render.ImageBuf, error) {
			img, err := webp.Decode(r)
			if err != nil {
				return nil, err
			}
			return render.ImageBufFromImage(img), nil
		},
		Config: func(r io.Reader) (image.Config, error) {
			return webp.DecodeConfig(r)
		},
	})
}
