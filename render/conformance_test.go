//go:build linux && !nogpu

package render_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/energye/gpui/gpu/hal"
	gputypes "github.com/energye/gpui/gpu/types"
	"github.com/energye/gpui/gpu/webgpu"
	"github.com/energye/gpui/render"
	rendgpu "github.com/energye/gpui/render/gpu"
	"github.com/energye/gpui/render/internal/conformance"

	// Register both backends: webgpu accelerator (native) and pure-Go GL.
	_ "github.com/energye/gpui/gpu/gwgpu/gles"
)

// P0.4 dual-backend conformance harness.
//
// One command runs all scenes on both backends:
//
//	WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so \
//	GPUI_CONFORMANCE_OUT=$PWD/docs/render_gpu_converge/baseline/conformance \
//	  go test ./render -run '^TestConformanceDualBackend$' -count=1 -v
//
// Run it from the repo root: the test process runs inside render/, so
// GPUI_CONFORMANCE_OUT must be absolute.
//
// Each backend runs in its own subprocess so devices, caches, and global
// accelerator state never mix. Scenes and expectations live in
// render/testdata/conformance/*.json; no assertion data is hardcoded here.
//
// Backend selection: GPUI_BACKEND today only routes the present/swapchain
// layer (render/present_swapchain.go). Context-level GPU work always goes
// through the registered accelerator, so the harness passes
// GPUI_CONFORMANCE_BACKEND and wires a boxed device explicitly:
//   - native: a native webgpu device bound via render/gpu.SetDeviceProvider
//     (requires WGPU_NATIVE_PATH; unset means the backend is unavailable);
//   - go: a boxed pure-Go GL device bound via SetAcceleratorDeviceProvider,
//     using a minimal, never-presented X11 window as the GL context provider.
func TestConformanceDualBackend(t *testing.T) {
	if spec := os.Getenv("GPUI_CONFORMANCE_MUTATE"); spec != "" {
		runConformanceNegativeControl(t, spec)
		return
	}

	outRoot := os.Getenv("GPUI_CONFORMANCE_OUT")
	if outRoot == "" {
		outRoot = filepath.Join("tmp", "conformance")
	}
	if err := os.MkdirAll(outRoot, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", outRoot, err)
	}

	backends := []string{"native", "go"}
	results := make(map[string]map[string]*conformance.Report, len(backends))
	var failed, unavailable []string

	for _, be := range backends {
		dir := t.TempDir()
		logPath := filepath.Join(dir, "run.txt")
		reports, runLog, err := runBackendSubprocess(t, be, dir, logPath)
		if err != nil {
			failed = append(failed, be+": "+err.Error())
			continue
		}
		if len(reports) == 0 {
			reason := skipReasonFromLog(runLog)
			if reason == "" {
				failed = append(failed, be+": no reports and no skip reason; see "+logPath)
			} else {
				unavailable = append(unavailable, be+": "+reason)
			}
			continue
		}
		results[be] = reports
		for _, scene := range sortedKeys(reports) {
			rep := reports[scene]
			dst := filepath.Join(outRoot, scene, be)
			if err := os.MkdirAll(dst, 0o755); err != nil {
				t.Fatalf("mkdir %s: %v", dst, err)
			}
			if err := conformance.WriteReport(filepath.Join(dst, "report.json"), rep); err != nil {
				t.Fatalf("write report: %v", err)
			}
			copyFile(t, filepath.Join(dir, scene+"_frame.png"), filepath.Join(dst, "frame.png"))
			copyFile(t, logPath, filepath.Join(dst, "run.txt"))
			if rep.Status != "ok" {
				unavailable = append(unavailable, be+" "+scene+": "+rep.Reason)
				continue
			}
			if !rep.OK {
				failed = append(failed, be+" "+scene+": "+strings.Join(rep.Failed(), "; "))
			}
		}
	}

	writeSummaryMarkdown(t, outRoot, results)

	if len(failed) > 0 {
		t.Fatalf("conformance failures:\n  %s", strings.Join(failed, "\n  "))
	}
	if len(unavailable) > 0 {
		t.Skipf("backend unavailable (recorded in reports and summary.md):\n  %s", strings.Join(unavailable, "\n  "))
	}
}

