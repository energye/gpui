//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2024
//
//----------------------------------------

package render

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/energye/gpui/gpu/shader/ir"
)

// P3.7 批 3a（D-3a-1 拍板 A）：用户片元进管线走「引擎模板合成」——Godot
// canvas.glsl 的 `#CODE : FRAGMENT` 同构（用户代码贴进引擎的片元主管线，
// 用户代码之后引擎还有输出段）。组合在 NewShaderEffect 期完成，组合产物重跑
// Parse→Lower→Validate 全链，任何手术偏差当场具名拒绝、不进后端。
//
//   - quad 变体（材质腿）：引擎顶点模块（36B 顶点，位置按 hom_w 缩放出真
//     clip-space w）+ 用户函数（按声明前缀传参）+ 引擎输出段（乘 opacity ×
//     RRect SDF 覆盖；tint 按 T2「声明即接管」：用户声明了 tint 输入就不再
//     乘）。引擎数据 @group(1)（transform+opacity，逐绘制）、@group(2)
//     （ClipParams，与 textured_quad 管线的裁剪绑定组同布局同对象）。
//   - fill 变体（填充路 pass-1）：现有全屏三角顶点（只出 uv）+ 用户函数
//     （未声明的输入以默认值喂参：tint=白、hom_w=1）；无引擎输出段——填充
//     路的 coverage/裁剪仍由读回后的 mask 链承担。
//
// 组合保持引擎乘链与标准 FS 同序（texel * opacity * tint），恒等材质（用户
// 字面返回纹理采样）与无材质逐位相同（探针 P3 实证，证据
// evidence/P3.7/batch3a/probe-p3-premul.txt §三）。

const composedUserEntry = "gpui_user_fs_main"

var (
	shaderParamAttrRe  = regexp.MustCompile(`@[a-zA-Z_]+\([^)]*\)`)
	shaderReturnAttrRe = regexp.MustCompile(`->\s*@[a-zA-Z_]+\([^)]*\)`)
)

// fragmentParams validates fs_main's declared input prefix and returns its
// length (0..4). Positions and types are fixed by the contract; names are
// free. 0 = fs_main()（零参，只上填充路）; 1 = uv; 2 = uv,tint; 3 =
// uv,tint,hom_w; 4 = uv,tint,hom_w,texel（材质×透视/bicubic 专项 2026-11-07：
// 第 4 参=引擎按本笔采样档采好的纹素——对齐 Godot「TEXTURE 按 item 档位采样」，
// 采样质量归引擎，用户只管上色）。
func fragmentParams(mod *ir.Module) (int, error) {
	entry := -1
	for i := range mod.EntryPoints {
		if mod.EntryPoints[i].Stage == ir.StageFragment && mod.EntryPoints[i].Name == "fs_main" {
			entry = i
			break
		}
	}
	if entry < 0 {
		return 0, fmt.Errorf("shader: contract: missing @fragment entry point fs_main")
	}
	args := mod.EntryPoints[entry].Function.Arguments
	if len(args) > 4 {
		return 0, fmt.Errorf("%w: contract: fs_main takes %d parameters, the contract prefix is at most (uv, tint, hom_w, texel)", ErrShaderSlotConflict, len(args))
	}
	for i := range args {
		arg := &args[i]
		var loc ir.LocationBinding
		var hasLoc bool
		if arg.Binding != nil {
			loc, hasLoc = (*arg.Binding).(ir.LocationBinding)
		}
		if !hasLoc || loc.Location != uint32(i) {
			return 0, fmt.Errorf("%w: contract: fs_main parameter %d must carry @location(%d)", ErrShaderSlotConflict, i, i)
		}
		inner := typeInnerOf(mod, arg.Type)
		wantSize := ir.Vec2
		if i == 1 || i == 3 {
			wantSize = ir.Vec4
		}
		if i != 2 {
			vec, ok := inner.(ir.VectorType)
			if !ok || vec.Size != wantSize || vec.Scalar.Kind != ir.ScalarFloat {
				return 0, fmt.Errorf("%w: contract: fs_main parameter %d must be a f32 vector (position %d of the uv/tint/hom_w/texel prefix)", ErrShaderSlotConflict, i, i)
			}
			continue
		}
		// i == 2: hom_w: f32
		if _, ok := inner.(ir.VectorType); ok {
			return 0, fmt.Errorf("%w: contract: fs_main parameter %d (hom_w) must be a plain f32, got a vector", ErrShaderSlotConflict, i)
		}
		if sc, ok := inner.(ir.ScalarType); !ok || sc.Kind != ir.ScalarFloat {
			return 0, fmt.Errorf("%w: contract: fs_main parameter %d (hom_w) must be a plain f32", ErrShaderSlotConflict, i)
		}
	}
	return len(args), nil
}

