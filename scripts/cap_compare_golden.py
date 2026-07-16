#!/usr/bin/env python3
"""Compare capability_matrix capture PNGs against golden corpus (RMSE / SSIM)."""
from __future__ import annotations
import argparse, json, math, os, sys
from pathlib import Path

try:
    from PIL import Image
except ImportError:
    print("Pillow required: pip install pillow", file=sys.stderr)
    sys.exit(2)

def load_rgb(path: Path):
    im = Image.open(path).convert("RGB")
    return im

def _fit(im: Image.Image, max_w: int = 400) -> Image.Image:
    w, h = im.size
    if w <= max_w:
        return im
    nh = max(1, int(h * (max_w / float(w))))
    return im.resize((max_w, nh), Image.BILINEAR)

def rmse(a: Image.Image, b: Image.Image) -> float:
    a, b = _fit(a), _fit(b)
    if a.size != b.size:
        b = b.resize(a.size, Image.BILINEAR)
    pa, pb = list(a.getdata()), list(b.getdata())
    n = len(pa)
    if n == 0:
        return 0.0
    acc = 0.0
    for (r1,g1,b1),(r2,g2,b2) in zip(pa, pb):
        acc += (r1-r2)**2 + (g1-g2)**2 + (b1-b2)**2
    return math.sqrt(acc / (n * 3)) / 255.0

def ssim_approx(a: Image.Image, b: Image.Image) -> float:
    """Lightweight SSIM on 64x48 luminance (not full ITU SSIM, good for gate)."""
    if a.size != b.size:
        b = b.resize(a.size, Image.BILINEAR)
    a2 = a.resize((64, 48), Image.BILINEAR).convert("L")
    b2 = b.resize((64, 48), Image.BILINEAR).convert("L")
    pa = list(a2.getdata())
    pb = list(b2.getdata())
    n = len(pa)
    ma = sum(pa)/n
    mb = sum(pb)/n
    va = sum((x-ma)**2 for x in pa)/n
    vb = sum((x-mb)**2 for x in pb)/n
    cov = sum((pa[i]-ma)*(pb[i]-mb) for i in range(n))/n
    c1, c2 = (0.01*255)**2, (0.03*255)**2
    num = (2*ma*mb + c1) * (2*cov + c2)
    den = (ma*ma + mb*mb + c1) * (va + vb + c2)
    if den <= 1e-12:
        return 1.0
    return max(0.0, min(1.0, num/den))

def write_diff(a: Image.Image, b: Image.Image, out: Path):
    if a.size != b.size:
        b = b.resize(a.size, Image.BILINEAR)
    da = a.load(); db = b.load()
    w,h = a.size
    d = Image.new("RGB", (w,h))
    dd = d.load()
    for y in range(h):
        for x in range(w):
            r1,g1,b1 = da[x,y]; r2,g2,b2 = db[x,y]
            dr = abs(r1-r2); dg=abs(g1-g2); dbv=abs(b1-b2)
            # amplify diffs for visibility
            s = min(255, (dr+dg+dbv)*3)
            dd[x,y] = (s, s//2, 0) if s>8 else (0,0,0)
    d.save(out)

def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("--got", required=True, help="directory of captured PNGs")
    ap.add_argument("--golden", required=True, help="directory of golden PNGs")
    ap.add_argument("--diff-dir", default="", help="optional diff PNG output dir")
    ap.add_argument("--max-rmse", type=float, default=0.08)
    ap.add_argument("--min-ssim", type=float, default=0.90)
    ap.add_argument("--report", default="")
    ap.add_argument("--ids", default="", help="comma IDs; empty = all golden/*.png")
    args = ap.parse_args()
    golden = Path(args.golden)
    got = Path(args.got)
    if args.ids.strip():
        ids = [x.strip() for x in args.ids.split(",") if x.strip()]
    else:
        ids = sorted(p.stem for p in golden.glob("C*.png"))
    rows = []
    fails = 0
    if args.diff_dir:
        Path(args.diff_dir).mkdir(parents=True, exist_ok=True)
    for i in ids:
        g = golden / f"{i}.png"
        c = got / f"{i}.png"
        if not g.exists():
            rows.append({"id": i, "status": "SKIP", "reason": "no_golden"})
            continue
        if not c.exists():
            rows.append({"id": i, "status": "FAIL", "reason": "no_capture"})
            fails += 1
            continue
        ga, ca = load_rgb(g), load_rgb(c)
        r = rmse(ga, ca)
        s = ssim_approx(ga, ca)
        ok = (r <= args.max_rmse) and (s >= args.min_ssim)
        if not ok:
            fails += 1
        if args.diff_dir and not ok:
            write_diff(ga, ca, Path(args.diff_dir)/f"{i}_diff.png")
        rows.append({
            "id": i, "status": "PASS" if ok else "FAIL",
            "rmse": round(r, 6), "ssim": round(s, 6),
            "max_rmse": args.max_rmse, "min_ssim": args.min_ssim,
        })
        print(f"{i} {rows[-1]['status']} rmse={r:.4f} ssim={s:.4f}")
    report = {
        "got": str(got), "golden": str(golden),
        "max_rmse": args.max_rmse, "min_ssim": args.min_ssim,
        "pass": sum(1 for r in rows if r["status"]=="PASS"),
        "fail": fails,
        "rows": rows,
    }
    if args.report:
        Path(args.report).write_text(json.dumps(report, indent=2, ensure_ascii=False))
        print("report", args.report)
    print(f"SUMMARY pass={report['pass']} fail={fails}")
    sys.exit(0 if fails==0 else 1)

if __name__ == "__main__":
    main()
