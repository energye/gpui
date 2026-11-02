# 批 1 步骤 0：容器结构一手核对（2026-11-02）

直话：动图解析的三个关键单位问题全部用实测锁定，其中两条**与我按规范文档的原稿记忆相反**，一条**与上游参考框架的依赖假设相反**（动图 WebP 不需要 libwebp）。这些数值是批 1 解析器的实现依据，任何一条记错都会让每一帧错位或每帧重新上传纹理。

取证方式（三路交叉，全部本机可复现）：
1. 造样本：`python3 scripts/gen_p36_testdata.py render/testdata/imgfmt`（Pillow 9.0.1 本机实测；另用 ffmpeg `libwebp_anim` 产出第二编码器样本对照）。
2. 读字节：Go 探针把 ANMF/fcTL/acTL 逐字段打印，并**把 ANMF 里的 VP8/VP8L 子帧原样喂给 `x/image` 的解码器**（即引擎将要走的同一批 API）。
3. 求地面真值：用 Pillow 自身（走 libwebp demux，等价参考实现）seek 每一帧，比对子帧内容在画布上的绝对位置，反推字段单位。

## 1 WebP 动图（`VP8X` / `ANIM` / `ANMF`）

| 项 | 实测结论 | 证据 |
|---|---|---|
| `VP8X` | 10 字节；flags bit1=animation、bit3=EXIF、bit4=alpha、bit5=ICC（与 `x/image/webp/decode.go:163-169` 常量一致）；Canvas 是 `(W-1,H-1)` 各 3 字节 LE，位于 body[4:7]/body[7:10] | 探针：`flags=0x12 anim=true alpha=true canvas=40x30` |
| `ANIM` | 6 字节：背景色 4B（BGRX）+ **loop_count 2B LE；0 = 无限** | `ANIM bg=00000000 loopCount=0` / `loopCount=2` |
| **`ANMF` 固定头 = 16 字节** | `X(3) Y(3) W-1(3) H-1(3) Duration(3) Flags(1)`；**子 chunk 紧跟其后，没有任何补白** | 我原稿按文档记忆预留了 3 字节 unused，探针在 `body[16:19]` 直接读到 ASCII `V8 `（子 chunk fourcc 的头三字节）→ 预留即错 |
| **X/Y 字段单位 = 2 像素**（存的是 `实际偏移/2`） | frame1 存 `X=3 Y=3`，子帧内容里绿块位于子帧内 `(12,4)`，Pillow 合成后绿块在画布 `(18,10)` → 实际偏移 `= 18-12 = 6 = 3×2`；frame2 同理 `X=9→18`、`Y=5→10` | 探针（解子帧）+ Pillow 合成绝对坐标，两者反推一致 |
| **W/H 字段单位 = 1 像素**（存的是 `实际尺寸-1`） | 存 `w-1=33` → 34，与 `vp8` 解出的子帧 `34x10` 逐位相符（**不除以 2**，与 X/Y 不对称） | 同上 |
| Duration | 3B LE，单位 ms；**0 允许**（我造的 `gif/webp` 样本里 frame1 就是 0，编码器如实写出） | `ANMF ... dur=0ms` |
| Flags | bit0=use_alpha、bit1=**dispose_to_background**、bit2=**blend**；两编码器（Pillow、ffmpeg）都对**第 0 帧**写 `flags=0x02`、后续帧写 0x00 | `flags=0b010(alpha=0,dispose=1,blend=0)` ×两个编码器一致 |
| 子帧 payload 可直接喂现有解码器 | `VP8 ` 子 chunk 体以 3 字节 frame tag + `9d 01 2a` 起始码开头，原样 `vp8.NewDecoder().Init()` → `DecodeFrameHeader()` 得到 `34x10 keyframe=true xScale=0 yScale=0`，`DecodeFrame()` 无错；`VP8L` 子 chunk 体以签名 `0x2f` 开头，原样 `vp8l.DecodeConfig/Decode` → `cfg=34x24 err=<nil>` | 探针输出（三帧全绿）；静态路径同调法见 `x/image/webp/decode.go:76-99,146` |
| 动图 WebP **不需要 libwebp** | 依赖已在 `go.mod`（`golang.org/x/image v0.44.0`）且 `vp8`/`vp8l`/`riff` 是公开包；只需自研 `ANIM/ANMF` 容器层 | 上两条 + `assessment.md` §3.1 |
| 待办核对项（关闭） | 原担心「ANMF 内 VP8 需按规范做 scale/偏移手术」——实测 `XScale=YScale=0`、帧尺寸与 ANMF W/H 逐帧相符，**无需剥字节**；但解析器仍应**拒收** `ShowFrame=false` 或 `XScale/YScale≠0` 的帧（畸形样本另立单测，不静默画错） | 探针 + `x/image/vp8/decode.go:35-44` 字段定义 |

