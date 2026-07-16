//go:build linux && !nogpu

package main

import (
	"fmt"
	"os"
	"strings"
)

// ProbeClass:
//
//	gate   — must PASS under default fps/cpu_fb gates (regression wall)
//	stress — expected to stress engine; FAIL is diagnostic signal, not scoreboard green
//	trap   — deliberately exercises a known-bad path to catch regressions (e.g. 1fps blend)
type ProbeClass string

const (
	classGate   ProbeClass = "gate"
	classStress ProbeClass = "stress"
	classTrap   ProbeClass = "trap"
)

// probeDef is one isolation-axis case for the diagnostic matrix.
type probeDef struct {
	ID     string
	NameCN string
	Class  ProbeClass
	// BaseTier seeds particle counts / region if ParticleN==0.
	BaseTier string
	// When true, loadConfig uses this feature set instead of tierDefaults features.
	OverrideFeatures                                     bool
	Solid, Blend, Glow, Mesh, Atlas, Text, Layer, Trails bool
	// Dig axes: Skia-facing correctness/perf traps.
	Clip, Grad, Filter, Dash, EvenOdd, Xform, MeshWave, ImagePx, TextBi, BlendSep bool
	ParticleN                                                                     int
	Region                                                                        float64
	BlendCircles                                                                  int
	// PerCircleBlend forces the ~1fps dual-tex path (regression trap).
	PerCircleBlend bool
	// ResizeOscillate recreates swapchain/context with oscillating size.
	ResizeOscillate bool
	// PathSubmitHeavy: many path Fill circles, skip dense base mesh batch.
	PathSubmitHeavy bool
	// MultiLayer: stack N translucent layers (0=default single if Layer).
	MultiLayer int
	// AltClear: alternate clear color each frame (clear/flicker path).
	AltClear bool
	// GrowN: grow particle count over time (realloc / mem pressure).
	GrowN bool
	// MaxCPUPct: if >0, FAIL when process CPU avg exceeds (diagnostic budget).
	MaxCPUPct float64
	// MaxJitter: if >0, FAIL when steady fps span exceeds (stability dig).
	MaxJitter float64
	// MemSoak seconds override (matrix uses GPUI_ANIM_SECONDS if set).
	MemSoakSec int
	// Gate relaxations
	AllowLowFPS bool
	// Min particle density — never gut content below this for "green".
	MinN   int
	Expect string
	// Hint for auto-bisect env flips on FAIL.
	BisectHint string
}

