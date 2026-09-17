package io_test

// T4 image-lane tests: registry, small-file decode from testdata,
// large-image single flight, window-close cancel, pool counters.
// Pixel files live in testdata/; the "t4stub"/"t4slow" formats are
// magic-gated test registrations that never match real image bytes.

import (
	"bytes"
	"context"
	"errors"
	"image"
	"io"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/energye/gpui/render"
	uio "github.com/energye/gpui/ui/io"
)

var (
	stubMagic = []byte("T4STUB01")
	slowMagic = []byte("T4SLOW01")

	stubLive atomic.Int64
	stubMax  atomic.Int64
)

func readAllNoT(r io.Reader, magic []byte) error {
	head := make([]byte, len(magic))
	n := 0
	for n < len(head) {
		m, err := r.Read(head[n:])
		n += m
		if err != nil {
			return err
		}
	}
	if !bytes.Equal(head, magic) {
		return errors.New("not stub format")
	}
	return nil
}

func stubDecode(r io.Reader, magic []byte, sleep time.Duration) (*render.ImageBuf, error) {
	if err := readAllNoT(r, magic); err != nil {
		return nil, err
	}
	cur := stubLive.Add(1)
	for {
		old := stubMax.Load()
		if cur <= old || stubMax.CompareAndSwap(old, cur) {
			break
		}
	}
	time.Sleep(sleep)
	stubLive.Add(-1)
	// Drain the rest so short payloads still decode.
	_, _ = io.Copy(io.Discard, r)
	return render.NewImageBuf(4, 4, render.FormatRGBA8)
}

func init() {
	uio.RegisterDecoder("t4stub", uio.Decoder{
		Decode: func(r io.Reader) (*render.ImageBuf, error) {
			return stubDecode(r, stubMagic, 30*time.Millisecond)
		},
		Config: func(r io.Reader) (image.Config, error) {
			if err := readAllNoT(r, stubMagic); err != nil {
				return image.Config{}, err
			}
			return image.Config{Width: 2000, Height: 1100}, nil
		},
	})
	uio.RegisterDecoder("t4slow", uio.Decoder{
		Decode: func(r io.Reader) (*render.ImageBuf, error) {
			return stubDecode(r, slowMagic, 200*time.Millisecond)
		},
		Config: func(r io.Reader) (image.Config, error) {
			if err := readAllNoT(r, slowMagic); err != nil {
				return image.Config{}, err
			}
			return image.Config{Width: 2000, Height: 1100}, nil
		},
	})
}

func containsName(t *testing.T, names []string, want string) {
	t.Helper()
	for _, n := range names {
		if n == want {
			return
		}
	}
	t.Fatalf("SupportedDecoders %v lacks %q", names, want)
}

func waitResult(t *testing.T, ch <-chan uio.Result) uio.Result {
	t.Helper()
	select {
	case r := <-ch:
		return r
	case <-time.After(5 * time.Second):
		t.Fatal("timeout waiting for decode")
		return uio.Result{}
	}
}

func TestRegistry_BuiltinFormats(t *testing.T) {
	names := uio.SupportedDecoders()
	containsName(t, names, "png")
	containsName(t, names, "jpeg")
	containsName(t, names, "webp")
}

func TestRegistry_Replaceable(t *testing.T) {
	uio.RegisterDecoder("t4probe", uio.Decoder{
		Decode: func(r io.Reader) (*render.ImageBuf, error) {
			return nil, errors.New("t4probe v1")
		},
		Config: func(r io.Reader) (image.Config, error) {
			return image.Config{}, errors.New("t4probe v1")
		},
	})
	uio.RegisterDecoder("t4probe", uio.Decoder{
		Decode: func(r io.Reader) (*render.ImageBuf, error) {
			return nil, errors.New("t4probe v2")
		},
		Config: func(r io.Reader) (image.Config, error) {
			return image.Config{}, errors.New("t4probe v2")
		},
	})
	containsName(t, uio.SupportedDecoders(), "t4probe")
	done := make(chan uio.Result, 1)
	uio.DecodeBytes([]byte("T4PROBE-no-magic"), func(r uio.Result) { done <- r })
	res := waitResult(t, done)
	if res.Err == nil {
		t.Fatal("expected error for unknown bytes")
	}
}

func TestDecodeFile_SmallPNG(t *testing.T) {
	p := uio.NewPool(2)
	defer p.Close()
	done := make(chan uio.Result, 1)
	p.DecodeFile(filepath.Join("testdata", "small.png"), func(r uio.Result) { done <- r })
	res := waitResult(t, done)
	if res.Err != nil {
		t.Fatalf("decode: %v", res.Err)
	}
	if res.Format != "png" {
		t.Fatalf("format=%q want png", res.Format)
	}
	if res.Width != 16 || res.Height != 12 {
		t.Fatalf("size=%dx%d want 16x12", res.Width, res.Height)
	}
	if res.Large {
		t.Fatal("16x12 must not route as large")
	}
	if res.Img == nil {
		t.Fatal("missing image")
	}
}