// TestConformanceInner is the per-backend subprocess body. It never fails on
// assertion reds — those are data in report.json so the orchestrator can
// collect every scene before deciding. It fails only on harness errors
// (bad case data, draw/IO failures).
func TestConformanceInner(t *testing.T) {
	if os.Getenv("GPUI_CONFORMANCE_INNER") != "1" {
		t.Skip("inner harness; driven by TestConformanceDualBackend")
	}
	backend := os.Getenv("GPUI_CONFORMANCE_BACKEND")
	out := os.Getenv("GPUI_CONFORMANCE_OUT")
	if out == "" {
		t.Fatalf("GPUI_CONFORMANCE_OUT unset")
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", out, err)
	}

	cases, err := conformance.LoadCases(filepath.Join("testdata", "conformance"))
	if err != nil {
		t.Fatalf("load cases: %v", err)
	}

	if spec := os.Getenv("GPUI_CONFORMANCE_MUTATE"); spec != "" {
		cases, err = mutateCases(cases, spec)
		if err != nil {
			t.Fatalf("mutation %q: %v", spec, err)
		}
	}

	var setupErr error
	switch backend {
	case "native":
		setupErr = setupNativeDevice(t)
	case "go":
		setupErr = setupGLDevice(t)
	default:
		t.Fatalf("unknown GPUI_CONFORMANCE_BACKEND %q", backend)
	}
	if setupErr != nil {
		for _, c := range cases {
			rep := conformance.Unavailable(backend, c.Name, setupErr.Error())
			writeReportOrFatal(t, out, c.Name, rep)
		}
		t.Skipf("backend %s unavailable: %v", backend, setupErr)
	}

	for _, c := range cases {
		rep := runConformanceScene(t, backend, c, ".", out)
		writeReportOrFatal(t, out, c.Name, rep)
		t.Logf("%s/%s ok=%v %s", backend, c.Name, rep.OK, strings.Join(rep.Failed(), "; "))
	}
}

func runConformanceScene(t *testing.T, backend string, c *conformance.Case, baseDir, outDir string) *conformance.Report {
	t.Helper()
	repeats := c.Resource.Repeats
	if repeats < 2 {
		repeats = 2
	}

	counts := make([]int, 0, repeats)
	bytesLive := make([]uint64, 0, repeats)
	ctxAfter := make([]int, 0, repeats)

	var last image.Image
	var pathStats conformance.PathStats
	for i := 0; i < repeats; i++ {
		dc := render.NewContext(c.Width, c.Height)
		if err := conformance.Draw(dc, c, baseDir); err != nil {
			dc.Close()
			t.Fatalf("%s/%s: draw: %v", backend, c.Name, err)
		}
		last = dc.Image()
		st := dc.RenderPathStats()
		pathStats = conformance.PathStats{
			GPUOps:                st.GPUOps,
			CPUFallbackOps:        st.CPUFallbackOps,
			FrameFlushes:          st.FrameFlushes,
			LastCPUFallbackReason: st.LastCPUFallbackReason,
		}
		if err := dc.Close(); err != nil {
			t.Fatalf("%s/%s: close: %v", backend, c.Name, err)
		}
		counts = append(counts, hal.VramLiveCount())
		bytesLive = append(bytesLive, hal.VramLiveBytes())
		ctxAfter = append(ctxAfter, render.GPUContextCount())
	}

	bounds := c.Resource
	res := &conformance.ResourceResult{
		Repeats:             repeats,
		LiveCount:           counts,
		LiveBytes:           bytesLive,
		SteadyCountDelta:    counts[repeats-1] - counts[repeats-2],
		SteadyBytesDelta:    int64(bytesLive[repeats-1]) - int64(bytesLive[repeats-2]),
		MaxSteadyCountDelta: bounds.MaxSteadyCountDelta,
		MaxSteadyBytesDelta: bounds.MaxSteadyBytesDelta,
		GPUContextsAfter:    ctxAfter,
	}
	res.OK = res.SteadyCountDelta <= res.MaxSteadyCountDelta &&
		res.SteadyBytesDelta <= res.MaxSteadyBytesDelta &&
		allZero(ctxAfter)

	rep := conformance.Evaluate(backend, c, last, pathStats, res)
	rep.OK = rep.OK && res.OK

	if err := conformance.WriteFramePNG(filepath.Join(outDir, c.Name+"_frame.png"), last); err != nil {
		t.Fatalf("write frame: %v", err)
	}
	return rep
}

