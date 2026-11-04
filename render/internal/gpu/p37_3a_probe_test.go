//go:build !nogpu

package gpu

// P3.7 批 3a 前置探针 P3（design.md §10 拍板 P1 档：探针先行，结论落
// evidence/P3.7/batch3a/probe-p3-premul.txt）。诊断 harness，不是正式钉：
//
//   1. 组合可行性：按 D-3a-1 拍板 A（引擎模板合成，Godot #CODE : FRAGMENT 同构）
//      把用户片元组合进引擎模板，组合产物重跑 Parse→Lower→Validate + native
//      CreateShaderModule 真编译；
//   2. premul 双路口径：同一份 shader「quad 片元路（组合管线直出 +
//      BlendStatePremultiplied）」对「填充路（runUserShaderPass 读回，premul
//      字节原样交接）」逐像素对拍；
//   3. 恒等材质 ≡ 无材质：用户字面返回 textureSample 的材质腿 vs 纹理原值，
//      逐位对拍（A 档红利的可行性实测）。
//
// 3a 实现开工时：组合器搬进 render 包（加固引擎符号前缀与错误分派）、本文件
// 三条探针成熟为正式钉（判据不变，错误分派换具名哨兵）。

import (
	"crypto/sha256"
	"fmt"
	"image"
	"math"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/energye/gpui/gpu/hal"
	"github.com/energye/gpui/gpu/shader"
	"github.com/energye/gpui/gpu/shader/ir"
	"github.com/energye/gpui/gpu/types"
	"github.com/energye/gpui/render"
)

// ---- 探针用用户片元（uv-only / 全前缀 / 零参 / 采样纹理） ----

const probeWGSLUVOnly = `@fragment
fn fs_main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    return vec4<f32>(uv.x, uv.y, 0.25, 1.0);
}`

const probeWGSLFullPrefix = `@fragment
fn fs_main(@location(0) uv: vec2<f32>, @location(1) tint: vec4<f32>, @location(2) hom_w: f32) -> @location(0) vec4<f32> {
    return vec4<f32>(uv.x * tint.r, uv.y * tint.g, hom_w * 0.5, tint.a);
}`

const probeWGSLZeroParam = `@fragment
fn fs_main() -> @location(0) vec4<f32> {
    return vec4<f32>(0.25, 0.5, 0.75, 1.0);
}`

const probeWGSLGrad = `@fragment
fn fs_main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    let a = 0.3 + 0.6 * uv.x;
    return vec4<f32>(0.8 * a, 0.35 * a, 0.1 * a, a);
}`

const probeWGSLIdentity = `
@group(0) @binding(1) var probe_user_tex: texture_2d<f32>;
@group(0) @binding(2) var probe_user_samp: sampler;

@fragment
fn fs_main(@location(0) uv: vec2<f32>) -> @location(0) vec4<f32> {
    return textureSample(probe_user_tex, probe_user_samp, uv);
}`

// ---- 组合器原形（D-3a-1 拍板 A） ----

var (
	probeParamAttrRe  = regexp.MustCompile(`@[a-zA-Z_]+\([^)]*\)`)
	probeReturnAttrRe = regexp.MustCompile(`->\s*@[a-zA-Z_]+\([^)]*\)`)
)

