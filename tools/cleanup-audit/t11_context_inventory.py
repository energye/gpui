"""T1.1 搬家清单生成：render/context.go 全函数按域分组（签名不动，只定去向）。"""
import re
fp = "/home/yanghy/app/projects/gogpu/gpui/render/context.go"
lines = open(fp, encoding="utf-8", errors="ignore").read().splitlines()
pat = re.compile(r'^func\s+(?:\(\s*\w+\s+\*?(\w+)\)\s+)?([A-Za-z_]\w*)')
groups = [
    ("状态存取", ["Save", "Restore", "Get", "Set", "Reset", "Identity", "Current"]),
    ("变换", ["Transform", "Translate", "Scale", "Rotate", "Concat", "Invert", "DeviceScale", "RoundTrip"]),
    ("路径", ["Path", "MoveTo", "LineTo", "Quad", "Cubic", "Bezier", "Arc", "Circle", "Ellipse", "Rect", "Round", "Close", "Clear", "Append", "Flatten", "Contains", "Bounding", "Length", "Area", "Winding", "Revers", "Verb", "HasCurve", "Point"]),
    ("裁剪", ["Clip"]),
    ("绘制", ["Draw", "Fill", "Stroke", "Paint", "Image", "Pixmap", "Video", "Frame", "Present", "Flush", "Submit", "Blit"]),
    ("文本", ["Text", "String", "Glyph", "Font", "Shap", "Emoji"]),
    ("样式画刷", ["Brush", "Gradient", "Pattern", "Solid", "Color", "Shade", "Mask", "Blend", "Filter", "Thick", "RoundStroke", "SquareStroke", "Dotted", "Checkerboard", "Stripes"]),
    ("离屏与层", ["Layer", "Push", "Pop", "Offscreen", "Surface", "Pixmap", "With"]),
    ("构造", ["New"]),
    ("求解器", ["Solve", "Cubic"]),
    ("向量", ["Vec", "PointTo"]),
]
rows = []
for i, ln in enumerate(lines, 1):
    m = pat.match(ln)
    if m:
        recv, name = m.group(1) or "", m.group(2)
        if name.startswith("Test") or name.startswith("Benchmark"):
            continue
        g = "其他"
        for gn, keys in groups:
            if any(k in name for k in keys):
                g = gn
                break
        rows.append((i, recv, name, g))
from collections import Counter
c = Counter(r[3] for r in rows)
print("TOTAL=" + str(len(rows)))
print("GROUPS=" + str(dict(c)))
for i, recv, name, g in rows:
    print(str(i) + "|" + recv + "|" + name + "|" + g)