// setupNativeDevice binds a boxed native webgpu device to the accelerator,
// mirroring setupGLDevice so both backends render through the same
// per-context GPU render context. Without the boxed device the default
// accelerator only accelerates SDF shapes and silently routes brushes,
// glyph masks, and images through the CPU gg rasterizer.
func setupNativeDevice(t *testing.T) error {
	t.Helper()
	if os.Getenv("WGPU_NATIVE_PATH") == "" {
		return errors.New("WGPU_NATIVE_PATH unset: native wgpu library path required")
	}
	if render.Accelerator() == nil {
		return errors.New("GPU accelerator not registered")
	}
	inst, err := webgpu.CreateInstance(&hal.InstanceDescriptor{Backends: gputypes.BackendsPrimary})
	if err != nil {
		return fmt.Errorf("native CreateInstance: %w", err)
	}
	ad, err := inst.RequestAdapter(&hal.RequestAdapterOptions{
		PowerPreference: gputypes.PowerPreferenceHighPerformance,
	})
	if err != nil {
		inst.Release()
		return fmt.Errorf("native RequestAdapter: %w", err)
	}
	dev, err := ad.RequestDevice(render.DeviceDescriptor("p0.4-conformance"))
	if err != nil {
		ad.Release()
		inst.Release()
		return fmt.Errorf("native RequestDevice: %w", err)
	}
	if err := rendgpu.SetDeviceProvider(&webgpu.SimpleDeviceProvider{
		Dev: dev, Adpt: ad, Format: gputypes.TextureFormatBGRA8Unorm,
	}); err != nil {
		dev.Release()
		ad.Release()
		inst.Release()
		return fmt.Errorf("native SetDeviceProvider: %w", err)
	}
	t.Cleanup(func() {
		_ = rendgpu.ResetAccelerator()
		_ = dev.WaitIdle()
		dev.Release()
		ad.Release()
		inst.Release()
	})
	return nil
}

// setupGLDevice binds a boxed pure-Go GL device to the render accelerator.
// GL rejects surfaceless RequestDevice, so a minimal X11 window serves as the
// context provider; it is never presented to (pixels are read back offscreen).
func setupGLDevice(t *testing.T) error {
	t.Helper()
	runtime.LockOSThread()
	t.Cleanup(runtime.UnlockOSThread)

	if os.Getenv("DISPLAY") == "" {
		return errors.New("no DISPLAY: pure-Go GL needs an X11 context-provider window")
	}
	if render.Accelerator() == nil {
		return errors.New("GPU accelerator not registered")
	}
	be, ok := hal.GetBackend(gputypes.BackendGL)
	if !ok || be == nil {
		return errors.New("BackendGL not registered")
	}
	inst, err := be.CreateInstance(&hal.InstanceDescriptor{Backends: gputypes.BackendsPrimary})
	if err != nil {
		return fmt.Errorf("GL CreateInstance: %w", err)
	}
	t.Cleanup(inst.Release)

	xw := memOpenX11(t, 64, 64)
	t.Cleanup(xw.close)
	surf, err := inst.CreateSurface(hal.SurfaceTarget{
		Kind:          hal.SurfaceTargetXlibWindow,
		DisplayHandle: xw.display,
		WindowHandle:  xw.window,
	})
	if err != nil {
		return fmt.Errorf("GL CreateSurface(Xlib): %w", err)
	}
	t.Cleanup(surf.Destroy)

	ad, err := inst.RequestAdapter(&hal.RequestAdapterOptions{
		PowerPreference:   gputypes.PowerPreferenceHighPerformance,
		CompatibleSurface: surf,
	})
	if err != nil {
		return fmt.Errorf("GL RequestAdapter: %w", err)
	}
	t.Cleanup(ad.Release)

	dev, err := ad.RequestDevice(render.DeviceDescriptor("p0.4-conformance"))
	if err != nil {
		return fmt.Errorf("GL RequestDevice: %w", err)
	}
	t.Cleanup(dev.Release)

	if err := render.SetAcceleratorDeviceProvider(&p1ProbeProvider{dev: dev, ad: ad}); err != nil {
		return fmt.Errorf("SetAcceleratorDeviceProvider: %w", err)
	}
	t.Cleanup(render.AbandonAcceleratorDevice)
	return nil
}