// composeProbeMaterialFS 组合用户片元进引擎模板：引擎前奏（@group(1) 裁剪
// uniform + SDF 覆盖 + 顶点模块，quad 覆盖 rect）+ 用户源（@fragment 摘除、
// fs_main 全量改名 user_fs_main、参数 @location 属性摘除）+ 引擎输出段
// （按声明前缀传参调用户函数，之后乘 clip 覆盖；clip 缓冲清零 ⇒ ×1.0 逐位恒等）。
func composeProbeMaterialFS(t *testing.T, userWGSL string, rect [4]float64, canvas float64) string {
	t.Helper()

	// 1. 前置 Parse→Lower（读声明参数；源非法在此拒绝，同 NewShaderEffect 第一道闸）。
	ast, err := shader.Parse(userWGSL)
	if err != nil {
		t.Fatalf("probe compose: parse user: %v", err)
	}
	mod, err := shader.Lower(ast)
	if err != nil {
		t.Fatalf("probe compose: lower user: %v", err)
	}
	var args []ir.FunctionArgument
	for i := range mod.EntryPoints {
		if mod.EntryPoints[i].Stage == ir.StageFragment && mod.EntryPoints[i].Name == "fs_main" {
			args = mod.EntryPoints[i].Function.Arguments
			break
		}
	}
	if args == nil && !strings.Contains(userWGSL, "fn fs_main") {
		t.Fatalf("probe compose: no fragment fs_main")
	}

	// 2. 手术：改名 + 摘 @fragment + 参数/返回类型的入口属性摘除（WGSL 规则：
	//    @location 等 IO 属性只许出现在入口上；降为普通函数后必须全部摘除，
	//    否则 naga 拒绝——首轮探针实测：只摘参数不摘返回类型 ⇒ 模块无效）。
	src := strings.ReplaceAll(userWGSL, "fs_main", "user_fs_main")
	src = strings.ReplaceAll(src, "@fragment", "")
	if i := strings.Index(src, "fn user_fs_main("); i >= 0 {
		open := i + len("fn user_fs_main")
		// 配对扫描找签名闭括号（@location(0) 自带括号，首现 ')' 不是签名结尾——
		// 探针首轮实测：按首现 ')' 截断会把参数表切在属性内部，摘除落空）。
		depth, close := 0, -1
		for j := open; j < len(src) && close < 0; j++ {
			switch src[j] {
			case '(':
				depth++
			case ')':
				depth--
				if depth == 0 {
					close = j
				}
			}
		}
		if close < 0 {
			t.Fatalf("probe compose: unbalanced user_fs_main signature")
		}
		params := probeParamAttrRe.ReplaceAllString(src[open+1:close], "")
		params = strings.Join(strings.Fields(params), " ")
		params = strings.ReplaceAll(params, " ,", ",")
		src = src[:open+1] + params + src[close:]
	}
	src = probeReturnAttrRe.ReplaceAllString(src, "->")

	// 3. 引擎输出段按声明前缀传参（探针里 tint/hom_w 用字面默认值；3a 正式版
	//    由引擎顶点模块逐顶点供值，fill 路以默认值喂参——design §2.1）。
	var callArgs string
	switch len(args) {
	case 0:
		callArgs = "()"
	case 1:
		callArgs = "(in.uv)"
	case 2:
		callArgs = "(in.uv, vec4<f32>(1.0, 1.0, 1.0, 1.0))"
	case 3:
		callArgs = "(in.uv, vec4<f32>(1.0, 1.0, 1.0, 1.0), 1.0)"
	default:
		t.Fatalf("probe compose: %d args outside the declared prefix", len(args))
	}

	half := canvas / 2.0
	w := rect[2] - rect[0]
	h := rect[3] - rect[1]
	prelude := fmt.Sprintf(`
struct ProbeClipParams {
    clip_rect: vec4<f32>,
    clip_radius: f32,
    clip_enabled: f32,
    _pad: vec2<f32>,
}
@group(1) @binding(0) var<uniform> probe_clip: ProbeClipParams;

fn probe_clip_cov(frag_pos: vec2<f32>) -> f32 {
    let cx = (probe_clip.clip_rect.x + probe_clip.clip_rect.z) * 0.5;
    let cy = (probe_clip.clip_rect.y + probe_clip.clip_rect.w) * 0.5;
    let hw = (probe_clip.clip_rect.z - probe_clip.clip_rect.x) * 0.5;
    let hh = (probe_clip.clip_rect.w - probe_clip.clip_rect.y) * 0.5;
    let r = probe_clip.clip_radius;
    let dx = sqrt((frag_pos.x - cx) * (frag_pos.x - cx));
    let dy = sqrt((frag_pos.y - cy) * (frag_pos.y - cy));
    let qx = dx - hw + r;
    let qy = dy - hh + r;
    let mqx = (qx + sqrt(qx * qx)) * 0.5;
    let mqy = (qy + sqrt(qy * qy)) * 0.5;
    let outside = sqrt(mqx * mqx + mqy * mqy);
    let qdiff = qx - qy;
    let max_qxy = (qx + qy + sqrt(qdiff * qdiff)) * 0.5;
    let inside = (max_qxy - sqrt(max_qxy * max_qxy)) * 0.5;
    let d = outside + inside - r;
    let aa_hw = 0.75;
    let t_raw = d / (2.0 * aa_hw) + 0.5;
    let t_pos = (t_raw + sqrt(t_raw * t_raw)) * 0.5;
    let t_diff = t_pos - 1.0;
    let t = (t_pos + 1.0 - sqrt(t_diff * t_diff)) * 0.5;
    let sdf_cov = 1.0 - t * t * (3.0 - 2.0 * t);
    return probe_clip.clip_enabled * sdf_cov + (1.0 - probe_clip.clip_enabled);
}

struct ProbeVSOut {
    @builtin(position) pos: vec4<f32>,
    @location(0) uv: vec2<f32>,
}

@vertex
fn vs_main(@builtin(vertex_index) vi: u32) -> ProbeVSOut {
    var px = array<vec2<f32>, 6>(
        vec2<f32>(%.1f, %.1f), vec2<f32>(%.1f, %.1f), vec2<f32>(%.1f, %.1f),
        vec2<f32>(%.1f, %.1f), vec2<f32>(%.1f, %.1f), vec2<f32>(%.1f, %.1f));
    var o: ProbeVSOut;
    let p = px[vi];
    o.pos = vec4<f32>(p.x / %.1f - 1.0, 1.0 - p.y / %.1f, 0.0, 1.0);
    o.uv = vec2<f32>((p.x - %.1f) / %.1f, (p.y - %.1f) / %.1f);
    return o;
}

struct ProbeFSIn {
    @builtin(position) pos: vec4<f32>,
    @location(0) uv: vec2<f32>,
}
`, rect[0], rect[1], rect[2], rect[1], rect[2], rect[3],
		rect[0], rect[1], rect[2], rect[3], rect[0], rect[3],
		half, half, rect[0], w, rect[1], h)

	wrapper := fmt.Sprintf(`
@fragment
fn fs_main(in: ProbeFSIn) -> @location(0) vec4<f32> {
    let user = user_fs_main%s;
    return user * probe_clip_cov(in.pos.xy);
}
`, callArgs)

	composed := prelude + "\n" + src + "\n" + wrapper

	// 4. 组合产物重跑全链（手术 bug 在此当场拒绝，不进后端）。
	ast2, err := shader.Parse(composed)
	if err != nil {
		t.Fatalf("probe compose: parse composed: %v\ncomposed:\n%s", err, composed)
	}
	mod2, err := shader.Lower(ast2)
	if err != nil {
		t.Fatalf("probe compose: lower composed: %v", err)
	}
	if errs, err := shader.Validate(mod2); err != nil {
		t.Fatalf("probe compose: validate composed: %v", err)
	} else if len(errs) > 0 {
		t.Fatalf("probe compose: validate composed: %d error(s), first: %v", len(errs), errs[0])
	}
	if !strings.Contains(composed, "fn user_fs_main(") {
		t.Fatalf("probe compose: surgery did not rename the user entry")
	}
	if dump := os.Getenv("P37A_DUMP_COMPOSED"); dump != "" {
		_ = os.WriteFile(dump, []byte(composed), 0o644)
	}
	return composed
}

