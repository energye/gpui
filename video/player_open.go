package video

import (
	"fmt"
	"os"
	"strings"
)

func (p *Player) announceReady() {
	p.readyDo.Do(func() { close(p.readyCh) })
}

// OpenFile opens path and starts the background decoder. The player owns
// nothing caller-side; Close must be called. Decoding runs on ffmpeg
// (video/ffmpeg): its demuxer opens the path directly, so any container,
// codec, protocol or URL the bundled libgpui_ffmpeg supports plays —
// local files, http(s), rtmp and friends included. Missing files keep
// their os error; unopenable inputs fail readably, never silently.
func OpenFile(path string, opt Options) (*Player, error) {
	// Same cleaning as NewSource: drag-drop file://, spaces, brackets.
	p := strings.TrimSpace(path)
	p = strings.Trim(p, "<>")
	if strings.HasPrefix(p, "file://") {
		p = strings.TrimPrefix(p, "file://")
	}
	if p == "" {
		return nil, fmt.Errorf("%w: empty path", ErrBadClip)
	}
	if !IsURL(p) {
		if _, err := os.Stat(p); err != nil {
			return nil, err
		}
	}
	return openFFmpeg(p, opt)
}

// OpenWithSource opens a kept-open Source (file, memory, HTTP Range) and
// starts the background decoder. Caller must not Close src after success
// (Player owns it); on failure src is left open for the caller to close.
//
// Backend note: ffmpeg opens the path/URL itself. File and URL sources
// route by name; memory sources are spooled to a temp .mp4 which is
// removed on Close, so old callers keep working.
func OpenWithSource(src Source, opt Options) (*Player, error) {
	if src == nil {
		return nil, fmt.Errorf("%w: nil source", ErrBadClip)
	}
	if bs, ok := src.(*BytesSource); ok {
		f, err := os.CreateTemp("", "gpui-ffmpeg-*.mp4")
		if err != nil {
			return nil, fmt.Errorf("video: spool memory source %s: %w", src.Name(), err)
		}
		fname := f.Name()
		if _, err := f.Write(bs.b); err != nil {
			f.Close()
			os.Remove(fname)
			return nil, fmt.Errorf("video: spool memory source %s: %w", src.Name(), err)
		}
		if err := f.Close(); err != nil {
			os.Remove(fname)
			return nil, fmt.Errorf("video: spool memory source %s: %w", src.Name(), err)
		}
		p, err := openFFmpeg(fname, opt)
		if err != nil {
			os.Remove(fname)
			return nil, err
		}
		p.ffTemp = fname
		src.Close()
		return p, nil
	}
	p, err := openFFmpeg(src.Name(), opt)
	if err != nil {
		return nil, err
	}
	// The ffmpeg demuxer owns its own handle; the passed Source is no
	// longer needed, so close it here to keep the old ownership rule
	// (caller must not close after success) leak-free.
	src.Close()
	return p, nil
}

func (p *Player) Close() {
	p.mu.Lock()
	already := p.closed
	p.closed = true
	p.mu.Unlock()
	select {
	case <-p.stopCh:
	default:
		if !already {
			close(p.stopCh)
		}
	}
	p.q.Close()
	<-p.doneCh
	p.dmu.Lock()
	ffdec := p.ffdec
	p.ffdec = nil
	ffaud := p.ffaud
	p.ffaud = nil
	p.dmu.Unlock()
	for _, fr := range p.q.Drain() {
		if fr == nil {
			continue
		}
		p.releasePix(fr.Pix)
	}
	p.mu.Lock()
	last := p.lastPix
	p.lastPix = nil
	p.mu.Unlock()
	p.releasePix(last)
	if ffdec != nil {
		ffdec.Close()
	}
	if ffaud != nil {
		ffaud.Close()
	}
	if p.aq != nil {
		p.aq.Close()
	}
	p.mu.Lock()
	ffTemp := p.ffTemp
	p.ffTemp = ""
	p.mu.Unlock()
	if ffTemp != "" {
		os.Remove(ffTemp)
	}
}