func runBackendSubprocess(t *testing.T, backend, dir, logPath string) (map[string]*conformance.Report, string, error) {
	t.Helper()
	args := []string{"-test.run", "^TestConformanceInner$", "-test.v", "-test.timeout", "5m", "-test.count", "1"}
	cmd := exec.Command(os.Args[0], args...)
	cmd.Env = append(os.Environ(),
		"GPUI_CONFORMANCE_INNER=1",
		"GPUI_CONFORMANCE_BACKEND="+backend,
		"GPUI_BACKEND="+backend,
		"GPUI_CONFORMANCE_OUT="+dir,
	)
	out, err := cmd.CombinedOutput()
	runLog := string(out)
	if werr := os.WriteFile(logPath, out, 0o644); werr != nil {
		return nil, runLog, fmt.Errorf("write %s: %w", logPath, werr)
	}
	if err != nil {
		return nil, runLog, fmt.Errorf("inner subprocess: %v\n%s", err, tailLines(runLog, 20))
	}

	paths, _ := filepath.Glob(filepath.Join(dir, "*.json"))
	reports := make(map[string]*conformance.Report, len(paths))
	for _, p := range paths {
		data, rerr := os.ReadFile(p)
		if rerr != nil {
			return nil, runLog, fmt.Errorf("read %s: %w", p, rerr)
		}
		var rep conformance.Report
		if jerr := json.Unmarshal(data, &rep); jerr != nil {
			return nil, runLog, fmt.Errorf("parse %s: %w", p, jerr)
		}
		reports[rep.Scene] = &rep
	}
	return reports, runLog, nil
}

