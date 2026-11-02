#!/usr/bin/env python3
# gen_p36_testdata.py — 生成 P3.6 批 1 的多帧 / EXIF 测试数据。
#
#   python3 scripts/gen_p36_testdata.py render/testdata/imgfmt
#
# 提交物是产出的字节文件本身，测试运行时不重建（AGENTS 测试数据纪律）。
# 依赖 Pillow 9.0.1（本机实测）。设计原则：每帧内容互不相同且不对称，使
# dispose / blend / 方向的差异能从像素上唯一判定，不靠「能播」过关。
#
# 为什么 GIF 与 APNG 都自己装配而不用 Pillow 的写端（本机实测结论，别改回去）：
#   · Pillow 的多帧 GIF 写端对每帧单独 ADAPTIVE 量化 → 各帧局部色板，索引同值不同
#     色；dispose=background 用全局背景索引清屏时，合成帧里出现源图根本没有的颜色
#     （实测 (20,0,0)、(200,60,250)）。故所有帧统一走同一张全局色板。
#   · Pillow 9.0.1 的 APNG 写端会写出整块全透明的 fdAT（该处源图明明有新颜色），
#     Pillow 与 ffmpeg 两个独立读端都把该帧读成「什么都没画」→ 样本本身就是坏的。
#     故 APNG 按规范自己装配：fcTL 26 字节（dispose_op/blend_op 各 1 字节），
#     fdAT = 4 字节 sequence_number + 与 IDAT 同形的 zlib 流，序列号按出现顺序连续。
# 其余容器事实见 evidence/P3.6/batch1-containers-probe.md。
import io
import os
import struct
import sys
import zlib

from PIL import Image

W, H = 40, 30


def canvas():
    im = Image.new("RGBA", (W, H), (20, 30, 40, 255))
    _rect(im, 0, 0, 4, H, (250, 250, 20, 255))  # 左侧标记列：判旋转 / 镜像用
    return im


def _rect(im, x0, y0, x1, y1, col):
    px = im.load()
    for y in range(y0, min(y1, im.size[1])):
        for x in range(x0, min(x1, im.size[0])):
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
    """半透明块：让 blend=alpha 与 blend=覆盖 产出不同像素。"""
    im = canvas()
    _rect(im, 26, 18, 42, 34, (60, 80, 240, 128))
    return im


def frame_sparse():
    """只画一小块，其余全透明：合成时其余位置必须留住画布。"""
    im = Image.new("RGBA", (W, H), (0, 0, 0, 0))
    _rect(im, 0, 0, 4, H, (250, 250, 20, 255))
    _rect(im, 18, 10, 42, 16, (40, 200, 60, 255))
    return im


def frame_dot():
    im = Image.new("RGBA", (W, H), (0, 0, 0, 0))
    _rect(im, 12, 20, 20, 28, (240, 240, 20, 255))
    return im


# ───────────────────────────── GIF ─────────────────────────────
#
# GIF 也自己装配：Pillow 的多帧写端会为每帧重排调色板索引，导致「透明处留住
# 画布」被写成黑块、dispose=background 清出源图根本没有的颜色（实测 Pillow 与
# ffmpeg 两个读端都同意这个坏结果，说明坏在文件本身）。这里逐字节控制：全局色板
# 一份、每帧 GCE 的 disposal/透明位显式写、NETSCAPE 循环扩展显式写。
# LZW 用「永不增长」编码（只发清码与字面码，码宽固定在 min+1），任何解码器都认。

# 索引 0..4 是帧内容真正用的颜色；5 是占位（免得黑色跟透明索引撞车）；
# 6 是透明索引，它的 RGB 无意义，是否可见由 GCE 的透明位决定。
GIF_COLORS = [(20, 30, 40), (250, 250, 20), (200, 40, 40), (40, 200, 60), (60, 80, 240),
              (128, 0, 128), (0, 0, 0)]
GIF_TRANSPARENT_INDEX = 6
GIF_CONTENT_COLORS = GIF_COLORS[:5]


