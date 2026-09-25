package video

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// Source is a random-access byte source for containers: local files,
// memory, or network (HTTP Range). It mirrors io.ReaderAt so mp4
// parsing reuses the same path, plus Size for bounds and Close.
// Production rule: Open parses only headers (moov), samples stream
// on demand — never full-download for length.
type Source interface {
	io.ReaderAt
	Size() int64
	Close() error
	Name() string
}

// IsURL reports http(s) sources (no os.Stat, Range GET instead).
func IsURL(path string) bool {
	return strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://")
}

// NewSource creates a Source for a path or URL: http(s) -> Range
// source, otherwise a local file. Caller must Close.
func NewSource(path string) (Source, error) {
	if IsURL(path) {
		return OpenHTTPSource(path)
	}
	// Strip file:// from drag-drop (defence; normal Files are clean).
	p := strings.TrimSpace(path)
	p = strings.Trim(p, "<>")
	if strings.HasPrefix(p, "file://") {
		p = strings.TrimPrefix(p, "file://")
	}
	f, err := os.Open(p)
	if err != nil {
		return nil, err
	}
	fi, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}
	return &FileSource{f: f, size: fi.Size(), name: p}, nil
}

// FileSource is an OS file as a Source (Pread, thread-safe).
type FileSource struct {
	f    *os.File
	size int64
	name string
}

func (s *FileSource) ReadAt(p []byte, off int64) (int, error) { return s.f.ReadAt(p, off) }
func (s *FileSource) Size() int64                             { return s.size }
func (s *FileSource) Name() string                            { return s.name }
func (s *FileSource) Close() error                            { return s.f.Close() }

// BytesSource is memory as a Source (tests, full-fallback).
type BytesSource struct {
	b    []byte
	name string
}

func NewBytesSource(b []byte, name string) *BytesSource {
	return &BytesSource{b: b, name: name}
}

func (s *BytesSource) ReadAt(p []byte, off int64) (int, error) {
	if off < 0 || off >= int64(len(s.b)) {
		return 0, io.EOF
	}
	n := copy(p, s.b[off:])
	if n < len(p) {
		return n, io.EOF
	}
	return n, nil
}
func (s *BytesSource) Size() int64  { return int64(len(s.b)) }
func (s *BytesSource) Name() string { return s.name }
func (s *BytesSource) Close() error { return nil }

// HTTPSource streams via HTTP Range with chunk cache: Size from HEAD
// (or Range 0-0), data in 1MB cached chunks so per-sample ReadAt
// coalesces into few GETs. No Range -> full download fallback once.
type HTTPSource struct {
	url    string
	client *http.Client
	size   int64

	mu     sync.Mutex
	chunks map[int64][]byte // chunk index -> 1MB data
}

const httpChunk = 1 << 20