// runConformanceNegativeControl injects an expectation mutation and requires
// the named assertion to turn red.
func runConformanceNegativeControl(t *testing.T, spec string) {
	scene, sample, delta, err := parseMutation(0, spec)
	if err != nil {
		t.Fatalf("%v", err)
	}
	outRoot := os.Getenv("GPUI_CONFORMANCE_OUT")
	if outRoot == "" {
		outRoot = filepath.Join("tmp", "conformance")
	}
	dir := filepath.Join(outRoot, "negative", scene+"_"+sample)
	if err := os.RemoveAll(dir); err != nil {
		t.Fatalf("clean %s: %v", dir, err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}

	logPath := filepath.Join(dir, "run.txt")
	reports, runLog, err := runBackendSubprocess(t, "native", dir, logPath)
	if err != nil {
		t.Fatalf("negative control subprocess: %v", err)
	}
	rep := reports[scene]
	if rep == nil {
		t.Fatalf("negative control: no report for %s\n%s", scene, tailLines(runLog, 20))
	}
	if rep.OK {
		t.Fatalf("negative control: %s stayed GREEN after mutating expected sample %s by %d", scene, sample, delta)
	}
	red := false
	for _, s := range rep.Samples {
		if s.Name == sample && !s.OK {
			red = true
		}
	}
	if !red {
		t.Fatalf("negative control: expected mutation of sample %s to be red, failures: %v", sample, rep.Failed())
	}
	if err := conformance.WriteReport(filepath.Join(dir, "report.json"), rep); err != nil {
		t.Fatalf("write report: %v", err)
	}
	t.Logf("negative control red as expected: %s", strings.Join(rep.Failed(), "; "))
}

// mutateCases keeps only the named scene and shifts the named sample's
// expected red channel by delta (clamped to 0..255).
func mutateCases(cases []*conformance.Case, spec string) ([]*conformance.Case, error) {
	scene, sample, delta, err := parseMutation(0, spec)
	if err != nil {
		return nil, err
	}
	for _, c := range cases {
		if c.Name != scene {
			continue
		}
		for i := range c.Samples {
			if c.Samples[i].Name != sample {
				continue
			}
			v := int(c.Samples[i].RGBA[0]) + delta
			if v < 0 {
				v = 0
			}
			if v > 255 {
				v = 255
			}
			c.Samples[i].RGBA[0] = uint8(v)
			c.Samples[i].Tol = 0
			return []*conformance.Case{c}, nil
		}
		return nil, fmt.Errorf("scene %q has no sample %q", scene, sample)
	}
	return nil, fmt.Errorf("no scene named %q", scene)
}

func parseMutation(defaultDelta int, spec string) (scene, sample string, delta int, err error) {
	if defaultDelta == 0 {
		defaultDelta = 64
	}
	parts := strings.Split(spec, ":")
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return "", "", 0, fmt.Errorf("mutation %q must be scene:sample[:delta]", spec)
	}
	scene, sample, delta = parts[0], parts[1], defaultDelta
	if len(parts) >= 3 {
		n, perr := strconv.Atoi(parts[2])
		if perr != nil || n == 0 {
			return "", "", 0, fmt.Errorf("mutation delta %q is not a non-zero integer", parts[2])
		}
		delta = n
	}
	return scene, sample, delta, nil
}

func writeReportOrFatal(t *testing.T, dir, scene string, rep *conformance.Report) {
	t.Helper()
	if err := conformance.WriteReport(filepath.Join(dir, scene+".json"), rep); err != nil {
		t.Fatalf("write report %s: %v", scene, err)
	}
}

