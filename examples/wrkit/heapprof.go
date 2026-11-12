// Package wrkit heap-profile helper for soak diagnostics.
//
// StartHeapProfiles writes heap and goroutine pprof snapshots at fixed
// wall-clock offsets so a long run can be compared with
// `go tool pprof -base <early>.pprof <late>.pprof`. Each heap snapshot forces
// a GC first: the diff then shows live (in-use) growth instead of garbage.
// Disabled unless dir is non-empty, so normal window runs are unaffected.
package wrkit

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"sync"
	"time"
)

// StartHeapProfiles starts the snapshot goroutine. The returned stop funcs
// ends it early; snapshots already due are skipped.
func StartHeapProfiles(dir string, atSeconds ...float64) (stop func()) {
	stop = func() {}
	if dir == "" || len(atSeconds) == 0 {
		return stop
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "heapprof: mkdir %s: %v\n", dir, err)
		return stop
	}
	done := make(chan struct{})
	var once sync.Once
	stop = func() { once.Do(func() { close(done) }) }

	go func() {
		t0 := time.Now()
		for _, sec := range atSeconds {
			wait := time.Duration(sec*float64(time.Second)) - time.Since(t0)
			if wait > 0 {
				timer := time.NewTimer(wait)
				select {
				case <-timer.C:
				case <-done:
					timer.Stop()
					return
				}
			}
			stamp := int(sec)
			writeProfile(filepath.Join(dir, fmt.Sprintf("heap_%03ds.pprof", stamp)), func(f *os.File) error {
				runtime.GC()
				return pprof.WriteHeapProfile(f)
			})
			writeProfile(filepath.Join(dir, fmt.Sprintf("goroutine_%03ds.pprof", stamp)), func(f *os.File) error {
				return pprof.Lookup("goroutine").WriteTo(f, 0)
			})
			fmt.Fprintf(os.Stderr, "heapprof: snapshot t=%ds dir=%s\n", stamp, dir)
		}
	}()
	return stop
}

func writeProfile(path string, fn func(*os.File) error) {
	f, err := os.Create(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "heapprof: create %s: %v\n", path, err)
		return
	}
	defer f.Close()
	if err := fn(f); err != nil {
		fmt.Fprintf(os.Stderr, "heapprof: write %s: %v\n", path, err)
	}
}