## 2 APNG（`acTL` / `fcTL` / `fdAT`）

| 项 | 实测结论 | 证据 |
|---|---|---|
| **`fcTL` 数据 = 26 字节**，`dispose_op` 与 `blend_op` 是**单字节** | 原稿按 2 字节读，读到的是 CRC 之后的下一个 chunk 头（Go 切片不越界所以静默出脏值：`dispose=256 blend=11348` 这类）。真实布局：`seq(4) w(4) h(4) xoff(4) yoff(4) delay_num(2) delay_den(2) dispose(1) blend(1)` | hex `00000000 00000028 0000001e 00000000 00000000 0078 0000 00 01`（帧 0：seq=0 40x30 @(0,0) num=120 den=0 dispose=0 blend=1） |
| `acTL` = `num_frames(4) num_plays(4)`；**num_plays 0 = 无限** | `acTL num_frames=3 num_plays=0` / `num_frames=2 num_plays=3` | 探针 |
| 帧会**自动收缩成子矩形** | 我提交的是三张全画布 RGBA，Pillow 写出 `fcTL w=34 h=10 x=6 y=6` 等变化包围盒 → 子矩形路径天然覆盖（不需手工造） | 探针（与 GIF 侧同样的 bbox 收缩） |
| **fdAT→IDAT 改写**（照上游 APNG 生成器的做法） | `fdAT` 数据 = `sequence_number(4)` + 与 IDAT 同形的 zlib 流；解析时剥掉前 4 字节序号即可复用单帧 PNG 解码（`flutter/engine/lib/ui/painting/image_generator_apng.cc:499-500,576-580` 同构） | 探针 `fdAT seq=2 dataLen=46` |
| **`delay_den==0` 的口径分歧** | 上游把分母 0 当 100（`image_generator_apng.cc:476-480`）；本机 Pillow 把自己的 den=0 文件读成 1200ms（放大 12 倍）。我方**采上游口径**（=100），并在单测里把这个分歧显式钉住，避免后来者以为两边一致 | 样本 `apng_delden0.png`（fcTL num=120 den=0） |

## 3 GIF（stdlib `image/gif` 能力面实测）