func writeSummaryMarkdown(t *testing.T, outRoot string, results map[string]map[string]*conformance.Report) {
	t.Helper()
	var scenes []string
	seen := map[string]bool{}
	for _, byScene := range results {
		for scene := range byScene {
			if !seen[scene] {
				seen[scene] = true
				scenes = append(scenes, scene)
			}
		}
	}
	sort.Strings(scenes)
	backends := []string{"native", "go"}

	var b strings.Builder
	b.WriteString("# P0.4 双后端一致性测试台：运行汇总\n\n")
	b.WriteString("> 由 `render/conformance_test.go` 自动生成；`report.json` 与 `frame.png` 位于 `<scene>/<backend>/`。\n\n")
	b.WriteString("回归命令（在仓库根目录执行；测试进程 cwd 是 render/，输出路径必须用绝对路径）：\n\n")
	b.WriteString("```bash\n")
	b.WriteString("WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so \\\n")
	b.WriteString("GPUI_CONFORMANCE_OUT=$PWD/docs/render_gpu_converge/baseline/conformance \\\n")
	b.WriteString("  go test ./render -run '^TestConformanceDualBackend$' -count=1 -v\n")
	b.WriteString("```\n\n")
	b.WriteString("负向对照（改错期望值必须转红）：\n\n")
	b.WriteString("```bash\n")
	b.WriteString("WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so \\\n")
	b.WriteString("GPUI_CONFORMANCE_OUT=$PWD/docs/render_gpu_converge/baseline/conformance \\\n")
	b.WriteString("GPUI_CONFORMANCE_MUTATE=solid_gradient:linear_left \\\n")
	b.WriteString("  go test ./render -run '^TestConformanceDualBackend$' -count=1 -v\n")
	b.WriteString("```\n\n")
	b.WriteString("| 场景 | " + strings.Join(backends, " | ") + " |\n")
	b.WriteString("|---|" + strings.Repeat("---|", len(backends)) + "\n")
	for _, scene := range scenes {
		cells := make([]string, 0, len(backends))
		for _, be := range backends {
			rep := results[be][scene]
			switch {
			case rep == nil:
				cells = append(cells, "未产出")
			case rep.Status != "ok":
				cells = append(cells, "不可用: "+rep.Reason)
			case rep.OK:
				cells = append(cells, "PASS")
			default:
				cells = append(cells, "FAIL: "+strings.Join(rep.Failed(), "; "))
			}
		}
		b.WriteString("| " + scene + " | " + strings.Join(cells, " | ") + " |\n")
	}
	appendKnownDivergences(&b, results)
	path := filepath.Join(outRoot, "summary.md")
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// appendKnownDivergences lists every per-backend relaxation that actually
// engaged, so a registered gap can never hide behind a green cell.
func appendKnownDivergences(b *strings.Builder, results map[string]map[string]*conformance.Report) {
	var rows []string
	for _, be := range []string{"native", "go"} {
		for _, scene := range sortedKeys(results[be]) {
			rep := results[be][scene]
			if rep.Status != "ok" {
				continue
			}
			fallbackRelaxed := rep.Path != nil && rep.Path.CPUFallbackOps > 0 && rep.Path.CPUFallbackOps <= rep.Path.MaxCPUFallbackOps
			countRelaxed := rep.Resource != nil && rep.Resource.SteadyCountDelta > 0 && rep.Resource.SteadyCountDelta <= rep.Resource.MaxSteadyCountDelta
			bytesRelaxed := rep.Resource != nil && rep.Resource.SteadyBytesDelta > 0 && rep.Resource.SteadyBytesDelta <= rep.Resource.MaxSteadyBytesDelta
			if !fallbackRelaxed && !countRelaxed && !bytesRelaxed {
				continue
			}
			var kinds []string
			reason := ""
			if fallbackRelaxed {
				kinds = append(kinds, fmt.Sprintf("CPU 回退 %d/%d ops", rep.Path.CPUFallbackOps, rep.Path.MaxCPUFallbackOps))
				reason = rep.Path.LastFallbackReason
			}
			if countRelaxed {
				kinds = append(kinds, fmt.Sprintf("账本 countΔ %d/%d", rep.Resource.SteadyCountDelta, rep.Resource.MaxSteadyCountDelta))
			}
			if bytesRelaxed {
				kinds = append(kinds, fmt.Sprintf("账本 bytesΔ %d/%d", rep.Resource.SteadyBytesDelta, rep.Resource.MaxSteadyBytesDelta))
			}
			rows = append(rows, fmt.Sprintf("| %s | %s | %s | %s |", scene, be, strings.Join(kinds, "；"), reason))
		}
	}
	b.WriteString("\n## 已知差异（用例数据显式放宽，非静默跳过）\n\n")
	if len(rows) == 0 {
		b.WriteString("无：全部后端落在满额断言内。\n")
		return
	}
	b.WriteString("| 场景 | 后端 | 放宽项（实测/上限） | 回退原因 |\n")
	b.WriteString("|---|---|---|---|\n")
	b.WriteString(strings.Join(rows, "\n"))
	b.WriteString("\n")
}

func skipReasonFromLog(log string) string {
	lines := strings.Split(log, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if strings.Contains(line, "--- SKIP:") {
			return line
		}
	}
	return ""
}

func tailLines(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}

func sortedKeys(m map[string]*conformance.Report) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func allZero(v []int) bool {
	for _, n := range v {
		if n != 0 {
			return false
		}
	}
	return true
}

func copyFile(t *testing.T, src, dst string) {
	t.Helper()
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatalf("copy %s -> %s: %v", src, dst, err)
	}
	if err := os.WriteFile(dst, data, 0o644); err != nil {
		t.Fatalf("copy %s -> %s: %v", src, dst, err)
	}
}