def _lzw_uncompressed(indices, min_code=8):
    """只发清码与字面码的 LZW 流（字典不新增条目，码宽恒定）。

    关键：解码器每读一个码（清码后的第一个除外）就会给自己加一条字典项，码宽在
    表长达到 2^width 时自动加一档。所以必须在**逼近该阈值之前**发清码把表长打回
    原形，否则位宽错位、整幅图变噪声（本机实测错过一次：按 pos>=2^width 清码时
    Pillow 直接报 cannot identify image file）。留 2 个码的余量。
    """
    clear = 1 << min_code
    out_bits = []
    width = min_code + 1
    size = clear + 2  # 解码器表长：256 色 + 清码 + 未用码

    def emit(code, w):
        for b in range(w):
            out_bits.append((code >> b) & 1)

    emit(clear, width)
    size += 1
    for idx in indices:
        if size >= (1 << width) - 2:
            emit(clear, width)
            width = min_code + 1
            size = clear + 2
        emit(idx, width)
        size += 1
    emit(clear, width)
    while len(out_bits) % 8:
        out_bits.append(0)
    data = bytearray()
    for i in range(0, len(out_bits), 8):
        byte = 0
        for b in range(8):
            byte |= out_bits[i + b] << b
        data.append(byte)
    return bytes(data)


def _subblocks(data):
    out = b""
    i = 0
    while i < len(data):
        chunk = data[i:i + 255]
        out += bytes([len(chunk)]) + chunk
        i += 255
    return out + b"\x00"


