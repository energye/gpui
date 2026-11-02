#!/usr/bin/env python3
# gen_p36_testdata.py — 生成 P3.6 批 1 的多帧 / EXIF 测试数据。
#
#   python3 scripts/gen_p36_testdata.py render/testdata/imgfmt
#
# 提交物是产出的字节文件本身，测试运行时不重建（AGENTS 测试数据纪律）。
# 依赖 Pillow（本机 9.0.1 实测）。设计原则：每帧内容互不相同且不对称，
# 使 dispose / blend / 方向的差异能从像素上唯一判定，不靠「能播」过关。
#
# 本机实测的容器事实（写进证据，勿凭规范文档记忆；原稿按文档预留的 3 字节
# ANMF 补白与 2 字节 dispose/blend 都是错的）：
#   · WebP ANMF 固定头 **16 字节**：X/Y/Width-1/Height-1/Duration 各 3 字节 LE + Flags 1 字节，
#     子 chunk（VP8 /VP8L /ALPH）紧跟其后无补白；子 chunk 体即位流本体，
#     可直接喂 golang.org/x/image 的 vp8.Decoder / vp8l.Decode（已端到端验通）。
#   · APNG fcTL 数据 **26 字节**：seq(4) w(4) h(4) xoff(4) yoff(4) delay_num(2) delay_den(2)
#     dispose_op(**1**) blend_op(**1**)。
#   · Pillow 的 APNG 循环次数取 `loop=N` → acTL.num_plays=N（传 `num_plays=` 被忽略）；
#     帧会自动收缩到变化包围盒，因此天然产生 x_offset/y_offset ≠ 0 的子矩形帧。
#   · Pillow 的 GIF duration 单位 ms，stdlib 读回的 Delay 是 1/100 秒整数。
import os
import struct
import sys
import zlib

from PIL import Image

W, H = 40, 30


def canvas():
    im = Image.new("RGBA", (W, H), (20, 30, 40, 255))
    px = im.load()
    for y in range(H):  # 左侧标记列：判旋转 / 镜像用
        for x in range(4):
            px[x, y] = (250, 250, 20, 255)
    return im


def _rect(im, x0, y0, x1, y1, col):
    px = im.load()
    for y in range(y0, min(y1, H)):
        for x in range(x0, min(x1, W)):
            px[x, y] = col


def frame_a():
    im = canvas()
    _rect(im, 6, 6, 30, 14, (200, 40, 40, 255))
    return im


def frame_b():
    im = canvas()
    _rect(im, 18, 10, 42, 16, (40, 200, 60, 255))
    return im


def frame_c():
    im = canvas()
    _rect(im, 26, 18, 42, 34, (60, 80, 240, 255))
    return im


def frame_c_semi():
    """半透明块：让 APNG 的 blend=1（alpha 合成）与 blend=0（覆盖）产出不同像素。"""
    im = canvas()
    _rect(im, 26, 18, 42, 34, (60, 80, 240, 128))
    return im


# 透明 GIF：索引 4 为透明色（stdlib 把该调色板项读成全 alpha=0，
# 见 GOROOT/src/image/gif/reader.go:396-408，因此透明必须落在小色板内才会被保留）
GIF_PAL = [(20, 30, 40), (250, 250, 20), (200, 40, 40), (40, 200, 60), (0, 0, 0)]
GIF_TRANSP = 4


def _p_frame(fill, rects):
    p = Image.new("P", (W, H), fill)
    flat = []
    for c in GIF_PAL:
        flat += list(c)
    p.putpalette(flat + [0] * (256 - len(GIF_PAL)) * 3)
    px = p.load()
    for y in range(H):
        for x in range(W):
            px[x, y] = fill
    for (x0, y0, x1, y1, idx) in rects:
        for y in range(y0, min(y1, H)):
            for x in range(x0, min(x1, W)):
                px[x, y] = idx
    return p


def write_gif_transparent(path):
    """帧 1 只画左侧列与一块前景，其余全透明——合成时那些位置必须留住帧 0 的像素。"""
    f0 = _p_frame(0, [(0, 0, 4, H, 1), (6, 6, 30, 14, 2)])
    f1 = _p_frame(GIF_TRANSP, [(0, 0, 4, H, 1), (18, 10, 42, 16, 3)])
    f1.info["transparency"] = GIF_TRANSP
    f0.save(path, save_all=True, append_images=[f1], duration=[100, 200],
            loop=0, disposal=[1, 1], transparency=GIF_TRANSP)


def write_gif(path, frames, delays_ms, disposal, loop):
    """loop: None=不写循环扩展；0=无限；N>0=N 次额外重复（与 stdlib LoopCount 同义）。"""
    conv = [f.convert("P", palette=Image.ADAPTIVE) for f in frames]
    save = {"save_all": True, "append_images": conv[1:], "duration": delays_ms, "disposal": disposal}
    if loop is not None:
        save["loop"] = loop
    conv[0].save(path, **save)


def write_apng(path, frames, delays_ms, disposal, blend, loop):
    frames[0].save(path, save_all=True, append_images=frames[1:], duration=delays_ms,
                   disposal=disposal, blend=blend, loop=loop)


