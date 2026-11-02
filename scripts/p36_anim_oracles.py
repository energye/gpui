#!/usr/bin/env python3
# p36_anim_oracles.py — 用 Pillow（GIF=自带解析器，APNG=自带解析器，WebP=libwebp demux）
# 把动图逐帧**合成后**的画布导出为 PNG，作为 P3.6 批 1b 合成器的独立参照。
#
#   python3 scripts/p36_anim_oracles.py render/testdata/imgfmt
#
# 参照必须来自独立实现：我方合成器若与它不一致，要么我错、要么它错，两者都可查；
# 用我方自己的语义自算期望值等于没有裁判。
import glob
import json
import os
import subprocess
import sys
import tempfile

from PIL import Image

SAMPLES = [
    "gif_keep.gif",
    "gif_restore_bg.gif",
    "gif_restore_prev.gif",
    "gif_once.gif",
    "gif_transparent.gif",
    "apng_keep.png",
    "apng_restorebg.png",
    "apng_restoreprev.png",
    "apng_alphaover.png",
    "apng_subset.png",
    "apng_plays3.png",
    "apng_delden0.png",
    "apng_keep_erase.png",
    "webp_anim_lossy.webp",
    "webp_anim_lossless.webp",
]


def dump_ffmpeg(path, outdir):
    """APNG 参照走 ffmpeg 的 apng 解码器：alpha 通道与容器规范 §alpha-blending 一致
    （blend.A = src.A + dst.A*(1-src.A/255)）；Pillow 在该通道给的是另一个值。
    两读端的 RGB 已实测逐点一致，差异只在这条 alpha 上。"""
    tmp = tempfile.mkdtemp(prefix="p36ff_") # 参照帧的中间产物不进 testdata
    os.makedirs(tmp, exist_ok=True)
    for f in glob.glob(os.path.join(tmp, "*.png")):
        os.remove(f)
    subprocess.run(["ffmpeg", "-hide_banner", "-loglevel", "error", "-y",
                    "-i", path, "-vsync", "0", os.path.join(tmp, "%02d.png")], check=True)
    outs = sorted(glob.glob(os.path.join(tmp, "*.png")))
    meta = {"file": os.path.basename(path), "size": list(Image.open(outs[0]).size),
            "n_frames": len(outs), "frames": [], "reference": "ffmpeg apng decoder"}
    for k, f in enumerate(outs):
        im = Image.open(f).convert("RGBA")
        im.save(os.path.join(outdir, f"{os.path.basename(path)}.frame{k}.png"))
        meta["frames"].append({"index": k, "size": list(im.size)})
    with open(os.path.join(outdir, os.path.basename(path) + ".meta.json"), "w", encoding="utf-8") as fh:
        json.dump(meta, fh, ensure_ascii=False, indent=1)
    return meta


def dump(path, outdir):
    """逐帧导出**合成后**的画布 PNG（参照），并落一份 Pillow 读到的元数据。"""
    im = Image.open(path)
    n = getattr(im, "n_frames", 1)
    meta = {"file": os.path.basename(path), "size": list(im.size), "n_frames": n, "frames": []}
    for k in range(n):
        im.seek(k)
        im.load()
        rgba = im.convert("RGBA")
        info = {}
        for key, val in im.info.items():
            if key not in ("duration", "disposal_method", "transparency", "loop", "blend", "version"):
                continue
            # GIF/APNG 的 transparency 有时是 bytes，JSON 不认，展开成字节列表再落盘。
            if isinstance(val, (bytes, bytearray)):
                val = list(val)
            info[key] = val
        meta["frames"].append({"index": k, "size": list(rgba.size), "info": info})
        rgba.save(os.path.join(outdir, f"{os.path.basename(path)}.frame{k}.png"))
    with open(os.path.join(outdir, os.path.basename(path) + ".meta.json"), "w", encoding="utf-8") as fh:
        json.dump(meta, fh, ensure_ascii=False, indent=1)
    return meta


def main(src_dir):
    out_dir = os.path.join(src_dir, "oracle")
    os.makedirs(out_dir, exist_ok=True)
    for name in SAMPLES:
        p = os.path.join(src_dir, name)
        if not os.path.exists(p):
            print(f"skip {name} (missing)")
            continue
        m = dump_ffmpeg(p, out_dir) if name.startswith("apng_") else dump(p, out_dir)
        durs = [f.get("info", {}).get("duration") for f in m["frames"]]
        loops = {f.get("info", {}).get("loop") for f in m["frames"]}
        print(f"{name:24s} n={m['n_frames']} size={m['size']} duration={durs} loop={sorted(x for x in loops if x is not None)}")


if __name__ == "__main__":
    main(sys.argv[1] if len(sys.argv) > 1 else "render/testdata/imgfmt")
