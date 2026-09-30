package ffmpeg

// L1 符号存在性：包内 RegisterLibFunc 绑的每个名字，库里必须有。
//
// 大白话：Go 代码里写了要调 so 的哪些函数，这个测试就把名单扫出来，
// 一个个问 so 有没有。名字对不上（拼错、版本换了符号没了）这里直接报出来，
// 不用等到真调那一下才崩。数据符号走 TestDataConst，变参走 TestVariadicGo，
// 这里只查函数符号。7 个 x86-linux 专有符号在别的平台允许缺失。

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/ebitengine/purego"
)

// x86LinuxOnly 是只在 x86-linux 库里有的符号（VDPAU 六件 + x86 重采样一件）。
// 别的平台（ARM/386/win/mac）库里没有，缺了不算错。
var x86LinuxOnly = map[string]bool{
	"av_alloc_vdpaucontext":           true,
	"av_vdpau_alloc_context":          true,
	"av_vdpau_bind_context":           true,
	"av_vdpau_get_surface_parameters": true,
	"av_vdpau_hwaccel_get_render2":    true,
	"av_vdpau_hwaccel_set_render2":    true,
	"swri_resample_dsp_x86_init":      true,
}

var regFuncName = regexp.MustCompile(`RegisterLibFunc\([^,]+,\s*h\s*,\s*"([^"]+)"`)

// boundNames 扫包内全部非 test 源码，把 RegisterLibFunc 的名字全收回来。
// 名单从源码实时扫，不另存清单，加新绑定不用改测试。
func boundNames(t *testing.T) []string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("ffmpeg: caller unknown")
	}
	entries, err := os.ReadDir(filepath.Dir(file))
	if err != nil {
		t.Fatalf("ffmpeg: readdir: %v", err)
	}
	seen := map[string]bool{}
	for _, e := range entries {
		name := e.Name()
		if filepath.Ext(name) != ".go" || strings.HasSuffix(name, "_test.go") {
			continue
		}
		buf, err := os.ReadFile(filepath.Join(filepath.Dir(file), name))
		if err != nil {
			t.Fatalf("ffmpeg: read %s: %v", name, err)
		}
		for _, m := range regFuncName.FindAllSubmatch(buf, -1) {
			seen[string(m[1])] = true
		}
	}
	out := make([]string, 0, len(seen))
	for s := range seen {
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

// TestSymbolCover 逐个 Dlsym 断言绑定名在库里存在。
// base 和 full 各跑一遍（GPUI_FFMPEG_VARIANT=full 切 full）。
func TestSymbolCover(t *testing.T) {
	if err := ensureLoaded(); err != nil {
		t.Skipf("lib missing: %v", err)
	}
	loadMu.Lock()
	h := libHandle
	loadMu.Unlock()
	if h == 0 {
		t.Skipf("lib missing: %s", LibPath())
	}
	names := boundNames(t)
	if len(names) < 900 {
		t.Fatalf("bound names = %d, want >= 900 (scanner broken?)", len(names))
	}
	strict := runtime.GOOS == "linux" && runtime.GOARCH == "amd64"
	var missing, skipped []string
	for _, n := range names {
		if _, err := purego.Dlsym(h, n); err != nil {
			if !strict && x86LinuxOnly[n] {
				skipped = append(skipped, n)
				continue
			}
			missing = append(missing, n)
		}
	}
	if len(skipped) > 0 {
		t.Logf("optional symbols absent on %s/%s: %v", runtime.GOOS, runtime.GOARCH, skipped)
	}
	if len(missing) > 0 {
		t.Fatalf("lib %s missing %d bound symbols: %v", LibPath(), len(missing), missing)
	}
	t.Logf("lib %s: %d bound symbols all present", LibPath(), len(names))
}