def write_gif(path, frames, delays_ms, disposal, loop, rects=None):
    """loop: None=不写循环扩展；0=无限；N>0=N 次额外重复（与 stdlib LoopCount 同义）。

    disposal 用 GIF 原生值：0=未指定 1=保留 2=还原背景 3=还原前一帧。
    rects[k]=(x,y,w,h) 给第 k 帧子矩形（缺省整画布）——dispose 的差异只有配合
    子矩形才看得见，全画布帧会把上一帧整个盖掉。
    帧里 alpha==0 的像素落透明索引，其余按 GIF_CONTENT_COLORS 精确匹配。
    """
    w0, h0 = frames[0].size
    if rects is None:
        rects = [(0, 0, w0, h0)] * len(frames)
    color_lookup = {c: i for i, c in enumerate(GIF_CONTENT_COLORS)}
    gct = bytearray()
    for c in GIF_COLORS:
        gct += bytes(c)
    while len(gct) < 3 * 256:
        gct += b"\x00\x00\x00"

    out = b"GIF89a" + w0.to_bytes(2, "little") + h0.to_bytes(2, "little")
    out += bytes([0xF7, 0, 0])  # GCT 存在、8 位色深（表长 256）、背景索引 0
    out += bytes(gct)
    if loop is not None:
        out += b"\x21\xFF\x0B" + b"NETSCAPE2.0" + b"\x03\x01" + loop.to_bytes(2, "little") + b"\x00"
    for k, im in enumerate(frames):
        x, y, fw, fh = rects[k]
        rgba = im.convert("RGBA").crop((x, y, x + fw, y + fh))
        px = rgba.load()
        indices = []
        transparent = False
        for yy in range(fh):
            for xx in range(fw):
                r, g, b, a = px[xx, yy]
                if a == 0:
                    indices.append(GIF_TRANSPARENT_INDEX)
                    transparent = True
                    continue
                idx = color_lookup.get((r, g, b))
                if idx is None:
                    raise ValueError(f"帧 {k} 有调色板外的颜色 {(r, g, b)}")
                indices.append(idx)
        tindex = GIF_TRANSPARENT_INDEX if transparent else 0
        # GCE 负载恰好 4 字节：packed(bit0 透明位、bit2..4 dispose) + 延迟 2 字节
        # (1/100 秒) + 透明索引。多写一字节会被读端判成「扩展块终止符非法」。
        packed = (0x01 if transparent else 0x00) | ((disposal[k] & 7) << 2)
        out += bytes([0x21, 0xF9, 4, packed, (delays_ms[k] // 10) & 0xFF, (delays_ms[k] // 10) >> 8,
                      tindex, 0x00])
        out += bytes([0x2C]) + x.to_bytes(2, "little") + y.to_bytes(2, "little")
        out += fw.to_bytes(2, "little") + fh.to_bytes(2, "little") + bytes([0x00])
        out += bytes([8]) + _subblocks(_lzw_uncompressed(indices, 8))
    out += b"\x3B"
    with open(path, "wb") as fh:
        fh.write(out)


# ───────────────────────────── APNG ─────────────────────────────

def _ck(typ, data):
    return (struct.pack(">I", len(data)) + typ + data +
            struct.pack(">I", zlib.crc32(typ + data) & 0xFFFFFFFF))


def _png_bytes(im):
    buf = io.BytesIO()
    im.save(buf, format="PNG")
    return buf.getvalue()


def _idat_of(png_bytes):
    out = b""
    i = 8
    while i + 12 <= len(png_bytes):
        ln = struct.unpack(">I", png_bytes[i:i + 4])[0]
        typ = png_bytes[i + 4:i + 8]
        if typ == b"IDAT":
            out += png_bytes[i + 8:i + 8 + ln]
        i += 12 + ln
    return out


def write_apng(path, frames, delays_ms, disposal, blend, num_plays, rects=None):
    """按规范装配 APNG。rects[k] = (x, y, w, h) 给第 k 帧子矩形，缺省整画布。

    num_plays 是 APNG 的**总播放次数**（0=无限），与 GIF 的 LoopCount / WebP 的
    loop_count 那种「额外重复次数」差一次，映射时必须单独钉。
    """
    w0, h0 = frames[0].size
    if rects is None:
        rects = [(0, 0, w0, h0)] * len(frames)
    out = b"\x89PNG\r\n\x1a\n"
    out += _ck(b"IHDR", struct.pack(">IIBBBBB", w0, h0, 8, 6, 0, 0, 0))
    out += _ck(b"acTL", struct.pack(">II", len(frames), num_plays))
    seq = 0
    for k, im in enumerate(frames):
        x, y, fw, fh = rects[k]
        out += _ck(b"fcTL", struct.pack(">IIIIIHHBB", seq, fw, fh, x, y, delays_ms[k], 1000,
                                        disposal[k], blend[k]))
        seq += 1
        sub = im.convert("RGBA").crop((x, y, x + fw, y + fh))
        if k == 0:
            out += _ck(b"IDAT", _idat_of(_png_bytes(sub)))
        else:
            out += _ck(b"fdAT", struct.pack(">I", seq) + _idat_of(_png_bytes(sub)))
            seq += 1
    out += _ck(b"IEND", b"")
    with open(path, "wb") as fh:
        fh.write(out)


def patch_apng_delay_den(src, out, den=0):
    """第 0 帧 fcTL 的 delay_den 置 0（畸形样本；上游按 100 处理，Pillow 按 12 倍）。"""
    d = bytearray(open(src, "rb").read())
    i = d.find(b"fcTL")
    assert i > 0, "no fcTL"
    struct.pack_into(">H", d, i + 4 + 22, den)  # fcTL 数据内 delay_den 偏移 22
    length = struct.unpack(">I", d[i - 4:i])[0]
    struct.pack_into(">I", d, i + 4 + length, zlib.crc32(bytes(d[i:i + 4 + length])) & 0xFFFFFFFF)
    with open(out, "wb") as fh:
        fh.write(bytes(d))


# ───────────────────────────── EXIF ─────────────────────────────

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
    归类成 R/B/K/G 四类，避开 JPEG 色度抽样与两套解码器的逐位差异。
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
        view = ImageOps.exif_transpose(src).convert("RGB")
        px = view.load()
        rows = ["".join(label(px[x, y]) for x in range(view.size[0])) for y in range(view.size[1])]
        with open(f"{out_dir}/exif_grid_{ori}.txt", "w", encoding="ascii") as fh:
            fh.write(f"# orientation={ori} {view.size[0]}x{view.size[1]}\n")
            fh.write("\n".join(rows))
            fh.write("\n")


def frame_semi_over_red():
    """半透明块压在帧 0 的红块上：只有 blend=alpha 才会得到混色。"""
    im = Image.new("RGBA", (W, H), (0, 0, 0, 0))
    _rect(im, 10, 8, 22, 16, (60, 80, 240, 128))
    return im


# ───────────────────────────── main ─────────────────────────────

def main(out_dir):
    os.makedirs(out_dir, exist_ok=True)
    A, B, C = frame_a(), frame_b(), frame_c()
    SP, DOT = frame_sparse(), frame_dot()
    SEMI = frame_semi_over_red()

    # 子矩形：dispose 的差异只有配合子帧才看得见（全画布帧会把上一帧整个盖掉）。
    rB, rC = (18, 10, 22, 6), (26, 18, 14, 12)
    rErase = (10, 8, 12, 6)  # B 在该处是底色，等于把红块擦掉一块

    # GIF：disposal 1=保留 2=还原背景 3=还原前一帧
    write_gif(f"{out_dir}/gif_keep.gif", [A, B, C], [100, 200, 300], [1, 1, 1], 0,
              rects=[(0, 0, W, H), rB, rC])
    write_gif(f"{out_dir}/gif_restore_bg.gif", [A, B, C], [100, 0, 300], [1, 2, 2], 2,
              rects=[(0, 0, W, H), rB, rC])
    # 还原前一帧 vs 保留：f1 擦掉一块红，f2 之后红该回来（3）还是留在擦除态（1）
    write_gif(f"{out_dir}/gif_restore_prev.gif", [A, B, C], [100, 200, 300], [1, 3, 1], 1,
              rects=[(0, 0, W, H), rErase, rC])
    write_gif(f"{out_dir}/gif_once.gif", [A, B], [150, 250], [1, 1], None)
    # 透明 + 保留：帧 1 只画黄列与绿块，其余透明 → 那些位置必须留住帧 0 的红块
    write_gif(f"{out_dir}/gif_transparent.gif", [A, SP], [100, 200], [1, 1], 0)

    # APNG：dispose 0=保留 1=还原背景 2=还原前一帧；blend 0=覆盖 1=alpha 合成
    write_apng(f"{out_dir}/apng_keep.png", [A, B, C], [100, 200, 300], [0, 0, 0], [0, 0, 0], 0,
               rects=[(0, 0, W, H), rB, rC])
    write_apng(f"{out_dir}/apng_restorebg.png", [A, B, C], [100, 200, 300], [0, 1, 0], [0, 0, 0], 0,
               rects=[(0, 0, W, H), rB, rC])
    write_apng(f"{out_dir}/apng_restoreprev.png", [A, B, C], [100, 200, 300], [0, 2, 0], [0, 0, 0], 0,
               rects=[(0, 0, W, H), rErase, rC])
    # 与上一行只差第 1 帧的 dispose 位（0=保留 vs 2=还原前一帧），帧内容与矩形完全相同，
    # 这样 (10,10) 的差别只能来自 dispose 语义本身。
    write_apng(f"{out_dir}/apng_keep_erase.png", [A, B, C], [100, 200, 300], [0, 0, 0], [0, 0, 0], 0,
               rects=[(0, 0, W, H), rErase, rC])
    write_apng(f"{out_dir}/apng_alphaover.png", [A, SEMI], [100, 200], [0, 0], [0, 1], 0,
               rects=[(0, 0, W, H), (10, 8, 12, 8)])
    write_apng(f"{out_dir}/apng_subset.png", [A, SP, DOT], [90, 90, 90], [0, 0, 0], [0, 1, 1], 0,
               rects=[(0, 0, W, H), (0, 0, W, H), (12, 20, 8, 8)])
    write_apng(f"{out_dir}/apng_plays3.png", [A, B], [120, 240], [0, 1], [1, 0], 3)
    patch_apng_delay_den(f"{out_dir}/apng_plays3.png", f"{out_dir}/apng_delden0.png", den=0)

    # 动图 WebP：有损（含 duration=0 帧、loop=2、子矩形帧）与无损（含半透明块）。
    # 注意 ffmpeg 不解析动图 WebP（实测 "image data not found"），故这两个样本的
    # 独立读端只有 Pillow（走 libwebp demux，即参考实现），字节层结构另由探针核对。
    SEMI_FULL = frame_c_semi()
    A.convert("RGB").save(f"{out_dir}/webp_static_lossy.webp", quality=80)
    A.save(f"{out_dir}/webp_anim_lossy.webp", save_all=True, append_images=[B, C],
           duration=[100, 0, 250], loop=2, lossless=False, quality=80, method=4)
    A.save(f"{out_dir}/webp_anim_lossless.webp", save_all=True, append_images=[SEMI_FULL],
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