// isolationProbes is the full diagnostic matrix (axes + combos beyond L0–L4).
func isolationProbes() []probeDef {
	return []probeDef{
		// ---- single-feature axes ----
		{
			ID: "P_SOLID", NameCN: "实心粒子轴", Class: classGate, BaseTier: "L0",
			OverrideFeatures: true, Solid: true,
			ParticleN: 800, Region: 0.65, MinN: 500,
			Expect:     "仅实心圆+mesh密度，稳定≈60fps，cpu_fb=0",
			BisectHint: "GPUI_PARTICLE_N / 检查 DrawCircle Fill GPU 路径",
		},
		{
			ID: "P_MESH", NameCN: "网格批轴", Class: classGate, BaseTier: "L0",
			OverrideFeatures: true, Mesh: true, Solid: true,
			ParticleN: 1500, Region: 0.65, MinN: 1000,
			Expect:     "DrawMesh 大批量三角粒子，应保持高帧",
			BisectHint: "GPUI_ENABLE_MESH=0 对比",
		},
		{
			ID: "P_BLEND_LAYER", NameCN: "层批高级混合", Class: classGate, BaseTier: "L1",
			OverrideFeatures: true, Solid: true, Blend: true,
			ParticleN: 900, Region: 0.65, BlendCircles: 96, MinN: 800,
			Expect:     "PushLayer Screen/Multiply 批处理高级混合（修复后路径）",
			BisectHint: "GPUI_ENABLE_BLEND=0 或 GPUI_BLEND_CIRCLES=16",
		},
		{
			ID: "P_BLEND_PER_CIRCLE", NameCN: "逐圆混合陷阱", Class: classTrap, BaseTier: "L1",
			OverrideFeatures: true, Solid: true, Blend: true, PerCircleBlend: true,
			ParticleN: 400, Region: 0.65, BlendCircles: 48, MinN: 200,
			AllowLowFPS: true,
			Expect:      "故意逐圆 SetBlendMode(Screen/Multiply)：引擎若回退到 Poll dual-tex 会掉到~1fps",
			BisectHint:  "对比 P_BLEND_LAYER；查 fillAdvancedBlend / dual_tex Poll",
		},
		{
			ID: "P_GLOW", NameCN: "光晕RT轴", Class: classStress, BaseTier: "L2",
			OverrideFeatures: true, Solid: true, Glow: true,
			ParticleN: 1000, Region: 0.65, MinN: 800,
			Expect:     "有界 glow RT + blur export 隔帧；查滤镜读回成本",
			BisectHint: "GPUI_ENABLE_GLOW=0",
		},
		{
			ID: "P_ATLAS", NameCN: "图集精灵轴", Class: classGate, BaseTier: "L0",
			OverrideFeatures: true, Solid: true, Atlas: true,
			ParticleN: 800, Region: 0.65, MinN: 500,
			Expect:     "DrawAtlas 精灵叠在粒子上",
			BisectHint: "GPUI_ENABLE_ATLAS=0",
		},
		{
			ID: "P_TEXT", NameCN: "中文文本轴", Class: classGate, BaseTier: "L0",
			OverrideFeatures: true, Solid: true, Text: true,
			ParticleN: 600, Region: 0.65, MinN: 400,
			Expect:     "舞台旁中文说明 + HUD 中英文",
			BisectHint: "GPUI_ENABLE_TEXT=0；字体路径",
		},
		{
			ID: "P_LAYER", NameCN: "半透明层轴", Class: classGate, BaseTier: "L0",
			OverrideFeatures: true, Solid: true, Layer: true,
			ParticleN: 700, Region: 0.65, MinN: 500,
			Expect:     "PushLayer Normal 半透明涂层 + 粒子",
			BisectHint: "GPUI_ENABLE_LAYER=0；offscreen RT 池",
		},
		{
			ID: "P_TRAILS", NameCN: "拖尾描边轴", Class: classGate, BaseTier: "L0",
			OverrideFeatures: true, Solid: true, Trails: true,
			ParticleN: 800, Region: 0.65, MinN: 500,
			Expect:     "有界拖尾 stroke（前 N 粒子）",
			BisectHint: "GPUI_ENABLE_TRAILS=0",
		},
		{
			ID: "P_ALPHA_MESH", NameCN: "半透明网格密度", Class: classGate, BaseTier: "L1",
			OverrideFeatures: true, Mesh: true, Blend: true, Solid: true,
			ParticleN: 1200, Region: 0.68, BlendCircles: 64, MinN: 1000,
			Expect:     "alpha mesh 密度叠压 + 层批混合，不砍 N",
			BisectHint: "GPUI_ENABLE_MESH=0 / GPUI_ENABLE_BLEND=0",
		},

		// ---- interaction / problem-finding probes ----
		{
			ID: "P_BLEND_GLOW", NameCN: "混合×光晕组合", Class: classStress, BaseTier: "L2",
			OverrideFeatures: true, Solid: true, Blend: true, Glow: true,
			ParticleN: 1200, Region: 0.65, BlendCircles: 80, MinN: 1000,
			Expect:     "L2 分解：blend+glow 无 text/atlas；定位组合掉帧（单独轴可过时）",
			BisectHint: "对比 P_BLEND_LAYER vs P_GLOW；逐关 ENABLE_GLOW/BLEND",
		},
		{
			ID: "P_LAYER_BLEND", NameCN: "半透明层×混合", Class: classStress, BaseTier: "L1",
			OverrideFeatures: true, Solid: true, Layer: true, Blend: true,
			ParticleN: 1000, Region: 0.65, BlendCircles: 80, MinN: 800,
			Expect:     "Normal 层 + Screen/Multiply 层批；挖嵌套 advanced composite",
			BisectHint: "GPUI_ENABLE_LAYER=0 / GPUI_ENABLE_BLEND=0",
		},
		{
			ID: "P_MULTI_LAYER", NameCN: "多层嵌套", Class: classStress, BaseTier: "L0",
			OverrideFeatures: true, Solid: true, Layer: true, MultiLayer: 3,
			ParticleN: 900, Region: 0.65, MinN: 700,
			Expect:     "3 层半透明嵌套 + 粒子；挖 layer RT 池/泄漏/闪烁",
			BisectHint: "GPUI_ENABLE_LAYER=0；RSS；present 闪烁",
		},
		{
			ID: "P_SUBMIT_PATH", NameCN: "路径提交风暴", Class: classStress, BaseTier: "L0",
			OverrideFeatures: true, Solid: true, PathSubmitHeavy: true,
			ParticleN: 600, Region: 0.65, MinN: 500,
			Expect:     "大量 path DrawCircle+Fill（无 base mesh 批）；挖提交/编码瓶颈",
			BisectHint: "对比 P_MESH；GPU 提交批合并",
		},
		{
			ID: "P_HIGH_N", NameCN: "高密度实心", Class: classStress, BaseTier: "L0",
			OverrideFeatures: true, Solid: true, Mesh: true,
			ParticleN: 5000, Region: 0.70, MinN: 4000,
			Expect:     "N=5000 实心+mesh：若过而 L2 不过，瓶颈在高级路径非粒子本身",
			BisectHint: "对比 P_SOLID/P_BLEND_GLOW",
		},
		{
			ID: "P_FULL_STAGE", NameCN: "全窗舞台", Class: classStress, BaseTier: "L1",
			OverrideFeatures: true, Solid: true, Blend: true, Mesh: true,
			ParticleN: 1500, Region: 1.0, BlendCircles: 96, MinN: 1200,
			Expect:     "region=100% 全窗粒子+混合；挖全屏 present 成本",
			BisectHint: "GPUI_REGION=0.65 对比",
		},
		{
			ID: "P_CLEAR_ALT", NameCN: "交替清屏", Class: classGate, BaseTier: "L0",
			OverrideFeatures: true, Solid: true, AltClear: true,
			ParticleN: 600, Region: 0.65, MinN: 500,
			Expect:     "每帧交替背景色+粒子：验证 clear/全屏重绘无卡死、无 cpu_fb",
			BisectHint: "PresentFrameFull clear 路径",
		},
		{
			ID: "P_ATLAS_TEXT", NameCN: "图集×文本", Class: classGate, BaseTier: "L0",
			OverrideFeatures: true, Solid: true, Atlas: true, Text: true,
			ParticleN: 700, Region: 0.65, MinN: 500,
			Expect:     "atlas+中文无高级混合：UI 标注路径",
			BisectHint: "逐关 ATLAS/TEXT",
		},
		{
			ID: "P_GROW_N", NameCN: "粒子增长", Class: classStress, BaseTier: "L1",
			OverrideFeatures: true, Solid: true, Blend: true, Mesh: true, GrowN: true,
			ParticleN: 800, Region: 0.65, BlendCircles: 64, MinN: 800,
			AllowLowFPS: true,
			Expect:      "运行中 N 阶梯增长到 ~2500：挖 realloc / 缓存失效 / RSS 爬升",
			BisectHint:  "RSS steady；sim.resize；mesh 缓冲复用",
		},
		{
			ID: "P_BLEND_CPU", NameCN: "混合CPU预算", Class: classStress, BaseTier: "L1",
			OverrideFeatures: true, Solid: true, Blend: true,
			ParticleN: 900, Region: 0.65, BlendCircles: 96, MinN: 800,
			MaxCPUPct:  80, // process % of one core-equivalent; multi-core can exceed 100
			Expect:     "与 P_BLEND_LAYER 同负载；CPU 均值超预算则 FAIL（挖主线程过重）",
			BisectHint: "层批是否仍走重 CPU；对比 P_SOLID cpu",
		},
		{
			ID: "P_RESIZE", NameCN: "尺寸振荡重建", Class: classStress, BaseTier: "L1",
			OverrideFeatures: true, Solid: true, Blend: true, Mesh: true,
			ParticleN: 800, Region: 0.65, BlendCircles: 48, MinN: 600,
			ResizeOscillate: true, AllowLowFPS: true,
			Expect:     "每 ~45 帧振荡表面尺寸；稳态 present 错误/不可恢复才硬 FAIL",
			BisectHint: "GPUI_ENABLE_RESIZE=0；swapchain Resize/BeginFrame recover",
		},
		{
			ID: "P_DARK_STAGE", NameCN: "暗场最小绘制", Class: classGate, BaseTier: "L0",
			OverrideFeatures: true, Solid: true,
			ParticleN: 120, Region: 0.55, MinN: 80,
			Expect:     "少粒子暗底+角标：验证空场仍有 GPU 路径与非空内容",
			BisectHint: "gpu_ops / probe 角标",
		},
		{
			ID: "P_MEM_SOAK", NameCN: "短内存浸泡", Class: classStress, BaseTier: "L3",
			OverrideFeatures: true,
			Solid:            true, Blend: true, Glow: true, Mesh: true, Atlas: true, Text: true, Layer: true,
			ParticleN: 1600, Region: 0.68, BlendCircles: 80, MinN: 1200,
			MemSoakSec: 20, AllowLowFPS: true,
			Expect:     "L3 特征短浸泡：RSS 稳态斜率不得暴涨",
			BisectHint: "RSS steady delta；layer RT 池；glow export",
		},

		// ---- pixel / empty-content evidence ----
		{
			ID: "P_PIXEL", NameCN: "像素指纹", Class: classGate, BaseTier: "L0",
			OverrideFeatures: true, Solid: true,
			ParticleN: 400, Region: 0.6, MinN: 300,
			Expect:     "小 RT Export 纯色块 RGB 采样必须正确；抓空栅格/错误格式",
			BisectHint: "ExportImageBuf / pixmap 路径；pixel_fail",
		},
		{
			ID: "P_STAGE_SIG", NameCN: "舞台标记签名", Class: classGate, BaseTier: "L0",
			OverrideFeatures: true, Solid: true, Text: true,
			ParticleN: 300, Region: 0.6, MinN: 200,
			Expect:     "角标绿/红与背景可区分（离线 RT 签名）；抓内容全灰空场",
			BisectHint: "stage_sig_fail / 绘制 API",
		},
		{
			ID: "P_EMPTY_TRAP", NameCN: "空内容陷阱", Class: classTrap, BaseTier: "L0",
			OverrideFeatures: true, Solid: true,
			ParticleN: 80, Region: 0.5, MinN: 80,
			AllowLowFPS: true,
			Expect:      "最少绘制仍须 markers>=2 + pixel/stage 签名；若装空绿则 FAIL",
			BisectHint:  "content_fail / pixel_fail",
		},
		{
			ID: "P_FLICKER", NameCN: "交替清屏闪烁", Class: classGate, BaseTier: "L0",
			OverrideFeatures: true, Solid: true, AltClear: true, Text: true,
			ParticleN: 500, Region: 0.65, MinN: 400,
			Expect:     "交替清屏+粒子：间歇 stage 签名不得 dropout；抓闪屏/丢内容",
			BisectHint: "intermittent_content / PresentFrameFull clear",
		},
		{
			ID: "P_FLICKER_BLEND", NameCN: "混合路径闪烁", Class: classStress, BaseTier: "L1",
			OverrideFeatures: true, Solid: true, Blend: true, AltClear: true,
			ParticleN: 800, Region: 0.65, BlendCircles: 64, MinN: 600,
			Expect:     "交替清屏+层批混合：hitch 与 intermittent_content 双抓",
			BisectHint: "GPUI_ENABLE_BLEND=0；intermittent_content",
		},
		{
			ID: "P_MEM_LONG", NameCN: "加长内存浸泡", Class: classStress, BaseTier: "L3",
			OverrideFeatures: true,
			Solid:            true, Blend: true, Glow: true, Mesh: true, Atlas: true, Layer: true,
			ParticleN: 1400, Region: 0.68, BlendCircles: 64, MinN: 1200,
			MemSoakSec: 25, AllowLowFPS: true, GrowN: true,
			Expect:     "25s L3+grow：RSS 稳态斜率硬顶；抓泄漏",
			BisectHint: "rss / layer RT 池 / glow export",
		},

		// ---- dig axes: Skia-facing paths that surface real bugs ----
		{
			ID: "P_CLIP_NEST", NameCN: "嵌套裁剪", Class: classGate, BaseTier: "L0",
			OverrideFeatures: true, Solid: true, Clip: true, Text: true,
			ParticleN: 600, Region: 0.65, MinN: 400,
			Expect:     "嵌套 ClipRect/圆角窗 + 运动粒子；不得裁剪错位或空内容",
			BisectHint: "GPUI_ENABLE_CLIP=0；ClipRect/ClipPath 栈",
		},
		{
			ID: "P_GRAD_RT", NameCN: "渐变瓦片", Class: classGate, BaseTier: "L0",
			OverrideFeatures: true, Solid: true, Grad: true, Text: true,
			ParticleN: 500, Region: 0.65, MinN: 400,
			Expect:     "有界 RT 线性/径向/扫描渐变 + Export 贴回；抓 ColorAt CPU 与空渐变",
			BisectHint: "GPUI_ENABLE_GRAD=0；SetFillBrush gradient",
		},
		{
			ID: "P_FILTER_TILE", NameCN: "滤镜瓦片", Class: classStress, BaseTier: "L0",
			OverrideFeatures: true, Solid: true, Filter: true, Text: true,
			ParticleN: 500, Region: 0.65, MinN: 400,
			Expect:     "blur/drop-shadow 小 RT 瓦片；抓滤镜读回、闪屏、掉帧",
			BisectHint: "GPUI_ENABLE_FILTER=0；ApplyBlur/ApplyDropShadow",
		},
		{
			ID: "P_BLEND_SEP", NameCN: "可分离混合板", Class: classStress, BaseTier: "L0",
			OverrideFeatures: true, Solid: true, BlendSep: true, Text: true,
			ParticleN: 500, Region: 0.65, MinN: 400,
			Expect:     "Multiply/Screen/Overlay 有界 RT 混合板；抓可分离混合闪/慢",
			BisectHint: "GPUI_ENABLE_BLEND_SEP=0；C07 路径",
		},
		{
			ID: "P_PATH_XFORM", NameCN: "路径×变换", Class: classStress, BaseTier: "L0",
			OverrideFeatures: true, Solid: true, PathSubmitHeavy: true, Xform: true, Text: true,
			ParticleN: 700, Region: 0.65, MinN: 500,
			Expect:     "大量 path fill + 变换栈旋转缩放；抓 submit 卡顿与 xform 错位",
			BisectHint: "GPUI_ENABLE_PATH_SUBMIT=0 / GPUI_ENABLE_XFORM=0",
		},
		{
			ID: "P_EVENODD", NameCN: "EvenOdd填充", Class: classGate, BaseTier: "L0",
			OverrideFeatures: true, Solid: true, EvenOdd: true, Text: true,
			ParticleN: 400, Region: 0.6, MinN: 300,
			Expect:     "EvenOdd 空心环 vs NonZero 实心对照；抓填充规则错误",
			BisectHint: "GPUI_ENABLE_EVENODD=0；FillRuleEvenOdd",
		},
		{
			ID: "P_DASH", NameCN: "虚线描边", Class: classGate, BaseTier: "L0",
			OverrideFeatures: true, Solid: true, Dash: true, Text: true,
			ParticleN: 400, Region: 0.6, MinN: 300,
			Expect:     "动画 dash 偏移 stroke；抓虚线/hairline 路径",
			BisectHint: "GPUI_ENABLE_DASH=0；SetDash",
		},
		{
			ID: "P_MESH_WAVE", NameCN: "波浪网格", Class: classStress, BaseTier: "L0",
			OverrideFeatures: true, Solid: true, MeshWave: true, Text: true,
			ParticleN: 300, Region: 0.72, MinN: 200,
			Expect:     "密网格正弦波浪；抓 tessellation 锯齿/错位（视觉+帧率）",
			BisectHint: "GPUI_ENABLE_MESH_WAVE=0；DrawMesh 密度",
		},
		{
			ID: "P_TEXT_BI", NameCN: "中英双文", Class: classGate, BaseTier: "L0",
			OverrideFeatures: true, Solid: true, Text: true, TextBi: true,
			ParticleN: 400, Region: 0.6, MinN: 300,
			Expect:     "中文+Latin 同屏；离线字形采样须非空（抓乱码/缺英）",
			BisectHint: "字体路径 latin/cjk；DrawString",
		},
		{
			ID: "P_IMAGE_PX", NameCN: "写像素贴图", Class: classGate, BaseTier: "L0",
			OverrideFeatures: true, Solid: true, ImagePx: true, Text: true,
			ParticleN: 400, Region: 0.6, MinN: 300,
			Expect:     "ImageBuf 写像素 + DrawImage；抓贴图上传/采样",
			BisectHint: "GPUI_ENABLE_IMAGE_PX=0；SetRGBA/DrawImage",
		},
		{
			ID: "P_FPS_JIT", NameCN: "帧稳定抖动", Class: classStress, BaseTier: "L0",
			OverrideFeatures: true, Solid: true, Mesh: true,
			ParticleN: 1200, Region: 0.65, MinN: 1000,
			MaxJitter:  18,
			Expect:     "稳态 fps 抖动 span 应 <18；抓 hitch/调度不稳",
			BisectHint: "present pacing / sleep budget / submit spikes",
		},
		{
			ID: "P_FILTER_FLICKER", NameCN: "滤镜闪烁", Class: classStress, BaseTier: "L0",
			OverrideFeatures: true, Solid: true, Filter: true, AltClear: true, Text: true,
			ParticleN: 500, Region: 0.65, MinN: 400,
			Expect:     "滤镜瓦片+交替清屏；抓 intermittent_content/闪屏",
			BisectHint: "intermittent_content；ApplyBlur 隔帧",
		},
		{
			ID: "P_COMBO_UI", NameCN: "UI能力合成", Class: classStress, BaseTier: "L1",
			OverrideFeatures: true,
			Solid:            true, Blend: true, Layer: true, Text: true, TextBi: true,
			Clip: true, Grad: true, Filter: true, Dash: true, ImagePx: true,
			ParticleN: 900, Region: 0.68, BlendCircles: 48, MinN: 700,
			Expect:     "clip×grad×filter×text×layer×blend 合成墙；对标 C20 动态密度版",
			BisectHint: "逐关 CLIP/GRAD/FILTER/BLEND/LAYER",
		},
		{
			ID: "P_CPU_MESH", NameCN: "网格CPU预算", Class: classStress, BaseTier: "L0",
			OverrideFeatures: true, Solid: true, Mesh: true, MeshWave: true,
			ParticleN: 1500, Region: 0.7, MinN: 1200, MaxCPUPct: 80,
			Expect:     "密网格+波浪下进程 CPU 预算；抓主线程过重",
			BisectHint: "mesh build on CPU；batch path",
		},
		{
			ID: "P_XFORM_STACK", NameCN: "变换栈风暴", Class: classStress, BaseTier: "L0",
			OverrideFeatures: true, Solid: true, Xform: true, Mesh: true, Text: true,
			ParticleN: 800, Region: 0.65, MinN: 600,
			Expect:     "每帧多层 Save/Translate/Rotate/Scale；抓矩阵栈与状态泄漏",
			BisectHint: "GPUI_ENABLE_XFORM=0",
		},

		// ---- tier aliases ----
		{
			ID: "P_L0", NameCN: "档位L0", Class: classGate, BaseTier: "L0",
			Expect: "档位回归 L0", BisectHint: "见 L0",
		},
		{
			ID: "P_L1", NameCN: "档位L1", Class: classGate, BaseTier: "L1",
			Expect: "档位回归 L1 层批混合", BisectHint: "GPUI_ENABLE_BLEND=0",
		},
		{
			ID: "P_L2", NameCN: "档位L2", Class: classStress, BaseTier: "L2",
			Expect: "档位 L2 glow 压力（对照 P_BLEND_GLOW）", BisectHint: "GPUI_ENABLE_GLOW=0；P_BLEND_GLOW",
		},
		{
			ID: "P_L3", NameCN: "档位L3", Class: classStress, BaseTier: "L3",
			Expect: "档位 L3 UI 叠压压力", BisectHint: "逐开关关闭 TEXT/ATLAS/LAYER/GLOW",
		},
		{
			ID: "P_L4", NameCN: "档位L4", Class: classStress, BaseTier: "L4",
			Expect: "厨房水槽全开压力", BisectHint: "分轴 P_* 定位",
		},
	}
}