// probeRequireGPU 引导真设备（opt21 同款三件套跳过口径）。
func probeRequireGPU(t *testing.T) *GPUShared {
	t.Helper()
	if os.Getenv("WGPU_NATIVE_PATH") == "" {
		t.Skip("WGPU_NATIVE_PATH required")
	}
	shared := NewGPUShared()
	t.Cleanup(func() { shared.Close() })
	if err := shared.ensureGPU(); err != nil || !shared.gpuReady {
		t.Skipf("GPU not ready: %v", err)
	}
	return shared
}

// probeRound8 float→u8（GPU unorm 写入的最近邻近似；±1 容差在判据里声明）。
func probeRound8(v float64) uint8 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 255
	}
	return uint8(math.Round(v * 255))
}

// probeGradAnalytic 梯度 shader 的解析值（与 WGSL 同式同序）。
func probeGradAnalytic(u float64) (r, g, b, a float64) {
	a = 0.3 + 0.6*u
	return 0.8 * a, 0.35 * a, 0.1 * a, a
}

// probeGradOverWhite premul 源对白底的一次合成（dst=255 ⇒ 整数精确：out = src + 255 - a）。
func probeGradOverWhite(r, g, b, a float64) [4]uint8 {
	return [4]uint8{probeRound8(r + 1 - a), probeRound8(g + 1 - a), probeRound8(b + 1 - a), probeRound8(a + 1 - a)}
}

// TestP37AProbeComposeValidate 探针 1：三类声明前缀组合可行性（组合产物过
// Parse→Lower→Validate + native CreateShaderModule 真编译）。
func TestP37AProbeComposeValidate(t *testing.T) {
	shared := probeRequireGPU(t)
	for name, wgsl := range map[string]string{
		"uv_only":    probeWGSLUVOnly,
		"full_pre":   probeWGSLFullPrefix,
		"zero_param": probeWGSLZeroParam,
		"identity":   probeWGSLIdentity,
	} {
		t.Run(name, func(t *testing.T) {
			composed := composeProbeMaterialFS(t, wgsl, [4]float64{4, 4, 28, 28}, 32)
			mod, err := shared.device.CreateShaderModule(&hal.ShaderModuleDescriptor{Label: "p37a_probe_fs", WGSL: composed})
			if err != nil {
				t.Fatalf("native CreateShaderModule rejected composed source: %v", err)
			}
			mod.Destroy()
		})
	}
}

