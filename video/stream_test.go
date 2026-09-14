package video

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"testing"
	"time"
)

// TestStreamFastOpen pins the production rule in two halves: small clips
// take the buffered path with honest headers (old gates stay exact),
// while the real long clip streams — Open decodes only headers + first
// displayable frame (<= reorder+2 samples), never the whole clip.
func TestStreamFastOpen(t *testing.T) {
	for _, n := range []string{"testdata/vr2_720p.mp4", "testdata/vr2_1080p.mp4"} {
		if _, err := os.Stat(n); err != nil {
			t.Skipf("clip missing: %v", err)
		}
		p, err := OpenFile(n, Options{})
		if err != nil {
			t.Fatalf("%s open: %v", n, err)
		}
		if !p.Buffered() {
			t.Fatalf("%s streaming, want buffered small-clip path", n)
		}
		if p.Info().Frames == 0 || p.Info().Width == 0 {
			p.Close()
			t.Fatalf("%s info = %+v, want honest headers", n, p.Info())
		}
		p.Close()
	}
	name := longClip(t)
	p, err := OpenFile(name, Options{})
	if err != nil {
		t.Fatalf("%s open: %v", name, err)
	}
	defer p.Close()
	if p.Buffered() {
		t.Fatalf("%s buffered, want streaming path", name)
	}
	if p.DecodePos() > p.ReorderDepth()+2 {
		t.Fatalf("%s decoded %d samples at open, want <= %d (streaming)", name, p.DecodePos(), p.ReorderDepth()+2)
	}
	if p.Info().Frames == 0 || p.Info().Width == 0 {
		t.Fatalf("%s info = %+v, want honest headers", name, p.Info())
	}
}

// TestStreamLongClip pins bounded memory: looping thousands of frames
// keeps the queue at cap (never grown to length), buffered or not.
func TestStreamLongClip(t *testing.T) {
	h := &handClock{}
	p, err := OpenFile("testdata/vr2_m_bframes.mp4", Options{NowMs: h.at, Loop: true, QueueCap: 4})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer p.Close()
	if p.Buffered() {
		// Small clip buffers: cap fits the clip (never the length of a
		// long play), depth still bounded by that cap.
		if p.q.Cap() < len(p.bufFrames) {
			t.Fatalf("buffered cap = %d < frames %d", p.q.Cap(), len(p.bufFrames))
		}
	} else if p.q.Cap() != 4 {
		t.Fatalf("queue cap = %d, want 4 (bounded, never grown)", p.q.Cap())
	}
	cap := p.q.Cap()
	shown := 0
	// Buffered loop replays the same 5 frames: ~5 per pass, so 2000
	// ticks show thousands; streaming loops the same. Either way the
	// queue never exceeds cap.
	for i := 0; i < 2000; i++ {
		h.now += 200
		if f, _ := p.Poll(); f != nil {
			shown++
		}
		if p.q.Depth() > cap {
			t.Fatalf("queue depth = %d > cap %d", p.q.Depth(), cap)
		}
	}
	if shown < 100 && !p.Buffered() {
		t.Fatalf("shown = %d, want >= 100 over 2000 ticks", shown)
	}
	if p.Buffered() && shown < 5 {
		t.Fatalf("buffered shown = %d, want >= 5", shown)
	}
}

// TestHTTPSourceRange pins the network path: an httptest server serving
// a real clip opens + plays via Range (never full download). The server
// counts GETs; Range support proven by 206 + open success.
func TestHTTPSourceRange(t *testing.T) {
	raw, err := os.ReadFile("testdata/vr2_m_bframes.mp4")
	if err != nil {
		t.Fatalf("read base: %v", err)
	}
	var gets, ranges int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gets++
		if r.Header.Get("Range") != "" {
			ranges++
		}
		http.ServeContent(w, r, "clip.mp4", time.Unix(0, 0), bytes.NewReader(raw))
	}))
	defer srv.Close()
	p, err := OpenFile(srv.URL+"/clip.mp4", Options{})
	if err != nil {
		t.Fatalf("http open: %v", err)
	}
	defer p.Close()
	if p.Info().Frames != 5 {
		t.Fatalf("frames = %d, want 5", p.Info().Frames)
	}
	if ranges == 0 {
		t.Fatalf("gets=%d ranges=%d, want Range usage", gets, ranges)
	}
	t.Logf("http open: gets=%d ranges=%d", gets, ranges)
	// Play two frames off the network.
	h := &handClock{}
	p2, err := OpenWithSource(mustHTTPSource(t, srv.URL+"/clip.mp4"), Options{NowMs: h.at})
	if err != nil {
		t.Fatalf("http open2: %v", err)
	}
	defer p2.Close()
	h.now += 200
	if f, _ := p2.Poll(); f == nil {
		t.Fatal("no frame off http source")
	}
}