// rewriteUserFragment de-entries the user fragment: renames fs_main to a
// callable function, strips the @fragment attribute and every entry-point IO
// attribute (parameter @location(...) and return @location(...)) — WGSL only
// allows IO attributes on entry points, so a non-entry function keeps none
// (probe P3 round 1: a surviving return attribute made naga reject the module).
func rewriteUserFragment(userWGSL string) (string, error) {
	src := strings.ReplaceAll(userWGSL, "fs_main", composedUserEntry)
	src = strings.ReplaceAll(src, "@fragment", "")
	marker := "fn " + composedUserEntry + "("
	i := strings.Index(src, marker)
	if i < 0 {
		return "", fmt.Errorf("shader: compose: renamed entry not found")
	}
	open := i + len(marker) - 1
	depth, close := 0, -1
	for j := open; j < len(src); j++ {
		switch src[j] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				close = j
			}
		}
		if close >= 0 {
			break
		}
	}
	if close < 0 {
		return "", fmt.Errorf("shader: compose: unbalanced %s signature", composedUserEntry)
	}
	params := shaderParamAttrRe.ReplaceAllString(src[open+1:close], "")
	params = strings.Join(strings.Fields(params), " ")
	params = strings.ReplaceAll(params, " ,", ",")
	src = src[:open+1] + params + src[close:]
	return shaderReturnAttrRe.ReplaceAllString(src, "->"), nil
}

// engineQuadSegment is the engine section of the composed quad (material)
// module. The user source comes first (already rewritten); every engine
// symbol carries the gpui_eng prefix so user declarations cannot collide.
const engineQuadSegment = `
struct GpuiEngDraw {
    transform: mat4x4<f32>,
    opacity_pad: vec4<f32>,
}
@group(1) @binding(0) var<uniform> gpui_eng_draw: GpuiEngDraw;

struct GpuiEngClipParams {
    clip_rect: vec4<f32>,
    clip_radius: f32,
    clip_enabled: f32,
    _pad: vec2<f32>,
}
@group(2) @binding(0) var<uniform> gpui_eng_clip: GpuiEngClipParams;

fn gpui_eng_clip_cov(frag_pos: vec2<f32>) -> f32 {
    let cx = (gpui_eng_clip.clip_rect.x + gpui_eng_clip.clip_rect.z) * 0.5;
    let cy = (gpui_eng_clip.clip_rect.y + gpui_eng_clip.clip_rect.w) * 0.5;
    let hw = (gpui_eng_clip.clip_rect.z - gpui_eng_clip.clip_rect.x) * 0.5;
    let hh = (gpui_eng_clip.clip_rect.w - gpui_eng_clip.clip_rect.y) * 0.5;
    let r = gpui_eng_clip.clip_radius;
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
    return gpui_eng_clip.clip_enabled * sdf_cov + (1.0 - gpui_eng_clip.clip_enabled);
}

struct GpuiEngVSIn {
    @location(0) position: vec2<f32>,
    @location(1) tex_coord: vec2<f32>,
    @location(2) tint: vec4<f32>,
    @location(3) hom_w: f32,
}
struct GpuiEngVSOut {
    @builtin(position) position: vec4<f32>,
    @location(0) tex_coord: vec2<f32>,
    @location(1) tint: vec4<f32>,
    @location(2) hom_w: f32,
}

@vertex
fn vs_main(in: GpuiEngVSIn) -> GpuiEngVSOut {
    let p = vec4<f32>(in.position, 0.0, 1.0);
    let t = gpui_eng_draw.transform;
    let pos = p.x * t[0] + p.y * t[1] + p.z * t[2] + p.w * t[3];
    // Emit true clip-space w (same form as textured_quad_persp.wgsl): the
    // rasterizer divides xy by it and interpolates varyings perspective-
    // correctly. Affine material draws feed hom_w = 1 (identity).
    let w = in.hom_w;
    var out: GpuiEngVSOut;
    out.position = vec4<f32>(pos.x * w, pos.y * w, pos.z * w, w);
    out.tex_coord = in.tex_coord;
    out.tint = in.tint;
    out.hom_w = in.hom_w;
    return out;
}

struct GpuiEngFSIn {
    @builtin(position) position: vec4<f32>,
    @location(0) tex_coord: vec2<f32>,
    @location(1) tint: vec4<f32>,
    @location(2) hom_w: f32,
}

@fragment
fn fs_main(in: GpuiEngFSIn) -> @location(0) vec4<f32> {
    let user = gpui_user_fs_main{{ARGS}};
    let opacity = gpui_eng_draw.opacity_pad.x * gpui_eng_clip_cov(in.position.xy);
    // T2 (batch 3a): a fragment that declares the tint input owns tint
    // application — the engine only multiplies opacity × clip coverage after
    // user code. Without the declaration the engine keeps the historic
    // texel * opacity * tint order (identity vs the standard FS bit-for-bit).
    return user * opacity{{TINT}};
}
`

