"""T1.1 机械拆分：render/context.go 按 12 组分文件（签名不动）。
映射表见 GROUPS；未命中映射的函数留在 context.go 并打印 UNMAPPED。
新文件暂带全量 import，后续由 import 修剪循环收敛。
用法：python3 tools/cleanup-audit/t11_split_context.py
"""
import re, os

ROOT = "/home/yanghy/app/projects/gogpu/gpui"
SRC = os.path.join(ROOT, "render/context.go")

GROUPS = {
    "context_construct.go": ["NewContext", "NewContextForPixmap", "NewContextForImage",
        "NewContextWithScale", "Close", "DropGPURenderContext"],
    "context_style.go": ["SetPipelineMode", "PipelineMode", "SetRasterizerMode",
        "RasterizerMode", "SetAntiAlias", "AntiAlias", "SetEffectSurface",
        "SetLCDLayout", "SetColor", "SetRGB", "SetRGBA", "SetHexColor",
        "SetFillBrush", "SetStrokeBrush", "FillBrush", "StrokeBrush",
        "SetLineWidth", "SetLineCap", "SetLineJoin", "SetFillRule",
        "SetMiterLimit", "SetStroke", "GetStroke", "SetDash", "SetDashOffset",
        "IsDashed", "currentColor"],
    "context_transform.go": ["Identity", "Translate", "Scale", "Rotate",
        "RotateAbout", "Shear", "Transform", "SetTransform", "GetTransform",
        "TransformPoint", "InvertY", "totalMatrix", "DeviceScale", "SetDeviceScale"],
    "context_path.go": ["MoveTo", "LineTo", "QuadraticTo", "CubicTo",
        "ClosePath", "ClearPath", "Clear", "ClearWithColor", "ClearDash",
        "SetPath", "AppendPath", "DrawPath", "FillPath", "StrokePath",
        "NewSubPath", "DrawPoint", "DrawLine", "DrawRectangle",
        "DrawRoundedRectangle", "DrawRoundedRectangleXY", "DrawCircle",
        "DrawEllipse", "DrawArc", "DrawEllipticalArc", "arcSegment",
        "ellipseArcSegment", "deviceSpacePath", "GetCurrentPoint", "FillRectCPU"],
    "context_draw.go": ["Fill", "Stroke", "FillPreserve", "StrokePreserve",
        "doFill", "doStroke", "SetPixel", "WritePixels", "tryGPUWritePixels",
        "Image", "applyMaskToPaint"],
    "context_clip.go": ["resetGPUClipForPass", "applyClipToPaint",
        "isClipActive", "setGPUClipRect", "setGPUClipPath"],
    "context_damage.go": ["FrameDamage", "FrameDamageUnion", "ResetFrameDamage",
        "SetDamageTracking", "TrackDamageRect", "trackDamageDevicePoints",
        "trackDamage", "FlushGPUWithViewDamageRects"],
    "context_gpu.go": ["recordGPUOp", "recordFrameFlush",
        "recordCPUFallbackReason", "BeginGPUFrame", "MemDigCmdBufs", "FlushGPU",
        "flushGPUWithViewCore", "flushWithTiming", "FlushGPUWithView",
        "FlushGPUWithViewDamage", "GPURenderContext", "ensureGPUCtx", "gpuCtxOps",
        "gpuRenderTarget", "warnGPUFallback", "flushGPUAccelerator", "tryGPUFill",
        "tryGPUStroke", "tryGPUOpRC", "tryGPUOp", "tryGPUFillWithMode",
        "tryGPUStrokeWithMode"],
    "context_layer.go": ["Push", "Pop", "BeginOffscreenPass",
        "offscreenPassSuspended", "SetSharedEncoder", "CreateSharedEncoder",
        "SubmitSharedEncoder", "acquireEffectPublishView"],
    "context_output.go": ["Width", "Height", "PixelWidth", "PixelHeight",
        "SavePNG", "EncodePNG", "EncodeJPEG", "Resize", "ResizeTarget"],
    "context_text.go": ["SetTextMode", "TextMode", "sdfAccelForShape",
        "takeBrushBootstrapIfAny", "setupGPUMask"],
    "context_diag.go": ["RenderPathStats", "ResetRenderPathStats", "LogLine",
        "gpuPathAvailable", "PixmapForTest", "RecordCPUFallbackForTest",
        "LastCPUFallbackReason", "setForceSDF"],
}
NAME2FILE = {}
for f, names in GROUPS.items():
    for n in names:
        assert n not in NAME2FILE, "duplicate map " + n
        NAME2FILE[n] = f