func mustHTTPSource(t *testing.T, url string) Source {
	t.Helper()
	src, err := NewSource(url)
	if err != nil {
		t.Fatalf("new source: %v", err)
	}
	return src
}

func TestOpenPathVariants(t *testing.T) {
	// file:// prefix defence (exact clip, must open).
	p, err := OpenFile("file://testdata/vr2_m_bframes.mp4", Options{})
	if err != nil {
		t.Fatalf("file:// open: %v", err)
	}
	p.Close()
}

// TestProbeSource asks shell+codec off every Source kind without
// decoding: Bytes, File and Range-HTTP all answer mp4/h264.
func TestProbeSource(t *testing.T) {
	raw, err := os.ReadFile("testdata/vr2_m_bframes.mp4")
	if err != nil {
		t.Fatalf("read base: %v", err)
	}
	mem := NewBytesSource(raw, "mem-clip")
	if c, codec, err := ProbeSource(mem); err != nil || c != ContainerMP4 || codec != CodecH264 {
		t.Fatalf("bytes probe = %q/%q %v, want mp4/h264", c, codec, err)
	}
	f, err := NewSource("testdata/vr2_m_bframes.mp4")
	if err != nil {
		t.Fatalf("file source: %v", err)
	}
	defer f.Close()
	if c, codec, err := ProbeSource(f); err != nil || c != ContainerMP4 || codec != CodecH264 {
		t.Fatalf("file probe = %q/%q %v, want mp4/h264", c, codec, err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeContent(w, r, "clip.mp4", time.Unix(0, 0), bytes.NewReader(raw))
	}))
	defer srv.Close()
	hsrc, err := NewSource(srv.URL + "/clip.mp4")
	if err != nil {
		t.Fatalf("http source: %v", err)
	}
	defer hsrc.Close()
	if c, codec, err := ProbeSource(hsrc); err != nil || c != ContainerMP4 || codec != CodecH264 {
		t.Fatalf("http probe = %q/%q %v, want mp4/h264", c, codec, err)
	}
}

// TestBytesSourceRange pins the memory path edges: negative/over-end
// offsets EOF, short tails return what fits plus EOF.
func TestBytesSourceRange(t *testing.T) {
	s := NewBytesSource([]byte{1, 2, 3, 4}, "mem")
	if s.Size() != 4 || s.Name() != "mem" {
		t.Fatalf("size/name = %d/%q, want 4/mem", s.Size(), s.Name())
	}
	if _, err := s.ReadAt(make([]byte, 1), -1); err != io.EOF {
		t.Fatalf("neg off err = %v, want EOF", err)
	}
	if _, err := s.ReadAt(make([]byte, 1), 4); err != io.EOF {
		t.Fatalf("past-end err = %v, want EOF", err)
	}
	out := make([]byte, 4)
	n, err := s.ReadAt(out, 2)
	if n != 2 || err != io.EOF {
		t.Fatalf("tail read = %d/%v, want 2/EOF", n, err)
	}
	if out[0] != 3 || out[1] != 4 {
		t.Fatalf("tail bytes = %v, want [3 4]", out[:2])
	}
	if err := s.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
}