| 项 | 实测结论 | 证据 |
|---|---|---|
| `Delay []int` 单位 = **1/100 秒** | 我传 ms（100/200/300）→ 读出 `[10 20 30]`；传 0 → 读出 0（允许） | `GOROOT/src/image/gif/reader.go`（`Delay` 字段注释）+ 探针 |
| `Disposal []byte` 取值 | 1=保留、2=还原背景、3=还原前一帧（0=未指定，按保留处理）；与上游 `Blend/Disposal` 枚举可一一对应 | `gif_restore_bg/prev` 探针读出 `[1 2 2]`/`[1 3 3]` |
| **`LoopCount` 语义**（关键映射） | `0`=无限循环、`-1`=只播一次、`N>0`=共播 N+1 次（即额外重复 N 次） | `GOROOT/src/image/gif/reader.go:577-582` 注释原文 + 探针 `-1/0/1/2` 四种样本 |
| 帧子矩形 | `gif.GIF.Image[i].Bounds()` 直接带偏移（`(6,6)-(40,16)`），即 `DisposalRect` 的来源，无需另解 GCE | 探针 |
| **透明没有独立字段** | stdlib 把透明索引表达成**调色板里一个 alpha=0 的条目**（`reader.go:396-408`，越界索引还会扩调色板）；`gif.GIF` 不暴露 `Transparency`。→ 合成时必须按「palette 项 alpha==0 的索引 = 不写画布」实现 | 样本 `gif_transparent.gif` 探针 `transparentIdx=4`；注：透明索引必须落在**小色板**内才不被 Pillow 写丢（256 色板时 Pillow 会重量化并丢弃 `transparency`，实测第一版样本 `transparentIdx=-1`） |

## 4 落到我方公开语义的映射表（写死，避免再猜）

| 容器 | Duration | 循环 | Blend | Disposal | RequiredFrame | 帧矩形 |
|---|---|---|---|---|---|---|
| GIF | `Delay×10ms` | `0→-1`、`-1→0`、`N→N`（-1=无限） | GIF 无 blend 概念 → 非透明像素恒 `Src`，透明像素保留底图 | `1→Keep`、`2→RestoreBGColor`、`3→RestorePrevious`、`0→Keep` | 恒「无需前帧」（GIF 帧自足，透明只是不覆盖） | `Image[i].Bounds()` |
| APNG | `num/den×1000ms`，`den==0` 按 100 | `num_plays 0→-1`、`N→N-1`（total plays 与 extra repeats 的差） | `blend 0→Src`、`1→SrcOver` | `0→Keep`、`1→RestoreBGColor`、`2→RestorePrevious` | 恒「无需前帧」（`blend=SrcOver` 时底图来自上一帧画布，由 `RequiredFrame=上一帧` 表达） | `fcTL (x,y,w,h)` |
| WebP | `dur ms`（0 允许） | `loop_count 0→-1`、`N→N` | `flags bit2 0→Src`、`1→SrcOver` | `flags bit1 0→Keep`、`1→RestoreBGColor`（WebP 只有两档，无 RestorePrevious） | 恒「无需前帧」（关键帧序列） | `(X×2, Y×2, W, H)`（**X/Y 单位 2 像素**） |

上游 `RepetitionCount` 口径：负=无限、非负=**额外**重复次数（`flutter/engine/lib/ui/painting/image_generator.cc:118-121`）；APNG 的 `num_plays` 是**总播放次数**，所以映射成 `N-1` 必须在单测里单独钉（GIF/WebP 是额外次数，APNG 是总次数——三者不同，最容易搞错的一处）。

## 5 复现命令

```bash
# 样本
python3 scripts/gen_p36_testdata.py render/testdata/imgfmt
# 结构核对（探针为会话内 scratch，字段与结论以本文表格为准；批 1 的单测会把这些字段级断言固化进仓库）
#   关键三条：ANMF 头 16 字节无补白 / X,Y 单位 2 像素 / fcTL 的 dispose、blend 是单字节
```

样本清单（20 个文件，全部 ≤2.9 KB）：`gif_keep/restore_bg/restore_prev/once/transparent`、`apng_plays0/plays3/delden0`、`webp_anim_lossy/lossless` + `webp_static_lossy`、`exif_1..8.jpg` + `noexif.jpg`。

## 6 步骤 0 结论

批 1 解析器可以按 §1–§4 的字段表直接写，无未决项。唯一实现期需再实测的：**EXIF APP1 在 Pillow 9.0.1 写出的字节形态**（我的最小 IFD 是否被 stdlib/常规解析器接受）——留到写 EXIF 解析时先跑一条红钉确认，不凭印象。
