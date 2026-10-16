//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

//go:build (windows || linux) && !(js && wasm)

package gles

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/energye/gpui/gpu/gwgpu/naga/glsl"
	"github.com/energye/gpui/gpu/hal"
)

// shaderDiskFormatVersion bounds staleness of on-disk shader translations.
// BUMP IT whenever naga's parse/lower/codegen or TranslationInfo changes
// shape: entries keyed by source+options stay valid across those changes
// otherwise, and a stale translation would be silently served.
//
// v2: the coordinate-space flip moved into the GpuiTargetAdjust block
// (glsl.WriterFlagTargetCoordinateAdjust). A v1 entry has the baked -1.0 and no
// block declaration, which would silently keep the pre-block behaviour.
const shaderDiskFormatVersion = 2

// Env knobs for the disk layer (internal defaults, operational overrides):
//
//	GPUI_SHADER_CACHE=0|off|false   disable; keep only the in-process memo
//	GPUI_SHADER_CACHE_DIR=<dir>     override the cache directory
const (
	shaderCacheEnvDisable = "GPUI_SHADER_CACHE"
	shaderCacheEnvDir     = "GPUI_SHADER_CACHE_DIR"
)

// shaderDiskCache persists WGSL→GLSL translations across processes.
//
// The in-process memo (shaderCache in shader.go) dies with the process, so
// every start re-runs naga parse→lower→codegen for every shader (measured
// ~180ms and ~3.3M allocations in the first second of a spinning-icon
// window, 96% in naga type resolution). This layer stores the same
// translation result — GLSL text plus reflection info — content-addressed on
// disk, so a warm start returns from one file read.
//
// Keying and lifecycle mirror Mesa's shader disk cache and Skia's program
// cache: key = hash(source + options), value = translation output, writes go
// through a temp file plus atomic rename (concurrent processes may race
// harmlessly), and any read/decode failure is treated as a miss rather than
// an error path.
type shaderDiskCache struct {
	dir    string
	hits   atomic.Uint64
	misses atomic.Uint64
	writes atomic.Uint64
	errors atomic.Uint64
}

func newShaderDiskCache(dir string) *shaderDiskCache {
	return &shaderDiskCache{dir: dir}
}

// fileName is the content-addressed file name for a canonical shader key.
func (c *shaderDiskCache) fileName(key string) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("v%d\x00%s", shaderDiskFormatVersion, key)))
	return hex.EncodeToString(sum[:20]) + ".json"
}

// shaderDiskEntry is the on-disk representation of one translation.
// Decoding is strict (unknown fields rejected), so a field rename or removal
// in TranslationInfo degrades to a cache miss instead of a half-decoded entry.
type shaderDiskEntry struct {
	Format int                  `json:"format"`
	GLSL   string               `json:"glsl"`
	Info   glsl.TranslationInfo `json:"info"`
}

// get returns the stored translation for key. Any failure — missing file,
// unreadable data, wrong format, undecodable shape — is a miss.
func (c *shaderDiskCache) get(key string) (string, glsl.TranslationInfo, bool) {
	b, err := os.ReadFile(filepath.Join(c.dir, c.fileName(key)))
	if err != nil {
		if !os.IsNotExist(err) {
			c.errors.Add(1)
			hal.Logger().Debug("gles: shader disk cache read failed", "err", err)
		}
		c.misses.Add(1)
		return "", glsl.TranslationInfo{}, false
	}
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.DisallowUnknownFields()
	var e shaderDiskEntry
	if err := dec.Decode(&e); err != nil || e.Format != shaderDiskFormatVersion || e.GLSL == "" {
		c.errors.Add(1)
		c.misses.Add(1)
		hal.Logger().Debug("gles: shader disk cache entry rejected", "err", err)
		return "", glsl.TranslationInfo{}, false
	}
	c.hits.Add(1)
	return e.GLSL, e.Info, true
}

// put stores the translation for key. Best effort: failures only bump the
// error counter, the in-process memo still serves the current run.
func (c *shaderDiskCache) put(key, glslCode string, info glsl.TranslationInfo) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(shaderDiskEntry{Format: shaderDiskFormatVersion, GLSL: glslCode, Info: info}); err != nil {
		c.errors.Add(1)
		return
	}
	tmp, err := os.CreateTemp(c.dir, ".tmp-*")
	if err != nil {
		c.errors.Add(1)
		hal.Logger().Debug("gles: shader disk cache temp file failed", "err", err)
		return
	}
	tmpName := tmp.Name()
	_, werr := tmp.Write(buf.Bytes())
	cerr := tmp.Close()
	if werr != nil || cerr != nil {
		_ = os.Remove(tmpName)
		c.errors.Add(1)
		return
	}
	if err := os.Rename(tmpName, filepath.Join(c.dir, c.fileName(key))); err != nil {
		_ = os.Remove(tmpName)
		c.errors.Add(1)
		hal.Logger().Debug("gles: shader disk cache rename failed", "err", err)
		return
	}
	c.writes.Add(1)
}

// resolveShaderDiskCache builds the process cache from env + user cache dir.
// Returns nil when disabled or when the directory cannot be prepared.
func resolveShaderDiskCache() *shaderDiskCache {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(shaderCacheEnvDisable))) {
	case "0", "off", "false":
		return nil
	}
	dir := os.Getenv(shaderCacheEnvDir)
	if dir == "" {
		base, err := os.UserCacheDir()
		if err != nil || base == "" {
			return nil
		}
		dir = filepath.Join(base, "gpui", "shaders")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		hal.Logger().Debug("gles: shader disk cache dir unavailable", "dir", dir, "err", err)
		return nil
	}
	return newShaderDiskCache(dir)
}

// shaderDiskCacheFor resolves the process-wide disk cache once, lazily, so
// tests and embedders can set the env before the first shader compile.
// Tests swap the variable itself to redirect or stub the cache.
var shaderDiskCacheFor = func() func() *shaderDiskCache {
	var once sync.Once
	var cache *shaderDiskCache
	return func() *shaderDiskCache {
		once.Do(func() { cache = resolveShaderDiskCache() })
		return cache
	}
}()

// shaderDiskCacheStats reports disk-layer counters (tests/observability).
func shaderDiskCacheStats() (hits, misses, writes, errors uint64) {
	c := shaderDiskCacheFor()
	if c == nil {
		return 0, 0, 0, 0
	}
	return c.hits.Load(), c.misses.Load(), c.writes.Load(), c.errors.Load()
}