func probeByID(id string) (probeDef, bool) {
	id = strings.ToUpper(strings.TrimSpace(id))
	for _, p := range isolationProbes() {
		if p.ID == id {
			return p, true
		}
	}
	for _, p := range isolationProbes() {
		if strings.TrimPrefix(p.ID, "P_") == id {
			return p, true
		}
	}
	return probeDef{}, false
}

func applyProbe(cfg featureConfig, p probeDef) featureConfig {
	if p.BaseTier != "" {
		base := tierDefaults(p.BaseTier)
		cfg = base
	}
	cfg.Tier = p.ID
	if p.NameCN != "" {
		cfg.NameCN = p.NameCN
	}
	cfg.ProbeID = p.ID
	cfg.ProbeClass = string(p.Class)
	cfg.PerCircleBlend = p.PerCircleBlend
	cfg.ResizeOscillate = p.ResizeOscillate
	cfg.PathSubmitHeavy = p.PathSubmitHeavy
	cfg.MultiLayer = p.MultiLayer
	cfg.AltClear = p.AltClear
	cfg.GrowN = p.GrowN
	cfg.MaxCPUPct = p.MaxCPUPct
	cfg.MaxJitter = p.MaxJitter
	cfg.MinParticleN = p.MinN
	if p.Expect != "" {
		cfg.Expect = p.Expect
	}
	cfg.BisectHint = p.BisectHint
	if p.OverrideFeatures {
		cfg.Solid = p.Solid
		cfg.Blend = p.Blend
		cfg.Glow = p.Glow
		cfg.Mesh = p.Mesh
		cfg.Atlas = p.Atlas
		cfg.Text = p.Text
		cfg.Layer = p.Layer
		cfg.Trails = p.Trails
		cfg.Clip = p.Clip
		cfg.Grad = p.Grad
		cfg.Filter = p.Filter
		cfg.Dash = p.Dash
		cfg.EvenOdd = p.EvenOdd
		cfg.Xform = p.Xform
		cfg.MeshWave = p.MeshWave
		cfg.ImagePx = p.ImagePx
		cfg.TextBi = p.TextBi
		cfg.BlendSep = p.BlendSep
	}
	if p.ParticleN > 0 {
		cfg.ParticleN = p.ParticleN
	}
	if p.Region > 0 {
		cfg.Region = p.Region
	}
	if p.BlendCircles > 0 {
		cfg.BlendCircles = p.BlendCircles
	}
	if p.AllowLowFPS {
		cfg.AllowLowFPS = true
	}
	if p.MemSoakSec > 0 {
		cfg.MemSoakSec = p.MemSoakSec
	}
	if cfg.MinParticleN > 0 && cfg.ParticleN < cfg.MinParticleN {
		cfg.ParticleN = cfg.MinParticleN
	}
	return cfg
}