func TestDecodeFile_SmallJPEG(t *testing.T) {
	p := uio.NewPool(2)
	defer p.Close()
	done := make(chan uio.Result, 1)
	p.DecodeFile(filepath.Join("testdata", "small.jpg"), func(r uio.Result) { done <- r })
	res := waitResult(t, done)
	if res.Err != nil {
		t.Fatalf("decode: %v", res.Err)
	}
	if res.Format != "jpeg" {
		t.Fatalf("format=%q want jpeg", res.Format)
	}
	if res.Width != 16 || res.Height != 12 {
		t.Fatalf("size=%dx%d want 16x12", res.Width, res.Height)
	}
	if res.Img == nil {
		t.Fatal("missing image")
	}
}

func TestDecodeBytes_SmallPNG(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("testdata", "small.png"))
	if err != nil {
		t.Fatalf("read testdata: %v", err)
	}
	p := uio.NewPool(2)
	defer p.Close()
	done := make(chan uio.Result, 1)
	p.DecodeBytes(data, func(r uio.Result) { done <- r })
	res := waitResult(t, done)
	if res.Err != nil {
		t.Fatalf("decode: %v", res.Err)
	}
	if res.Format != "png" || res.Width != 16 || res.Height != 12 || res.Img == nil {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func TestLarge_SingleFlight(t *testing.T) {
	stubLive.Store(0)
	stubMax.Store(0)
	p := uio.NewPool(2)
	defer p.Close()
	const jobs = 4
	done := make(chan uio.Result, jobs)
	payload := append(append([]byte{}, stubMagic...), bytes.Repeat([]byte("."), 64)...)
	for i := 0; i < jobs; i++ {
		p.DecodeBytes(payload, func(r uio.Result) { done <- r })
	}
	for i := 0; i < jobs; i++ {
		res := waitResult(t, done)
		if res.Err != nil {
			t.Fatalf("large decode: %v", res.Err)
		}
		if !res.Large {
			t.Fatal("stub 2000x1100 must flag Large")
		}
	}
	if got := stubMax.Load(); got != 1 {
		t.Fatalf("large concurrency=%d want 1 (single flight)", got)
	}
	if got := p.Stats().LargeRouted; got < jobs {
		t.Fatalf("large_routed=%d want >=%d", got, jobs)
	}
}

func TestLarge_FileSniffFallback(t *testing.T) {
	p := uio.NewPool(2)
	defer p.Close()
	done := make(chan uio.Result, 1)
	p.DecodeFile(filepath.Join("testdata", "big.t4stub"), func(r uio.Result) { done <- r })
	res := waitResult(t, done)
	if res.Err != nil {
		t.Fatalf("decode: %v", res.Err)
	}
	if res.Format != "t4stub" || !res.Large {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func TestCancel_PreCancelled(t *testing.T) {
	p := uio.NewPool(2)
	defer p.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	fired := make(chan uio.Result, 1)
	data, err := os.ReadFile(filepath.Join("testdata", "small.png"))
	if err != nil {
		t.Fatalf("read testdata: %v", err)
	}
	p.DecodeBytesWithContext(ctx, data, func(r uio.Result) { fired <- r })
	select {
	case r := <-fired:
		t.Fatalf("cancelled job must not call back, got %+v", r)
	case <-time.After(200 * time.Millisecond):
	}
	st := p.Stats()
	if st.Cancelled < 1 {
		t.Fatalf("cancelled=%d want >=1", st.Cancelled)
	}
	if st.Completed != 0 {
		t.Fatalf("completed=%d want 0", st.Completed)
	}
}

func TestCancel_SuppressesInFlightCallback(t *testing.T) {
	p := uio.NewPool(2)
	defer p.Close()
	ctx, cancel := context.WithCancel(context.Background())
	fired := make(chan uio.Result, 1)
	payload := append(append([]byte{}, slowMagic...), bytes.Repeat([]byte("."), 64)...)
	p.DecodeBytesWithContext(ctx, payload, func(r uio.Result) { fired <- r })
	time.Sleep(50 * time.Millisecond)
	cancel()
	select {
	case r := <-fired:
		t.Fatalf("cancelled in-flight job must not call back, got %+v", r)
	case <-time.After(time.Second):
	}
	if got := p.Stats().Cancelled; got < 1 {
		t.Fatalf("cancelled=%d want >=1", got)
	}
}

func TestStats_Counts(t *testing.T) {
	p := uio.NewPool(2)
	defer p.Close()
	data, err := os.ReadFile(filepath.Join("testdata", "small.png"))
	if err != nil {
		t.Fatalf("read testdata: %v", err)
	}
	done := make(chan uio.Result, 1)
	p.DecodeBytes(data, func(r uio.Result) { done <- r })
	_ = waitResult(t, done)
	st := p.Stats()
	if st.Submitted < 1 || st.Completed < 1 {
		t.Fatalf("stats=%+v want submitted/completed >=1", st)
	}
}