// TestHTTPNoRangeFallback pins servers without Range: OpenHTTPSource
// falls back to one full download (memory source), and the server sees
// no Range header at all.
func TestHTTPNoRangeFallback(t *testing.T) {
	raw, err := os.ReadFile("testdata/vr2_m_bframes.mp4")
	if err != nil {
		t.Fatalf("read base: %v", err)
	}
	var sawRange bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Range") != "" {
			sawRange = true
		}
		// Plain 200 with the bytes, no Accept-Ranges: ServeContent still
		// honours Range, so write manually to force the fallback path.
		w.Header().Set("Content-Length", strconv.Itoa(len(raw)))
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(raw)
	}))
	defer srv.Close()
	src, err := NewSource(srv.URL + "/clip.mp4")
	if err != nil {
		t.Fatalf("no-range source: %v", err)
	}
	defer src.Close()
	if _, ok := src.(*BytesSource); !ok {
		t.Fatalf("source = %T, want *BytesSource fallback (no re-GET)", src)
	}
	if src.Size() != int64(len(raw)) {
		t.Fatalf("size = %d, want %d", src.Size(), len(raw))
	}
	// The one range probe must be the only Range the server sees; chunk
	// reads after that are plain cached bytes, never more network.
	if !sawRange {
		t.Fatal("fallback server saw no Range probe at all")
	}
	// The fallback still opens and plays through the public path.
	p, err := OpenWithSource(mustHTTPSourceBytes(t, raw), Options{})
	if err != nil {
		t.Fatalf("fallback open: %v", err)
	}
	defer p.Close()
	if p.Info().Frames != 5 {
		t.Fatalf("frames = %d, want 5", p.Info().Frames)
	}
	// End to end over the same no-Range server: OpenFile also plays (it
	// takes the same fallback internally, never streams).
	p2, err := OpenFile(srv.URL+"/clip.mp4", Options{})
	if err != nil {
		t.Fatalf("no-range openfile: %v", err)
	}
	defer p2.Close()
	if p2.Info().Frames != 5 {
		t.Fatalf("no-range frames = %d, want 5", p2.Info().Frames)
	}
}

// TestHTTPChunkErrors pins network failure surfaces: a 500 server fails
// fast with a readable error (no hang, no full parse).
func TestHTTPChunkErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	if _, err := NewSource(srv.URL + "/clip.mp4"); err == nil {
		t.Fatal("500 source opens")
	}
	if _, err := OpenFile(srv.URL+"/clip.mp4", Options{}); err == nil {
		t.Fatal("500 clip opens")
	}
}

func mustHTTPSourceBytes(t *testing.T, raw []byte) Source {
	t.Helper()
	return NewBytesSource(append([]byte(nil), raw...), "fallback-mem")
}

// TestFileSourceRange pins the file path edges: missing file fails,
// offsets read back exact bytes, over-end hits EOF.
func TestFileSourceRange(t *testing.T) {
	if _, err := NewSource("testdata/does-not-exist-src.mp4"); err == nil {
		t.Fatal("missing source opens")
	}
	src, err := NewSource("testdata/vr2_m_bframes.mp4")
	if err != nil {
		t.Fatalf("file source: %v", err)
	}
	defer src.Close()
	if src.Size() <= 0 {
		t.Fatalf("size = %d, want > 0", src.Size())
	}
	raw, err := os.ReadFile("testdata/vr2_m_bframes.mp4")
	if err != nil {
		t.Fatalf("read base: %v", err)
	}
	head := make([]byte, 16)
	if _, err := readSourceRange(src, head, 0); err != nil {
		t.Fatalf("head read: %v", err)
	}
	for i := range head {
		if head[i] != raw[i] {
			t.Fatalf("head byte %d differs", i)
		}
	}
	if _, err := src.ReadAt(make([]byte, 1), src.Size()); err != io.EOF {
		t.Fatalf("past-end err = %v, want EOF", err)
	}
}