// TestP37AProbeQuadPremulPath 探针 2：quad 片元路（组合管线 + BlendStatePremultiplied
// 直出）vs 解析期望 vs 填充路复刻（runUserShaderPass → 去预乘 → 上传重预乘 → 同一合成）。
func TestP37AProbeQuadPremulPath(t *testing.T) {
	shared := probeRequireGPU(t)
	device, queue := shared.device, shared.queue
	rc := shared.NewRenderContext()

	const W, H = 32, 32
	rect := [4]float64{4, 4, 28, 28} // 与填充腿 DrawRectangle(4,4,24,24) 同一设备矩形

	composed := composeProbeMaterialFS(t, probeWGSLGrad, rect, W)
	fsMod, err := device.CreateShaderModule(&hal.ShaderModuleDescriptor{Label: "p37a_probe_grad", WGSL: composed})
	if err != nil {
		t.Fatalf("shader module: %v", err)
	}
	defer fsMod.Destroy()

	bgl1, err := device.CreateBindGroupLayout(&hal.BindGroupLayoutDescriptor{Label: "p37a_probe_bgl1",
		Entries: []types.BindGroupLayoutEntry{{
			Binding: 0, Visibility: types.ShaderStageFragment,
			Buffer: &types.BufferBindingLayout{Type: types.BufferBindingTypeUniform, MinBindingSize: 32},
		}}})
	if err != nil {
		t.Fatalf("bgl1: %v", err)
	}
	defer bgl1.Destroy()
	// 布局数组按位对 @group(N)：组合模块引擎段在 @group(1)，用户无绑定时
	// 槽 0 也要垫一个空布局（identity 案例用户声明占 @group(0)，无需垫）。
	bglEmpty, err := device.CreateBindGroupLayout(&hal.BindGroupLayoutDescriptor{Label: "p37a_probe_bgl_empty"})
	if err != nil {
		t.Fatalf("bgl empty: %v", err)
	}
	defer bglEmpty.Destroy()
	layout, err := device.CreatePipelineLayout(&hal.PipelineLayoutDescriptor{Label: "p37a_probe_layout",
		BindGroupLayouts: []hal.BindGroupLayout{bglEmpty, bgl1}})
	if err != nil {
		t.Fatalf("layout: %v", err)
	}
	defer layout.Destroy()
	bs := types.BlendStatePremultiplied()
	pipe, err := device.CreateRenderPipeline(&hal.RenderPipelineDescriptor{
		Label: "p37a_probe_material", Layout: layout,
		Vertex:      hal.VertexState{Module: fsMod, EntryPoint: "vs_main"},
		Primitive:   triangleListPrimitive(),
		Multisample: types.MultisampleState{Count: 1, Mask: 0xFFFFFFFF},
		Fragment: &hal.FragmentState{
			Module: fsMod, EntryPoint: "fs_main",
			Targets: []types.ColorTargetState{{Format: types.TextureFormatBGRA8Unorm, WriteMask: types.ColorWriteMaskAll, Blend: &bs}},
		},
	})
	if err != nil {
		t.Fatalf("pipeline: %v", err)
	}
	defer pipe.Destroy()

	clipBuf, err := device.CreateBuffer(&hal.BufferDescriptor{Label: "p37a_probe_clip", Size: 32,
		Usage: types.BufferUsageUniform | types.BufferUsageCopyDst})
	if err != nil {
		t.Fatalf("clip buf: %v", err)
	}
	defer clipBuf.Destroy()
	if err := queue.WriteBuffer(clipBuf, 0, make([]byte, 32)); err != nil {
		t.Fatalf("clip upload: %v", err)
	}
	bg1, err := device.CreateBindGroup(&hal.BindGroupDescriptor{Layout: bgl1,
		Entries: []hal.BindGroupEntry{{Binding: 0, Buffer: clipBuf, Offset: 0, Size: 32}}})
	if err != nil {
		t.Fatalf("bg1: %v", err)
	}
	defer bg1.Destroy()

	tex, view, err := dualTexCreateTexFmt(device, queue, "p37a_probe_target", W, H, nil,
		types.TextureUsageRenderAttachment|types.TextureUsageCopySrc, types.TextureFormatBGRA8Unorm)
	if err != nil {
		t.Fatalf("target: %v", err)
	}
	defer tex.Destroy()
	defer view.Destroy()

	enc, err := device.CreateCommandEncoder(&hal.CommandEncoderDescriptor{Label: "p37a_probe_enc"})
	if err != nil {
		t.Fatalf("encoder: %v", err)
	}
	pass, err := enc.BeginRenderPass(&hal.RenderPassDescriptor{Label: "p37a_probe_pass",
		ColorAttachments: []hal.RenderPassColorAttachment{{
			View: view, LoadOp: types.LoadOpClear, StoreOp: types.StoreOpStore,
			ClearValue: types.Color{R: 1, G: 1, B: 1, A: 1},
		}}})
	if err != nil {
		t.Fatalf("pass: %v", err)
	}
	pass.SetPipeline(pipe)
	pass.SetBindGroup(1, bg1, nil)
	pass.Draw(6, 1, 0, 0)
	pass.End()
	cmd, err := enc.Finish()
	if err != nil {
		t.Fatalf("finish: %v", err)
	}
	if _, err := queue.Submit(cmd); err != nil {
		t.Fatalf("submit: %v", err)
	}
	_ = device.WaitIdle()
	quadPix, err := readTextureViewRegionRGBA(device, queue, packView(view), image.Rect(0, 0, W, H), W, H)
	if err != nil {
		t.Fatalf("readback: %v", err)
	}

	// --- quad 腿 vs 解析期望（引擎后乘 ×1.0 逐位恒等 ⇒ 差应只来自 u8 写入舍入）。
	quadWorst, quadWorstAt, quadDiff := 0, "", 0
	for py := int(rect[1]); py < int(rect[3]); py++ {
		for px := int(rect[0]); px < int(rect[2]); px++ {
			u := (float64(px) + 0.5 - rect[0]) / (rect[2] - rect[0])
			r, g, b, a := probeGradAnalytic(u)
			want := probeGradOverWhite(r, g, b, a)
			o := (py*W + px) * 4
			for c := 0; c < 4; c++ {
				d := int(quadPix[o+c]) - int(want[c])
				if d < 0 {
					d = -d
				}
				if d > quadWorst {
					quadWorst, quadWorstAt = d, fmt.Sprintf("(%d,%d)c%d", px, py, c)
				}
				if d != 0 {
					quadDiff++
				}
			}
		}
	}
	if quadWorst > 1 {
		t.Fatalf("quad leg vs analytic: max delta %d at %s (diff channels %d) — 组合管线直出偏离 premul 契约", quadWorst, quadWorstAt, quadDiff)
	}

	// --- 填充路复刻：真实 pass-1 读回（premul 字节原样，与修复后的
	//     fillShaderAsImage 同一交接）→ 同一白底合成，与 quad 腿逐像素对拍。
	eff, err := render.NewShaderEffect(probeWGSLGrad)
	if err != nil {
		t.Fatalf("effect: %v", err)
	}
	defer eff.Close()
	bw, bh := 24, 24
	pm, err := rc.runUserShaderPass(device, queue, eff, bw, bh)
	if err != nil {
		t.Fatalf("fill leg pass1: %v", err)
	}

	fillWorst, fillWorstAt, fillDiff := 0, "", 0
	hist := map[int]int{}
	for cy := 0; cy < bh; cy++ {
		for cx := 0; cx < bw; cx++ {
			o := (cy*bw + cx) * 4
			pr, pg, pb, pa := int(pm[o]), int(pm[o+1]), int(pm[o+2]), int(pm[o+3])
			// premul 对白底：out = src + 255 - a（整数精确，dst 侧无舍入）
			want := [4]uint8{uint8(pr + 255 - pa), uint8(pg + 255 - pa), uint8(pb + 255 - pa), 255}
			px, py := rect[0]+float64(cx), rect[1]+float64(cy)
			qo := (int(py)*W + int(px)) * 4
			for c := 0; c < 4; c++ {
				d := int(quadPix[qo+c]) - int(want[c])
				if d < 0 {
					d = -d
				}
				if d > 0 {
					hist[d]++
					fillDiff++
					if d > fillWorst {
						fillWorst, fillWorstAt = d, fmt.Sprintf("rt(%d,%d)c%d", cx, cy, c)
					}
				}
			}
		}
	}
	t.Logf("probe premul: quad-vs-analytic max=%d diff=%d; fill-vs-quad max=%d at %s diff=%d hist=%v",
		quadWorst, quadDiff, fillWorst, fillWorstAt, fillDiff, hist)
	if fillWorst > 6 {
		t.Fatalf("fill leg vs quad leg: max delta %d at %s (hist %v) — 两路超出量化往返预期（RT 写入+去预乘+重预乘各 ≤1）",
			fillWorst, fillWorstAt, hist)
	}
}