// listProbeIDs returns IDs for matrix scripts.
func listProbeIDs(filter string) []string {
	filter = strings.ToLower(strings.TrimSpace(filter))
	out := make([]string, 0, 32)
	for _, p := range isolationProbes() {
		switch filter {
		case "", "all":
			out = append(out, p.ID)
		case "gate":
			if p.Class == classGate {
				out = append(out, p.ID)
			}
		case "stress":
			if p.Class == classStress {
				out = append(out, p.ID)
			}
		case "trap":
			if p.Class == classTrap {
				out = append(out, p.ID)
			}
		case "axes", "axis":
			// isolation single-feature axes (no L* aliases, no combo names with x)
			if strings.HasPrefix(p.ID, "P_L") {
				continue
			}
			switch p.ID {
			case "P_BLEND_GLOW", "P_LAYER_BLEND", "P_MULTI_LAYER", "P_SUBMIT_PATH",
				"P_HIGH_N", "P_FULL_STAGE", "P_GROW_N", "P_BLEND_CPU", "P_ATLAS_TEXT",
				"P_ALPHA_MESH", "P_RESIZE", "P_MEM_SOAK",
				"P_COMBO_UI", "P_FILTER_FLICKER", "P_PATH_XFORM", "P_CPU_MESH",
				"P_FPS_JIT", "P_XFORM_STACK":
				// keep as extended axes / combos under "all"; exclude from pure axes
				continue
			}
			out = append(out, p.ID)
		case "combo", "combos":
			switch p.ID {
			case "P_BLEND_GLOW", "P_LAYER_BLEND", "P_MULTI_LAYER", "P_ALPHA_MESH",
				"P_ATLAS_TEXT", "P_FULL_STAGE", "P_BLEND_CPU", "P_L2", "P_L3", "P_L4",
				"P_SUBMIT_PATH", "P_HIGH_N", "P_GROW_N", "P_FLICKER_BLEND",
				"P_COMBO_UI", "P_FILTER_FLICKER", "P_PATH_XFORM", "P_CPU_MESH":
				out = append(out, p.ID)
			}
		case "dig", "quality":
			// Skia-facing dig wall: correctness + stability traps beyond particle axes.
			switch p.ID {
			case "P_CLIP_NEST", "P_GRAD_RT", "P_FILTER_TILE", "P_BLEND_SEP", "P_PATH_XFORM",
				"P_EVENODD", "P_DASH", "P_MESH_WAVE", "P_TEXT_BI", "P_IMAGE_PX",
				"P_FPS_JIT", "P_FILTER_FLICKER", "P_COMBO_UI", "P_CPU_MESH", "P_XFORM_STACK",
				"P_PIXEL", "P_STAGE_SIG", "P_EMPTY_TRAP", "P_FLICKER":
				out = append(out, p.ID)
			}
		case "core":
			// fast daily wall: gates + trap + key combos
			if p.Class == classGate || p.Class == classTrap {
				out = append(out, p.ID)
			} else if p.ID == "P_BLEND_GLOW" || p.ID == "P_RESIZE" || p.ID == "P_SUBMIT_PATH" || p.ID == "P_MULTI_LAYER" || p.ID == "P_BLEND_CPU" {
				out = append(out, p.ID)
			}
		case "evidence":
			switch p.ID {
			case "P_PIXEL", "P_STAGE_SIG", "P_EMPTY_TRAP", "P_FLICKER", "P_DARK_STAGE", "P_CLEAR_ALT":
				out = append(out, p.ID)
			}
		case "mem":
			switch p.ID {
			case "P_MEM_SOAK", "P_MEM_LONG", "P_GROW_N", "P_RESIZE", "P_MULTI_LAYER":
				out = append(out, p.ID)
			}
		default:
			out = append(out, p.ID)
		}
	}
	return out
}