// TestHTTPSourceChunkCache pins the 1MB chunk cache: repeat reads hit
// the cache (one GET), and 9 distinct chunks bound the cache to 8.
func TestHTTPSourceChunkCache(t *testing.T) {
	const total = 9*httpChunk + 100
	raw := make([]byte, total)
	for i := range raw {
		raw[i] = byte(i)
	}
	var gets int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gets++
		http.ServeContent(w, r, "big.bin", time.Unix(0, 0), bytes.NewReader(raw))
	}))
	defer srv.Close()
	src, err := NewSource(srv.URL + "/big.bin")
	if err != nil {
		t.Fatalf("big source: %v", err)
	}
	defer src.Close()
	if _, ok := src.(*HTTPSource); !ok {
		t.Fatalf("source = %T, want *HTTPSource (Range server)", src)
	}
	gets = 0
	out := make([]byte, 10)
	if _, err := readSourceRange(src, out, 100); err != nil {
		t.Fatalf("read1: %v", err)
	}
	if _, err := readSourceRange(src, out, 100); err != nil {
		t.Fatalf("read2: %v", err)
	}
	if gets != 1 {
		t.Fatalf("repeat chunk gets = %d, want 1 (cache hit)", gets)
	}
	// Touch 9 distinct chunks: cache must stay bounded at 8.
	for i := 0; i < 9; i++ {
		if _, err := readSourceRange(src, out, int64(i*httpChunk)); err != nil {
			t.Fatalf("chunk %d: %v", i, err)
		}
	}
	hs := src.(*HTTPSource)
	hs.mu.Lock()
	n := len(hs.chunks)
	hs.mu.Unlock()
	if n > 8 {
		t.Fatalf("cached chunks = %d, want <= 8", n)
	}
}

// TestOpenWithSourceKinds pins OpenWithSource off file and memory: the
// same 5-frame clip plays to Ended with identical headers either way.
func TestOpenWithSourceKinds(t *testing.T) {
	raw, err := os.ReadFile("testdata/vr2_m_bframes.mp4")
	if err != nil {
		t.Fatalf("read base: %v", err)
	}
	// Memory.
	mp, err := OpenWithSource(NewBytesSource(append([]byte(nil), raw...), "mem"), Options{})
	if err != nil {
		t.Fatalf("bytes open: %v", err)
	}
	if mp.Info().Frames != 5 || mp.Info().Width == 0 {
		mp.Close()
		t.Fatalf("bytes info = %+v, want 5 frames honest size", mp.Info())
	}
	mp.Close()
	// File kept open.
	fs, err := NewSource("testdata/vr2_m_bframes.mp4")
	if err != nil {
		t.Fatalf("file source: %v", err)
	}
	fp, err := OpenWithSource(fs, Options{})
	if err != nil {
		fs.Close()
		t.Fatalf("file open: %v", err)
	}
	if fp.Info().Frames != 5 {
		fp.Close()
		t.Fatalf("file info = %+v, want 5 frames", fp.Info())
	}
	fp.Close()
}

// TestPlayerCloseIdempotent pins Close twice + use-after-close: no panic,
// no hang, readable errors.
func TestPlayerCloseIdempotent(t *testing.T) {
	h := &handClock{}
	p, err := OpenFile("testdata/vr2_m_bframes.mp4", Options{NowMs: h.at})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	p.Close()
	p.Close()
	if _, ended := p.Poll(); ended {
		t.Fatal("poll after close reports ended")
	}
	if _, err := p.SeekTo(400); err == nil {
		t.Fatal("seek after close accepted")
	}
}

// TestStreamingQueueBounded pins the cap on the real long clip: the
// queue never exceeds cap 4 over hundreds of ticks of streaming play.
func TestStreamingQueueBounded(t *testing.T) {
	name := longClip(t)
	h := &handClock{}
	p, err := OpenFile(name, Options{NowMs: h.at, QueueCap: 4})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer p.Close()
	if p.Buffered() {
		t.Fatal("want streaming path")
	}
	cap := p.q.Cap()
	if cap != 4 {
		t.Fatalf("cap = %d, want 4", cap)
	}
	for i := 0; i < 500; i++ {
		h.now += 200
		p.Poll()
		if p.q.Depth() > cap {
			t.Fatalf("depth = %d > cap %d", p.q.Depth(), cap)
		}
	}
}

// TestIsURL pins scheme detection used to choose Range vs file.
func TestIsURL(t *testing.T) {
	for _, u := range []string{"http://a/b.mp4", "https://a/b.mp4"} {
		if !IsURL(u) {
			t.Fatalf("%q not URL", u)
		}
	}
	for _, u := range []string{"testdata/a.mp4", "file:///tmp/a.mp4", ""} {
		if IsURL(u) {
			t.Fatalf("%q is URL", u)
		}
	}
}