def patch_apng_delay_den(src, out, den=0):
    """第 0 帧 fcTL 的 delay_den 置 0（畸形样本；上游按 100 处理，Pillow 按 12 倍解释）。"""
    d = bytearray(open(src, "rb").read())
    i = d.find(b"fcTL")
    assert i > 0, "no fcTL"
    struct.pack_into(">H", d, i + 4 + 22, den)  # data 内 delay_den 偏移 22
    length = struct.unpack(">I", d[i - 4:i])[0]
    struct.pack_into(">I", d, i + 4 + length, zlib.crc32(bytes(d[i:i + 4 + length])) & 0xFFFFFFFF)
    open(out, "wb").write(bytes(d))


def exif_with_orientation(ori):
    """最小 EXIF APP1：0th IFD 只写 Orientation=ori（SHORT，值内联 4 字节）。"""
    return (b"Exif\x00\x00" + b"II" + struct.pack("<H", 42) + struct.pack("<I", 8) +
            struct.pack("<H", 1) +                # 目录项数
            struct.pack("<HHI", 0x0112, 3, 1) +   # tag / SHORT / count
            struct.pack("<HH", ori, 0) +          # 值（内联）
            struct.pack("<I", 0))                 # next IFD


def asymmetric_jpeg():
    im = Image.new("RGB", (W, H), (250, 250, 250))
    px = im.load()
    for y in range(H):
        for x in range(W // 2):
            px[x, y] = (220, 30, 30) if y < H // 2 else (30, 30, 220)
    for i in range(min(W, H)):
        px[i, i] = (0, 0, 0)  # 对角带：判转置与镜像
    return im


def write_exif_reference_grids(out_dir):
    """用 Pillow 的 exif_transpose 生成方向参照网格。

    参照必须来自**独立实现**，不能用我方映射自算（那是循环论证）。网格把像素
    归类成 R/B/K/G 四字符，避开 JPEG 色度抽样与两套解码器的逐位差异。
    """
    from PIL import ImageOps

    def label(px):
        r, g, b = px[:3]
        if r > 150 and g < 100 and b < 100:
            return "R"
        if b > 150 and r < 100 and g < 100:
            return "B"
        if r < 90 and g < 90 and b < 90:
            return "K"
        return "G"

    for ori in range(1, 9):
        src = Image.open(f"{out_dir}/exif_{ori}.jpg")
        transposed = ImageOps.exif_transpose(src)
        rgba = transposed.convert("RGB")
        px = rgba.load()
        rows = ["".join(label(px[x, y]) for x in range(rgba.size[0])) for y in range(rgba.size[1])]
        header = f"# orientation={ori} {rgba.size[0]}x{rgba.size[1]}\n"
        with open(f"{out_dir}/exif_grid_{ori}.txt", "w", encoding="ascii") as fh:
            fh.write(header)
            fh.write("\n".join(rows))
            fh.write("\n")


def main(out_dir):
    os.makedirs(out_dir, exist_ok=True)
    A, B, C = frame_a(), frame_b(), frame_c()

    # GIF：disposal 1=保留 2=还原背景 3=还原前一帧
    write_gif(f"{out_dir}/gif_keep.gif", [A, B, C], [100, 200, 300], [1, 1, 1], 0)
    write_gif(f"{out_dir}/gif_restore_bg.gif", [A, B, C], [100, 0, 300], [1, 2, 2], 2)
    write_gif(f"{out_dir}/gif_restore_prev.gif", [A, B, C], [100, 200, 300], [1, 3, 3], 1)
    write_gif(f"{out_dir}/gif_once.gif", [A, B], [150, 250], [1, 1], None)
    write_gif_transparent(f"{out_dir}/gif_transparent.gif")

    # APNG：dispose 0=保留 1=还原背景 2=还原前一帧；blend 0=覆盖 1=alpha 合成
    S = frame_c_semi()
    write_apng(f"{out_dir}/apng_plays0.png", [A, B, S], [100, 200, 300], [0, 1, 0], [0, 1, 1], 0)
    write_apng(f"{out_dir}/apng_plays3.png", [A, B], [120, 240], [0, 1], [1, 0], 3)
    patch_apng_delay_den(f"{out_dir}/apng_plays3.png", f"{out_dir}/apng_delden0.png", den=0)

    # 动图 WebP：有损（含 duration=0 帧、loop=2、子矩形帧）与无损
    A.convert("RGB").save(f"{out_dir}/webp_static_lossy.webp", quality=80)
    A.save(f"{out_dir}/webp_anim_lossy.webp", save_all=True, append_images=[B, C],
           duration=[100, 0, 250], loop=2, lossless=False, quality=80, method=4)
    A.save(f"{out_dir}/webp_anim_lossless.webp", save_all=True, append_images=[S],
           duration=[120, 120], loop=0, lossless=True)

    # EXIF 1..8 + 无 EXIF 对照
    im = asymmetric_jpeg()
    for ori in range(1, 9):
        im.save(f"{out_dir}/exif_{ori}.jpg", exif=exif_with_orientation(ori), quality=92)
    write_exif_reference_grids(out_dir)
    canvas().convert("RGB").save(f"{out_dir}/noexif.jpg", quality=92)

    for name in sorted(os.listdir(out_dir)):
        print(f"{os.path.getsize(os.path.join(out_dir, name)):8d}  {name}")


if __name__ == "__main__":
    main(sys.argv[1] if len(sys.argv) > 1 else "render/testdata/imgfmt")