// TestP37AProbeIdentityMaterial 探针 3：恒等材质 ≡ 无纹理加工 —— 用户字面
// textureSample 的材质腿，1:1 贴 16×16 不透明棋盘，逐位等于源纹素（A 档红利：
// 组合保持引擎乘链不变 ⇒ 可逐位钉）。
func TestP37AProbeIdentityMaterial(t *testing.T) {
	shared := probeRequireGPU(t)
	device, queue := shared.device, shared.queue

	const W, H = 32, 32
	const imgW, imgH = 16, 16
	rect := [4]float64{4, 4, 4 + imgW, 4 + imgH}

	checker := make([]byte, imgW*imgH*4)
	for ty := 0; ty < imgH; ty++ {
		for tx := 0; tx < imgW; tx++ {
			o := (ty*imgW + tx) * 4
			checker[o] = uint8(40 + ((tx*7 + ty*13) % 200))
			checker[o+1] = uint8(30 + ((tx*11 + ty*5) % 220))
			checker[o+2] = uint8(20 + ((tx*3 + ty*17) % 230))
			checker[o+3] = 255 // 不透明：premul == 原值
		}
	}
	digest := probeCheckerDigestHex(checker)

	composed := composeProbeMaterialFS(t, probeWGSLIdentity, rect, W)
	fsMod, err := device.CreateShaderModule(&hal.ShaderModuleDescriptor{Label: "p37a_probe_identity", WGSL: composed})
	if err != nil {
		t.Fatalf("shader module: %v", err)
	}
	defer fsMod.Destroy()

	bgl0, err := device.CreateBindGroupLayout(&hal.BindGroupLayoutDescriptor{Label: "p37a_probe_bgl0",
		Entries: []types.BindGroupLayoutEntry{
			{Binding: 1, Visibility: types.ShaderStageFragment, Texture: &types.TextureBindingLayout{SampleType: types.TextureSampleTypeFloat, ViewDimension: types.TextureViewDimension2D}},
			{Binding: 2, Visibility: types.ShaderStageFragment, Sampler: &types.SamplerBindingLayout{Type: types.SamplerBindingTypeFiltering}},
		}})
	if err != nil {
		t.Fatalf("bgl0: %v", err)
	}
	defer bgl0.Destroy()
	bgl1, err := device.CreateBindGroupLayout(&hal.BindGroupLayoutDescriptor{Label: "p37a_probe_bgl1",
		Entries: []types.BindGroupLayoutEntry{{
			Binding: 0, Visibility: types.ShaderStageFragment,
			Buffer: &types.BufferBindingLayout{Type: types.BufferBindingTypeUniform, MinBindingSize: 32},
		}}})
	if err != nil {
		t.Fatalf("bgl1: %v", err)
	}
	defer bgl1.Destroy()
	layout, err := device.CreatePipelineLayout(&hal.PipelineLayoutDescriptor{Label: "p37a_probe_layout2",
		BindGroupLayouts: []hal.BindGroupLayout{bgl0, bgl1}})
	if err != nil {
		t.Fatalf("layout: %v", err)
	}
	defer layout.Destroy()
	bs := types.BlendStatePremultiplied()
	pipe, err := device.CreateRenderPipeline(&hal.RenderPipelineDescriptor{
		Label: "p37a_probe_material_identity", Layout: layout,
		Vertex:      hal.VertexState{Module: fsMod, EntryPoint: "vs_main"},
		Primitive:   triangleListPrimitive(),
		Multisample: types.MultisampleState{Count: 1, Mask: 0xFFFFFFFF},
		Fragment: &hal.FragmentState{
			Module: fsMod, EntryPoint: "fs_main",
			Targets: []types.ColorTargetState{{Format: types.TextureFormatBGRA8Unorm, WriteMask: types.ColorWriteMaskAll, Blend: &bs}},
		},
	})
	if err != nil {
		t.Fatalf("pipeline: %v", err)
	}
	defer pipe.Destroy()

	tex, view, err := dualTexCreateTex(device, queue, "p37a_probe_checker", imgW, imgH, checker,
		types.TextureUsageTextureBinding|types.TextureUsageCopyDst)
	if err != nil {
		t.Fatalf("checker tex: %v", err)
	}
	defer tex.Destroy()
	defer view.Destroy()
	samp, err := device.CreateSampler(&hal.SamplerDescriptor{
		AddressModeU: types.AddressModeClampToEdge, AddressModeV: types.AddressModeClampToEdge,
		MagFilter: types.FilterModeLinear, MinFilter: types.FilterModeLinear,
	})
	if err != nil {
		t.Fatalf("sampler: %v", err)
	}
	defer samp.Destroy()
	bg0, err := device.CreateBindGroup(&hal.BindGroupDescriptor{Layout: bgl0,
		Entries: []hal.BindGroupEntry{
			{Binding: 1, TextureView: view},
			{Binding: 2, Sampler: samp},
		}})
	if err != nil {
		t.Fatalf("bg0: %v", err)
	}
	defer bg0.Destroy()

	clipBuf, err := device.CreateBuffer(&hal.BufferDescriptor{Label: "p37a_probe_clip2", Size: 32,
		Usage: types.BufferUsageUniform | types.BufferUsageCopyDst})
	if err != nil {
		t.Fatalf("clip buf: %v", err)
	}
	defer clipBuf.Destroy()
	if err := queue.WriteBuffer(clipBuf, 0, make([]byte, 32)); err != nil {
		t.Fatalf("clip upload: %v", err)
	}
	bg1, err := device.CreateBindGroup(&hal.BindGroupDescriptor{Layout: bgl1,
		Entries: []hal.BindGroupEntry{{Binding: 0, Buffer: clipBuf, Offset: 0, Size: 32}}})
	if err != nil {
		t.Fatalf("bg1: %v", err)
	}
	defer bg1.Destroy()

	target, targetView, err := dualTexCreateTexFmt(device, queue, "p37a_probe_target2", W, H, nil,
		types.TextureUsageRenderAttachment|types.TextureUsageCopySrc, types.TextureFormatBGRA8Unorm)
	if err != nil {
		t.Fatalf("target: %v", err)
	}
	defer target.Destroy()
	defer targetView.Destroy()

	enc, err := device.CreateCommandEncoder(&hal.CommandEncoderDescriptor{Label: "p37a_probe_enc2"})
	if err != nil {
		t.Fatalf("encoder: %v", err)
	}
	pass, err := enc.BeginRenderPass(&hal.RenderPassDescriptor{Label: "p37a_probe_pass2",
		ColorAttachments: []hal.RenderPassColorAttachment{{
			View: targetView, LoadOp: types.LoadOpClear, StoreOp: types.StoreOpStore,
			ClearValue: types.Color{R: 1, G: 1, B: 1, A: 1},
		}}})
	if err != nil {
		t.Fatalf("pass: %v", err)
	}
	pass.SetPipeline(pipe)
	pass.SetBindGroup(0, bg0, nil)
	pass.SetBindGroup(1, bg1, nil)
	pass.Draw(6, 1, 0, 0)
	pass.End()
	cmd, err := enc.Finish()
	if err != nil {
		t.Fatalf("finish: %v", err)
	}
	if _, err := queue.Submit(cmd); err != nil {
		t.Fatalf("submit: %v", err)
	}
	_ = device.WaitIdle()
	pix, err := readTextureViewRegionRGBA(device, queue, packView(targetView), image.Rect(0, 0, W, H), W, H)
	if err != nil {
		t.Fatalf("readback: %v", err)
	}

	diff, worst, at := 0, 0, ""
	for ty := 0; ty < imgH; ty++ {
		for tx := 0; tx < imgW; tx++ {
			px, py := int(rect[0])+tx, int(rect[1])+ty
			qo := (py*W + px) * 4
			co := (ty*imgW + tx) * 4
			for c := 0; c < 3; c++ {
				d := int(pix[qo+c]) - int(checker[co+c])
				if d < 0 {
					d = -d
				}
				if d > 0 {
					diff++
					if d > worst {
						worst, at = d, fmt.Sprintf("(%d,%d)c%d", px, py, c)
					}
				}
			}
			if pix[qo+3] != 255 {
				diff++
			}
		}
	}
	t.Logf("probe identity: checker digest=%s diff=%d worst=%d at %s", digest[:16], diff, worst, at)
	if diff != 0 {
		t.Fatalf("identity material vs source texels: %d differing channels, worst %d at %s — 组合腿没有逐位保持纹理（uv 约定/采样态/乘链问题）", diff, worst, at)
	}
}