lines = open(SRC, encoding="utf-8").read().splitlines(keepends=True)
pat = re.compile(r'^func\s+(?:\(\s*\w+\s+\*?(\w+)\)\s+)?([A-Za-z_]\w*)')
idx = [i for i, ln in enumerate(lines) if pat.match(ln)]
print("FUNCS=" + str(len(idx)))

# import 块与文件头
imp_start = next(i for i, ln in enumerate(lines) if ln.startswith("import ("))
depth = 0
imp_end = imp_start
for i in range(imp_start, len(lines)):
    depth += lines[i].count("(") - lines[i].count(")")
    if lines[i].strip() == ")" or (depth == 0 and i > imp_start):
        imp_end = i
        break
header = lines[:9]
import_block = lines[imp_start:imp_end + 1]

chunks = []  # (name, start, end)
for k, i in enumerate(idx):
    end = idx[k + 1] if k + 1 < len(idx) else len(lines)
    m = pat.match(lines[i])
    chunks.append((m.group(2), i, end))

def doc_start(i):
    j = i - 1
    while j >= 0 and lines[j].startswith("//"):
        j -= 1
    return j + 1

buckets = {f: [] for f in GROUPS}
kept = []  # context.go 保留区间
unmapped = []
for name, i, end in chunks:
    f = NAME2FILE.get(name)
    if f is None:
        unmapped.append((i + 1, name))
        continue
    buckets[f].append((name, doc_start(i), end))

# 从后往前切，避免行号漂移：先算 context.go 保留行
cut = []
for f, items in buckets.items():
    for _, s, e in items:
        cut.append((s, e))
cut.sort()
keep_lines = []
prev = 0
for s, e in cut:
    keep_lines.extend(lines[prev:s])
    prev = e
keep_lines.extend(lines[prev:])
open(SRC, "w", encoding="utf-8").write("".join(keep_lines))

titles = {
    "context_construct.go": "构造与生命周期",
    "context_style.go": "样式状态存取",
    "context_transform.go": "变换",
    "context_path.go": "路径与形状",
    "context_draw.go": "绘制",
    "context_clip.go": "裁剪",
    "context_damage.go": "损伤跟踪",
    "context_gpu.go": "GPU 提交与帧",
    "context_layer.go": "层与离屏编码器",
    "context_output.go": "尺寸与输出",
    "context_text.go": "文本相关",
    "context_diag.go": "诊断与测试钩子",
}
def imported_paths(block_lines):
    return re.findall(r'"([^"]+)"', "".join(block_lines))

ctx_import_paths = imported_paths(import_block)

import subprocess
def tracked(rel):
    r = subprocess.run(["git", "ls-files", rel], cwd=ROOT,
                       capture_output=True, text=True)
    return bool(r.stdout.strip())

for f, items in buckets.items():
    body = []
    for name, s, e in sorted(items, key=lambda x: x[1]):
        body.extend(lines[s:e])
    outpath = os.path.join(ROOT, "render", f)
    if os.path.exists(outpath) and tracked(os.path.join("render", f)):
        # 追加模式：已入库域文件，原内容一个字不动，只在尾部追加；
        # import 取并集（缺啥补啥），多余的由修剪脚本收。
        cur = open(outpath, encoding="utf-8").read().splitlines(keepends=True)
        have = set(imported_paths(cur))
        missing = [p for p in ctx_import_paths if p not in have]
        if missing:
            for i, ln in enumerate(cur):
                if ln.strip() == ")":
                    ins = "".join('\t"' + p + '"\n' for p in missing)
                    cur.insert(i, ins)
                    break
        cur.append("\n")
        cur.extend(body)
        open(outpath, "w", encoding="utf-8").write("".join(cur))
    else:
        out = []
        out.extend(header)
        out.append("\n")
        out.append("package render\n")
        out.append("\n")
        out.extend(import_block)
        out.append("\n")
        out.extend(body)
        open(outpath, "w", encoding="utf-8").write("".join(out))
    print(f + " funcs=" + str(len(items)))

print("UNMAPPED=" + str(unmapped))
print("DONE")