// engineFillSegment wraps the user function for the fill pass-1 leg: the
// fullscreen-triangle vertex stage only emits uv, so undeclared inputs are
// fed literal defaults (tint = white, hom_w = 1). No engine output section —
// coverage/clip still ride the readback mask chain (design §2.1).
const engineFillSegment = `
struct GpuiEngFSIn {
    @builtin(position) position: vec4<f32>,
    @location(0) uv: vec2<f32>,
}

@fragment
fn fs_main(in: GpuiEngFSIn) -> @location(0) vec4<f32> {
    return gpui_user_fs_main{{ARGS}};
}
`

// engineTexelNames resolves the sprite-texture binding pair for the texel
// feed: if the user declared group(0) binding(1)=texture_2d<f32> and
// binding(2)=sampler, the engine section references THOSE variables (one
// declaration per binding — duplicates are invalid WGSL); otherwise the
// engine injects its own reserved-name declarations (declared=false) and the
// effect's binding face must count the pair (shader_effect.go).
func engineTexelNames(mod *ir.Module) (texName, sampName string, declared bool) {
	for i := range mod.GlobalVariables {
		gv := &mod.GlobalVariables[i]
		if gv.Binding == nil || gv.Binding.Group != 0 {
			continue
		}
		switch gv.Binding.Binding {
		case 1:
			texName = gv.Name
		case 2:
			sampName = gv.Name
		}
	}
	if texName != "" && sampName != "" {
		return texName, sampName, true
	}
	return "gpui_eng_tex", "gpui_eng_samp", false
}

// gpuiEngTexelSection renders the engine texel-feed section for the composed
// quad module. bicubic selects the embedded 16-tap Catmull-Rom kernel (same
// constants as textured_quad_bicubic.wgsl / CPU SampleBicubic); plain
// variants use a single textureSample through the bound sampler (nearest /
// linear state comes from the per-draw sampler binding). Params < 4 never
// reach this (no section → existing modules stay byte-identical).
func gpuiEngTexelSection(texName, sampName string, declaredByUser, bicubic bool) string {
	decls := ""
	if !declaredByUser {
		decls = "@group(0) @binding(1) var " + texName + ": texture_2d<f32>;\n" +
			"@group(0) @binding(2) var " + sampName + ": sampler;\n\n"
	}
	if !bicubic {
		return decls + "fn gpui_eng_texel(uv: vec2<f32>) -> vec4<f32> {\n" +
			"    return textureSample(" + texName + ", " + sampName + ", uv);\n" +
			"}\n"
	}
	// bicubic 变体：16-tap Catmull-Rom（与 textured_quad_bicubic.wgsl / CPU
	// SampleBicubic 同常数同形；采样用命令绑定的 nearest 档 sampler——核内
	// 自算权重，硬件不得再插值）。engine 符号全 gpui_eng 前缀防碰撞。
	return decls + `fn gpui_eng_cubic_weight(t_in: f32, B: f32, C: f32) -> f32 {
    let t = sqrt(t_in * t_in);
    if t < 1.0 {
        let a = (12.0 - 9.0 * B - 6.0 * C) / 6.0;
        let b = (-18.0 + 12.0 * B + 6.0 * C) / 6.0;
        let c = (6.0 - 2.0 * B) / 6.0;
        return a * t * t * t + b * t * t + c;
    }
    if t < 2.0 {
        let a = (-B - 6.0 * C) / 6.0;
        let b = (6.0 * B + 30.0 * C) / 6.0;
        let c = (-12.0 * B - 48.0 * C) / 6.0;
        let d = (8.0 * B + 24.0 * C) / 6.0;
        return a * t * t * t + b * t * t + c * t + d;
    }
    return 0.0;
}

fn gpui_eng_texel(uv: vec2<f32>) -> vec4<f32> {
    let B = 0.0;
    let C = 0.5;
    let dims = vec2<f32>(textureDimensions(` + texName + `));
    let texel_pos = uv * dims - vec2<f32>(0.5, 0.5);
    let base = floor(texel_pos);
    let frac = texel_pos - base;
    var acc = vec4<f32>(0.0, 0.0, 0.0, 0.0);
    for (var j = 0; j < 4; j = j + 1) {
        let wy = gpui_eng_cubic_weight(frac.y - f32(j - 1), B, C);
        let iy = base.y + f32(j - 1);
        for (var i = 0; i < 4; i = i + 1) {
            let wx = gpui_eng_cubic_weight(frac.x - f32(i - 1), B, C);
            let ix = base.x + f32(i - 1);
            let tuv = vec2<f32>(ix + 0.5, iy + 0.5) / dims;
            let w = wx * wy;
            let sample_color = textureSampleLevel(` + texName + `, ` + sampName + `, tuv, 0.0);
            acc = acc + sample_color * w;
        }
    }
    return acc;
}
`
}