func printProbeCatalog() {
	fmt.Println("# particle_kitchen_sink isolation probes")
	fmt.Println("| id | class | n | features | expect |")
	fmt.Println("|----|-------|---|----------|--------|")
	for _, p := range isolationProbes() {
		feats := []string{}
		if p.OverrideFeatures {
			if p.Solid {
				feats = append(feats, "solid")
			}
			if p.Blend {
				feats = append(feats, "blend")
			}
			if p.PerCircleBlend {
				feats = append(feats, "per_circle")
			}
			if p.Glow {
				feats = append(feats, "glow")
			}
			if p.Mesh {
				feats = append(feats, "mesh")
			}
			if p.Atlas {
				feats = append(feats, "atlas")
			}
			if p.Text {
				feats = append(feats, "text")
			}
			if p.Layer {
				feats = append(feats, "layer")
			}
			if p.Trails {
				feats = append(feats, "trails")
			}
			if p.ResizeOscillate {
				feats = append(feats, "resize")
			}
			if p.PathSubmitHeavy {
				feats = append(feats, "path_submit")
			}
			if p.MultiLayer > 0 {
				feats = append(feats, fmt.Sprintf("layers=%d", p.MultiLayer))
			}
			if p.AltClear {
				feats = append(feats, "alt_clear")
			}
			if p.GrowN {
				feats = append(feats, "grow_n")
			}
			if p.Clip {
				feats = append(feats, "clip")
			}
			if p.Grad {
				feats = append(feats, "grad")
			}
			if p.Filter {
				feats = append(feats, "filter")
			}
			if p.Dash {
				feats = append(feats, "dash")
			}
			if p.EvenOdd {
				feats = append(feats, "evenodd")
			}
			if p.Xform {
				feats = append(feats, "xform")
			}
			if p.MeshWave {
				feats = append(feats, "mesh_wave")
			}
			if p.ImagePx {
				feats = append(feats, "image_px")
			}
			if p.TextBi {
				feats = append(feats, "text_bi")
			}
			if p.BlendSep {
				feats = append(feats, "blend_sep")
			}
			if p.MaxCPUPct > 0 {
				feats = append(feats, fmt.Sprintf("cpu<%.0f", p.MaxCPUPct))
			}
			if p.MaxJitter > 0 {
				feats = append(feats, fmt.Sprintf("jit<%.0f", p.MaxJitter))
			}
		} else {
			feats = append(feats, "tier:"+p.BaseTier)
		}
		n := p.ParticleN
		if n == 0 {
			n = tierDefaults(p.BaseTier).ParticleN
		}
		fmt.Printf("| %s | %s | %d | %s | %s |\n", p.ID, p.Class, n, strings.Join(feats, ","), p.Expect)
	}
	_ = os.Getenv("GPUI_PROBE_CATALOG_EXIT")
}