func OpenHTTPSource(url string) (Source, error) {
	c := &http.Client{Timeout: 30 * time.Second}
	// Size via HEAD, fallback to Range probe.
	size := int64(-1)
	if req, err := http.NewRequest("HEAD", url, nil); err == nil {
		if resp, err := c.Do(req); err == nil {
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if resp.ContentLength > 0 {
				size = resp.ContentLength
			}
		}
	}
	// HEAD tells size but not Range support: verify with one byte probe.
	// 206 keeps streaming; 200 means the server ignores Range, so keep
	// that body as the full download (never re-GET).
	if size > 0 {
		req, err := http.NewRequest("GET", url, nil)
		if err == nil {
			req.Header.Set("Range", "bytes=0-0")
			if resp, err := c.Do(req); err == nil {
				defer resp.Body.Close()
				if resp.StatusCode == http.StatusPartialContent {
					if cr := resp.Header.Get("Content-Range"); cr != "" {
						if i := strings.LastIndex(cr, "/"); i >= 0 {
							var total int64
							if _, err := fmt.Sscanf(cr[i+1:], "%d", &total); err == nil && total > 0 {
								size = total
							}
						}
					}
					io.Copy(io.Discard, resp.Body)
				} else {
					body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<30))
					if err != nil {
						return nil, err
					}
					if int64(len(body)) > 0 {
						return NewBytesSource(body, url), nil
					}
					// Empty 200 with known HEAD size: fall through to
					// full download below.
				}
			}
		}
	}
	if size <= 0 {
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Range", "bytes=0-0")
		resp, err := c.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()
		if resp.StatusCode == http.StatusPartialContent {
			// Content-Range: bytes 0-0/12345
			if cr := resp.Header.Get("Content-Range"); cr != "" {
				if i := strings.LastIndex(cr, "/"); i >= 0 {
					var total int64
					if _, err := fmt.Sscanf(cr[i+1:], "%d", &total); err == nil && total > 0 {
						size = total
					}
				}
			}
		} else if resp.ContentLength > 0 {
			// No Range: the body we just read IS the content — keep it
			// (never re-GET).
			body, err := io.ReadAll(io.LimitReader(resp.Body, 2<<30))
			if err != nil {
				return nil, err
			}
			return NewBytesSource(body, url), nil
		}
	}
	if size <= 0 {
		// Last resort: full download.
		full, err := downloadFull(c, url)
		if err != nil {
			return nil, fmt.Errorf("video: http size unknown %s: %w", url, err)
		}
		return NewBytesSource(full, url), nil
	}
	return &HTTPSource{url: url, client: c, size: size, chunks: map[int64][]byte{}}, nil
}

func downloadFull(c *http.Client, url string) ([]byte, error) {
	resp, err := c.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return nil, fmt.Errorf("video: http %d %s", resp.StatusCode, url)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 2<<30))
}

func (s *HTTPSource) Size() int64  { return s.size }
func (s *HTTPSource) Name() string { return s.url }
func (s *HTTPSource) Close() error { return nil }

func (s *HTTPSource) ReadAt(p []byte, off int64) (int, error) {
	if off < 0 || off >= s.size {
		return 0, io.EOF
	}
	// Clamp to size.
	if max := s.size - off; int64(len(p)) > max {
		p = p[:max]
	}
	total := 0
	for len(p) > 0 {
		chunkIdx := (off + int64(total)) / httpChunk
		chunkOff := (off + int64(total)) % httpChunk
		chunk, err := s.fetchChunk(chunkIdx)
		if err != nil {
			if total == 0 {
				return 0, err
			}
			return total, nil
		}
		n := copy(p, chunk[chunkOff:])
		total += n
		p = p[n:]
		if n == 0 {
			break
		}
	}
	if int64(total) < s.size-off && total == 0 {
		return total, io.EOF
	}
	return total, nil
}

func (s *HTTPSource) fetchChunk(idx int64) ([]byte, error) {
	s.mu.Lock()
	if b, ok := s.chunks[idx]; ok {
		s.mu.Unlock()
		return b, nil
	}
	s.mu.Unlock()
	start := idx * httpChunk
	end := start + httpChunk - 1
	if end >= s.size {
		end = s.size - 1
	}
	req, err := http.NewRequest("GET", s.url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", start, end))
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusPartialContent && resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("video: http range %d %s", resp.StatusCode, s.url)
	}
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	s.mu.Lock()
	// Bound cache: keep last 8 chunks (~8MB), drop oldest.
	if len(s.chunks) >= 8 {
		for k := range s.chunks {
			delete(s.chunks, k)
			break
		}
	}
	s.chunks[idx] = b
	s.mu.Unlock()
	return b, nil
}

// readSourceRange reads exactly len(p) from src at off (short reads
// retried; EOF only when source exhausted).
func readSourceRange(src Source, p []byte, off int64) (int, error) {
	total := 0
	for total < len(p) {
		n, err := src.ReadAt(p[total:], off+int64(total))
		total += n
		if err != nil {
			if err == io.EOF && total == len(p) {
				return total, nil
			}
			return total, err
		}
		if n == 0 {
			break
		}
	}
	return total, nil
}

var _ = bytes.MinRead