// TestP37AProbeQueueDrain 隔离段：pass-1 premul 字节（与修复后 fillShaderAsImage
// 同一交接）直接走 QueueImageDraw + FlushOffscreen 到自管离屏纹理，绕开
// render.Context 的表面机制。画面对 ⇒ 坏在 Context 表面/读回链；画面坏 ⇒ 队列→图像腿自身有洞。
func TestP37AProbeQueueDrain(t *testing.T) {
	shared := probeRequireGPU(t)
	device, queue := shared.device, shared.queue
	rc := shared.NewRenderContext()

	eff, err := render.NewShaderEffect(probeWGSLGrad)
	if err != nil {
		t.Fatalf("effect: %v", err)
	}
	defer eff.Close()
	bw, bh := 24, 24
	srcRegion, err := rc.runUserShaderPass(device, queue, eff, bw, bh)
	if err != nil {
		t.Fatalf("pass1: %v", err)
	}

	const W, H = 32, 32
	rect := [4]float64{4, 4, 28, 28}
	tex, view, err := dualTexCreateTexFmt(device, queue, "p37a_probe_drain_target", W, H, nil,
		types.TextureUsageRenderAttachment|types.TextureUsageCopySrc, types.TextureFormatBGRA8Unorm)
	if err != nil {
		t.Fatalf("target: %v", err)
	}
	defer tex.Destroy()
	defer view.Destroy()
	target := render.GPURenderTarget{
		View: packView(view), ViewWidth: W, ViewHeight: H, Width: W, Height: H,
	}

	rc.QueueImageDraw(target, append([]byte(nil), srcRegion...), 0, nil, render.MipmapNone, bw, bh, bw*4,
		float32(rect[0]), float32(rect[1]), float32(rect[2]), float32(rect[1]),
		float32(rect[2]), float32(rect[3]), float32(rect[0]), float32(rect[3]),
		1.0, W, H, 0, 0, 1, 1, false, false)
	if err := rc.FlushOffscreen(target); err != nil {
		t.Fatalf("FlushOffscreen: %v", err)
	}
	_ = device.WaitIdle()
	pix, err := readTextureViewRegionRGBA(device, queue, packView(view), image.Rect(0, 0, W, H), W, H)
	if err != nil {
		t.Fatalf("readback: %v", err)
	}

	// 期望：premul 数据被 level0 原样上传采样（image_cache.go 语义），
	// FlushOffscreen 透明清底 + premul blend ⇒ out = texel 原样（黑底贡献 0）。
	worst, worstAt, diff := 0, "", 0
	row0 := ""
	for cx := 0; cx < bw; cx++ {
		o := cx * 4
		want := [4]uint8{srcRegion[o], srcRegion[o+1], srcRegion[o+2], srcRegion[o+3]}
		py, px := int(rect[1]), int(rect[0])+cx
		qo := (py*W + px) * 4
		if cx < 6 {
			row0 += fmt.Sprintf("got(%d,%d,%d,%d) want(%d,%d,%d,%d) ",
				pix[qo], pix[qo+1], pix[qo+2], pix[qo+3], want[0], want[1], want[2], want[3])
		}
		for c := 0; c < 4; c++ {
			d := int(pix[qo+c]) - int(want[c])
			if d < 0 {
				d = -d
			}
			if d > worst {
				worst, worstAt = d, fmt.Sprintf("(%d,%d)c%d", px, py, c)
			}
			if d != 0 {
				diff++
			}
		}
	}
	t.Logf("probe queue-drain: max=%d at %s diff=%d head: %s", worst, worstAt, diff, row0)
}