// composeUserFragment builds the pipeline fragment module for one leg.
// params is the validated declared-prefix length of fs_main (0..4).
func composeUserFragment(userWGSL string, params int, quad bool, bicubic bool, texName, sampName string, texelDeclaredByUser bool) (string, string, error) {
	user, err := rewriteUserFragment(userWGSL)
	if err != nil {
		return "", "", err
	}
	args := make([]string, 0, params)
	if quad {
		if params >= 1 {
			args = append(args, "in.tex_coord")
		}
		if params >= 2 {
			args = append(args, "in.tint")
		}
		if params >= 3 {
			args = append(args, "in.hom_w")
		}
		if params >= 4 {
			args = append(args, "gpui_eng_texel(in.tex_coord)")
		}
	} else {
		if params >= 1 {
			args = append(args, "in.uv")
		}
		if params >= 2 {
			args = append(args, "vec4<f32>(1.0, 1.0, 1.0, 1.0)")
		}
		if params >= 3 {
			args = append(args, "1.0")
		}
		if params >= 4 {
			// 填充路（pass-1 全屏三角）无纹理绑定可采：纹素默认零值（语义如实
			// 写进契约注释——texel 声明是材质腿意图，填充路吃到零）。
			args = append(args, "vec4<f32>(0.0, 0.0, 0.0, 0.0)")
		}
	}
	call := composedUserEntry + "(" + strings.Join(args, ", ") + ")"
	segment := engineFillSegment
	if quad {
		segment = engineQuadSegment
		// 材质×bicubic 纹素契约（2026-11-07）：params>=4 时引擎段前挂纹素馈送
		// （绑定声明按需注入 + gpui_eng_texel：plain 单采样 / bicubic 内嵌
		// 16-tap 核）。params<4 不拼——存量模块字节不变。
		if params >= 4 {
			segment = gpuiEngTexelSection(texName, sampName, texelDeclaredByUser, bicubic) + segment
		}
	}
	seg := strings.Replace(segment, "{{ARGS}}", call[len(composedUserEntry):], 1)
	seg = strings.Replace(seg, "{{TINT}}", tintTail(params), 1)
	return user + "\n" + seg, call, nil
}

func tintTail(params int) string {
	if params >= 2 {
		return ""
	}
	return " * in.tint"
}

// composeAndValidate runs the full front end over a composed module — any
// surgery drift or user/engine collision is rejected here, eagerly, before
// any backend sees the source.
func composeAndValidate(userWGSL string, params int, quad, bicubic bool, texName, sampName string, texelDeclaredByUser bool) (string, error) {
	composed, _, err := composeUserFragment(userWGSL, params, quad, bicubic, texName, sampName, texelDeclaredByUser)
	if err != nil {
		return "", err
	}
	if _, err := validateUserShader(composed); err != nil {
		name := "fill"
		if quad {
			name = "quad"
		}
		return "", fmt.Errorf("shader: compose %s: %w", name, err)
	}
	return composed, nil
}