// TestP37AProbeFillShaderOnOwnTarget 隔离段 2：真实 fillShaderAsImage（生产
// 填充腿本体，修复后交 premul）画到自管离屏纹理，绕开 render.Context 表面机制。
// FlushOffscreen 透明清底 + premul blend ⇒ out = premul texel 原样
// （黑底贡献 0）；逐字节对账交队数据。
func TestP37AProbeFillShaderOnOwnTarget(t *testing.T) {
	shared := probeRequireGPU(t)
	device, queue := shared.device, shared.queue
	rc := shared.NewRenderContext()

	eff, err := render.NewShaderEffect(probeWGSLGrad)
	if err != nil {
		t.Fatalf("effect: %v", err)
	}
	defer eff.Close()

	const W, H = 32, 32
	rect := [4]float64{4, 4, 28, 28}
	tex, view, err := dualTexCreateTexFmt(device, queue, "p37a_probe_fill_target", W, H, nil,
		types.TextureUsageRenderAttachment|types.TextureUsageCopySrc, types.TextureFormatBGRA8Unorm)
	if err != nil {
		t.Fatalf("target: %v", err)
	}
	defer tex.Destroy()
	defer view.Destroy()
	target := render.GPURenderTarget{
		View: packView(view), ViewWidth: W, ViewHeight: H, Width: W, Height: H,
	}

	pb := render.BuildPath().Rect(rect[0], rect[1], rect[2]-rect[0], rect[3]-rect[1])
	path := pb.Path()
	paint := &render.Paint{Shader: eff}
	if err := rc.fillShaderAsImage(target, path, paint); err != nil {
		t.Fatalf("fillShaderAsImage: %v", err)
	}
	// 交队数据的对账副本（fillShaderAsImage 内部管线与探针 2 同一套转换链，
	// 修复后原样交 premul）。
	srcBytes, err := rc.runUserShaderPass(device, queue, eff, 24, 24)
	if err != nil {
		t.Fatalf("pass1 replica: %v", err)
	}
	if err := rc.FlushOffscreen(target); err != nil {
		t.Fatalf("FlushOffscreen: %v", err)
	}
	_ = device.WaitIdle()
	pix, err := readTextureViewRegionRGBA(device, queue, packView(view), image.Rect(0, 0, W, H), W, H)
	if err != nil {
		t.Fatalf("readback: %v", err)
	}

	head, diff := "", 0
	worst, worstAt := 0, ""
	for cx := 0; cx < 24; cx++ {
		o := cx * 4
		want := [4]uint8{srcBytes[o], srcBytes[o+1], srcBytes[o+2], srcBytes[o+3]}
		py, px := int(rect[1]), int(rect[0])+cx
		qo := (py*W + px) * 4
		if cx < 6 {
			head += fmt.Sprintf("(%d,%d,%d,%d)", pix[qo], pix[qo+1], pix[qo+2], pix[qo+3])
		}
		for c := 0; c < 4; c++ {
			d := int(pix[qo+c]) - int(want[c])
			if d < 0 {
				d = -d
			}
			if d > 0 {
				diff++
				if d > worst {
					worst, worstAt = d, fmt.Sprintf("(%d,%d)c%d", px, py, c)
				}
			}
		}
	}
	t.Logf("probe fill-on-own-target row0: %s — diff=%d worst=%d at %s", head, diff, worst, worstAt)
	if diff != 0 {
		t.Fatalf("fill leg on own target no longer passes premul texels through: %d differing channels, worst %d at %s", diff, worst, worstAt)
	}
}

// probeCheckerDigestHex 输出棋盘逐位摘要（与 render 包探针的 production
// DrawImage 对照腿共享同一字节面，跨包对拍用）。
func probeCheckerDigestHex(checker []byte) string {
	return fmt.Sprintf("%x", sha256.Sum256(checker))
}
