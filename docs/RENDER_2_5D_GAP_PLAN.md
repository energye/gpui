# 生产级 2.5D 游戏引擎能力差距表

日期：2026-09-14
范围：生产级 2.5D 游戏引擎（原题“动画背景”已扩为全引擎，见§0）。
结论先说：V1 算数已过（V1口径，§5待W10回炉），V2–V6 未开工，见§0做成算完。

## §0 冻结目标（全文档唯一目标口径，2026-09-17；目标数只在这里，门槛数只在§5）

- 范围：本线只做 2.5D 游戏引擎 `game/` 侧。V1 算对→V2 做像→V3 做大→V4 玩法机制→V5 量产工具→V6 联机多人。平台层（窗口、显卡后端、声音后端、打包签名、跨端重测）归别线，本线只定接口与数的口子（见“平台口子”一节）。玩法以 Godot 4 为唯一实现参考（G01–G35＋D01–D20），底层只用自家 `render/`，只抄做法不搬代码。网同步无 Godot 现成答案处按 MMO 通用做法写并注明不标 G 号。
- 做成算完：W0–W24 共 25 波全绿；S00–S112 共 113 号（S80、S101 作废，实做 111 项）每项走完五步，带日期＋测试文件＋真窗名；真窗每窗自动＋人工双 JSON（人工事件非 0），纯算数留离屏对比、音频加人工听；帧门、量门、像素门、长跑门、换机门、坏路门全见§5（阈值数只在§5出现一次，这里不重写）。
- 非目标：音频效果器除 S103 三样外其余以后；平台层实现与跨端重测归别线；框架只支持 linux/windows/macos 桌面三端，手机端不在本线范围；具体示例画法、美术资源、关卡设计；窗口、输入法、文字排版（别线管）。人和网原单独立项，现 V6 已补入本线（S106–S112，W23–W24）。
- 规矩：上一波没绿下一波别开；拿活按 W 块拿，P 只帮助记忆；能力行是唯一状态源；修订只记文末 log，行内不再散记。

## §0.1 重评估映射（每章结论＋缺口补项，2026-09-17；补项全已落 S 号）

| 章节 | 结论 | 说明 / 补项去处 |
|---|---|---|
| §1 三层目标 | 改 | 以§0为准，§1只留三层大白话 |
| §2 禁空壳 | 留 |  |
| §3 开工拿号 | 改 | 号段 W0–W24 / S00–S112，拿活按 W 块 |
| §4 卡模子 | 改 | S55 卡指针精确到“新人S55完整卡示例” |
| §5 门禁 | 留 | 全文档唯一门槛数（三数只在这里） |
| §6 参考 / §7 黑话 | 留 |  |
| 两成八成（1–9）、另一半（10–17） | 留 | V1 背景，不挂开工；开工看能力台账 |
| 参考引擎 | 留 | 背景；清单见 V2 的 G 表 |
| 包设计 / 底座 / 数据冻结 / 接口冻结 | 留 |  |
| 工业级表 / 依赖顺序 | 改 | 号段扩至 W24 / S112（S80、S101作废，实做111项） |
| 能力台账（V1 52 行） | 留 | 唯一状态源，过程套话已去 |
| 五步回归 | 留 |  |
| 验收壳 / 门禁挂波壳 / V2 指标总表壳 / 做成样壳 / 全不全壳 / 天花板通用门禁 | 并入§5 | 只留指针壳，不重写数 |
| 窗口清单 / 组合大窗 / 长跑接谁 | 改 | 只留结论，数见各波卡 |
| 不在做 | 改 | 以§0非目标为准 |
| V2 需求定稿 | 改 | 性能条见§5，量保留 |
| G 清单 / D 表 / S79 基向量验收 | 留 |  |
| V2 开发计划 W10–W15 | 改 | W15 只剩 S73；S80 作废并入 S87 |
| V2 真窗怎么做 | 改 | “同上帧门”定义见§5 |
| V2 波卡 / V3 波卡 / V4 波卡 / V5 波卡 | 改 | 卡内门槛三数指针化，只留本项单数 |
| V2 不含 N1–N8 | 改 | 去向更新：N1→S87，N2→S100，N3作废（手机端不在本线），N4→S84/S85，N5→S98，N6→S89＋S103，N7→S104，N8金属→S99、联机→V6 |
| V2–V6 能力行 / 门禁挂波 | 留 |  |
| V3 / V4 / V5 / V6 整节 | 留 | 做法卡已满 |
| V6 联机多人（S106–S112，W23–W24） | 补 | 2026-09-17 补：技能/8方向玩法层/视野同步/服存档社交/装备数值/同屏压测反作弊；画质不重复加（已在V2/V5） |
| 行内修订句 | 删 | 归拢文末 log，行内只留“见文末 log” |
| S80 占位条目 | 删 | 作废并入 S87，只留能力行作废行 |
| 修订 log | 留 | 唯一修订口径 |
| 缺口→补项：真 2.5D→S79；大世界→S84–S90；玩法→S91–S97；Q1–Q10→S98–S105；默认值→D01–D20 | 补完 | 见各节，不另开号 |

## 新人必读（先看这节再开工，2026-09-16）

一句话：我们要做的是能真正拿来做 2.5D 游戏的工业级引擎，不是画几个色块的演示。立体细腻只是其中一关。

### 1. 三层目标（做到哪算成）

- 第一层看着像东西：木箱像木箱、人像人、石头像石头，远看有前后，近看有鼓包，放大不糊，暗部不死黑。不到照片，但一眼能认。这关过了只算过了一关。
- 第二层能做完整游戏：第一阶段躲避小怪真能玩，人能动、怪会刷、撞了会死、分加得对、有音乐有音效、有开局有存盘，用的全是真图真音，不是色块占位。
- 第三层工业级引擎：Godot 那全套（镜头、精灵、动画、粒子、地图、骨骼、灯影、物理、音频、输入、资源、工具、性能）一个不少，开发者拿过去能直接开新游戏，不用等我们补洞。

### 2. 禁空壳（什么不许交）

- 不许只画红块绿块圆脸小人就说完。每个功能必须拿真资源验：人物拿真帧验，灯影拿真鼓包验，粒子拿真火烟验，地图拿真瓦片验，动画拿真骨头验。
- 做完和 Godot 原版并排放一起看，行为一样才算过。速度 400 就是 400，怪速 150–250 就是 150–250，手感不对就是没抄对。
- 数算对了但眼睛看着假，不算过。均匀灰无轮廓、暗部和背景一色、人眼只见一团亮晕，直接打回（S47 历史问题，不许重犯）。

### 3. 你怎么开工（拿号干活）

- W 是开工波（W0→W24），上一波没绿下一波别开。S 是单会话号（S00→S112），一次只做一个，新会话拿新号不干扰。同一波标可并行的可一起开，标串行的必须一个完再开下一个。
- 一个能力走完五步才许标已完成：1 接口先冻（doc.go 写清结构体函数错码，评审过）→ 2 单测（*_test.go 单跑过，数进 testdata）→ 3 接线（两边齐或记不适用，坏路不崩）→ 4 真窗（独立窗先自动 JSON 再人工 JSON，纯算数留离屏）→ 5 回归（本包全测加关联窗重跑，界面金图不红）。
- 状态有四种：未开工、进行中、已完成、冻结。能力行只填这四种（“已完成·V1口径”是已完成的标注写法，不是第五种：表示V1口径完成、§5待回炉，见读数规矩）。波关门另用“已绿”表示整波可进下波。写已完成必须带日期加测试文件加真窗名，口头说完不算。这张表是唯一状态源，做完改行，能力行加关门行加修订三处一次写齐。

### 4. 每个功能怎么做（一张卡 8 项，照新人S55完整卡抄）

新人S55完整卡示例在“W11 总指标”下一行（标题“新人S55完整卡示例”），其余S同模子，每节写满：
1 是干嘛的（一句话大白话）→ 2 照抄 Godot 哪个节点哪几个函数（G01–G35 有文件路径）→ 3 我们接口长什么样（结构体函数进出）→ 4 数据长什么样（文件放哪版本怎么升）→ 5 正常和坏数据各长什么样 → 6 性能要到多少（同屏多少还 60 帧）→ 7 拿什么真资源验（目标图长什么样）→ 8 怎么算过（自动窗看什么人工看什么）。

### §5 数到多少算过（全文档唯一门禁口径，任一条红整波红；别处不再重写，只写每波单数）

- 包：只认正式 release 包，debug 包数再好看也不认。
- 跑法：预热 5 秒扔掉，跑 3 遍，去头尾，取最差那遍判；每个 `*_test.go` 单独跑，一个跑完再跑下一个，长跑单独给足 `-timeout`；只贴最好那遍算假绿。
- 帧：目标 60 帧，看 p95 进 16ms、p99 进 20ms，不看平均；超 20ms 算一次卡顿，每分不超 3 次。
- 量：静合批数千、骨骼上百、CPU 粒子千内、GPU 粒子万级、碰撞百盒、实体千个为起步，帧率和调用次数一起记。大型档见“V3 大型化”一节的“规模对照”表，开工前先看那张。
- 存：长跑 2 小时涨不超 5%（V3 大包 8 小时），漏一次不过；切一次后台、缩一次窗口，回来黑屏直接红。
- GC（2026-09-17 定稿，Go 1.25 为准，W10 先跑基线，后面每项照着守，只许降不许涨）：
  - 目标：热路径少分配，不是零内存。卡顿多半是每帧不停 new，GC 突然扫一大片帧就掉了；管住分配，帧就稳。
  - 池子分两种：每帧用的走有界留存池（像 `game/step/pool.go`，上锁、有上限、GC 来了也在）；冷路径（解码、中转、加载）才走 `sync.Pool`（GC 来了可倒掉）。`Put` 前先清空引用，`Get` 出来先重写，取用还三动作齐，不许取了不还。
  - 值指针分冷热：小东西（坐标/颜色/帧号几十字节）传值不进堆；大东西（实体/地图块/骨骼几百字节以上）全程传指针，池里放指针、还的还是同一个。热结构做小做扁，里面不塞切片/哈希表/接口。冷路径（加载/存盘/报错）该返回 error 就返回；热路径（更新/排序/碰撞/粒子）不许接口装箱、不许循环里写闭包、不许 `Sprintf`/JSON。
  - 容器与字符串：热路径不用哈希表（能砍就砍，换切片加下标或整数编号；`map[float64]` 禁止）；字符串只认整数编号，不拼串、不转字节；切片提前按量申请、复用底子（切到 0 长再写），`make` 只许在初始化和扩容时出现。
  - 量法三件套：热点函数加分配断言（`testing.AllocsPerRun`，先预热池子、结果要用掉防优化）；长跑看 `runtime.MemStats` 的分配总量、GC 次数、最大停顿；再加 `-benchmem` 基准。只看次数不看停顿不算过。
  - 堆外另算：Go 的 GC 只管 Go 堆，贴图/显卡缓冲/中转映射走 S88 显存预算加清理链，不进 GC 门。内存上限按高/中/低三档定死，W10 基线时一起冻。
- 像：显卡和 CPU 差只许抗锯齿边缘，像素差超 1% 打回；梯形变方块、渐变变纯色、暗部和背景糊成一色，直接打回；小金不能顶大窗。
- 环境：写清显卡、驱动、分辨率、开不开垂直同步、是不是 release、有没有插电、烫没烫；换一台机器照样过，只在一台上过不算（换机=集显独显各一遍，或 X11 换 Wayland，二选一必做一款）。
- 图：对比图全进 `testdata/`，测试只读文件，不许现编标准数据；容差写死在测试里（静图 0 容差）；改基准必须说清谁改的、为啥改、记哪天（能力行尾＋testdata 日期文件，双处留名）；基准一花先查代码，不许直接更新图装绿；首遍重冻基线后必须第二遍 0 差，单遍 0 差不算过。
- 坏路：缺图坏档坏骨头坏地图占位加报错，不崩不卡死；显存不够走清理链记日志，OOM 降档必须单窗正式包重测替代，不替代不算过；坏着色器只黑游戏层，不崩主路；切后台窗口最小驱动掉线回来能画；静默降级算假绿。
- 窗：主能力必须独立真窗，先自动 JSON 再人工 JSON（人工必须有点按/按键事件，事件 0 算空转不算过）；纯算数留离屏对比，音频加人工听；跑完即关的一次性验收窗不能代替常驻人工窗；逻辑探针＋像素断言＋Golden 三证据缺一不可。
- 文档：能力行＋关门行＋修订三处一次写齐；口头说完不算，门禁绿不等于洞修完。
- 帧门简称（本节即§5，各波卡写“帧门见§5”即指本条，不重写数）：正式包＋presents400＋＋单窗fps57＋＋帧条三数见本节（数不重写）＋双金0差；S78 presents6000＋、S90 presents34000＋两处单数见各自卡，其余门槛同本条。

### 6. 参考去哪找（只抄做法，不搬代码）

- 引擎本地：`/home/yanghy/app/projects/gogpu/godot`（只读，不编译不改）。示例本地：`/home/yanghy/app/projects/gogpu/godot-demo-projects`。底层渲染只用自家 `render/`，Go 重写接到现有接口，不搬 C++。
- 自家 `render/` 缺功能就补：“不搬代码”指的是不搬 Godot 的 C++，不是不许动自家库。顺序是先改 render（正向优化/新增/修改，老样子：新分支新文件、老路不动、两边齐、坏路不崩）；render 实在做不到的，才调 `gpu/` 公开接口。两条路都在当前分支上做，不另开分支，不直碰驱动。
- 玩法以 Godot 4 为唯一实现参考，参数名默认值对齐，坏了怎么报错跟它对齐。全量清单 G01–G35 在 V2 章，含文件路径和抄哪几个函数。Skia 只管画布思想，Cocos 只做第二意见。
- 第一阶段照抄 `godot-demo-projects/2d/dodge_the_creeps`，真图真音用原版那套（人物走跑各两帧、三怪各两帧、音乐死亡音各一），CC0 可直接用。真窗叫 `examples/game_dodge`，自动 120 秒加人工 2 分钟。

### 7. 黑话翻译（新人先认这几个）

- 合批：很多小图一次交给显卡，省次数。
- 视差：远慢近快，镜头动时远山动得少。
- 法线：管鼓包方向的图，有它灯才知道哪鼓。
- 柔边：影子边软一点，不硬切。
- 各向异性：斜着看地面不糊的开关。
- 回炉：已绿的数太虚，正式包重测一遍叫回炉。

这份文档只看源码，不看文档和注释吹什么。

## 现在有的两成（能用，但只够跑小演示）

| 有的东西 | 在源码哪里 | 够用到哪 |
|---|---|---|
| 画矩形圆圈线条路径 | `render/shapes.go`、`render/path*.go`、`ui/rendering/draw.go` | 画卡通小场景够了 |
| 显示文字 | `render/text.go`、`render/internal/gpu/*glyph*` | 界面字够了 |
| 贴一张图、按块取图、九宫格拉伸 | `render/context_image.go`、`render/vertices.go`、`render/nine_patch.go` | 单张背景、按钮拉伸够了 |
| 简单透明叠加 | `render/internal/image/draw.go`、`render/blendmode.go` | 普通叠放够了 |
| 简单模糊阴影颜色矩阵灰度反色共 6 种 | `render/filter_ops.go` | 简单特效够了 |
| 三角渐变（显卡真渐变，另有索引网格） | `render/vertices.go`（`DrawVertices/DrawMesh`）、`render/internal/gpu/*colored*` | 显卡上有就行 |
| 定时喊一声的 Tick | `ui/scheduler/ticker.go` | 简单动起来够了 |
| 垂直同步跟随、双缓冲、最新帧顶旧帧 | `ui/scheduler/scheduler.go`、`ui/raster/loop.go` | 帧率不崩够了 |
| 图片复用池、释放、显存不够清理 | `render/internal/image/pool.go`、`render/internal/image/buf.go`、`render/oom_purge.go` | 短时间不爆够了 |
| 留存静态块（纯色块文字图片） | `ui/rendering/boundary_cache.go`、`ui/scene/picture.go` | 静止界面省帧够了 |

## 还差的八成（9 大块）

### 缺口1. 镜头和远近

近大远小、镜头推拉跟随震动、远近景错开动，游戏叫相机和视差。

- 1.1 上层透视投影（已完成）：老矩阵还是 6 个数，只能平移缩放旋转错切，算不出透视除法（位置：`render/matrix.go`、`render/internal/image/affine.go`），底层不动；游戏侧在 `game/camera/project.go` 另算好屏坐标和深浅，再喂现有梯形三角，CPU 和显卡两条路算出同一个数。
- 1.2 前后遮挡：游戏要的深浅值排序接口现在没有，画出来只看谁后画谁盖谁。源码里显卡确实用了深度比较，但只给任意路径裁剪用（`render/internal/gpu/shaders/depth_clip.wgsl`、`stencil_renderer.go`、`glyph_mask_pipeline.go`、`convex_renderer.go`里的 `DepthCompare=GreaterEqual`），不对游戏开放。要做：加深度值，远的先画近的后画，显卡打开深度比较。
- 1.3 镜头能力：推拉、跟随、震动、变焦、远近景错开动，外加限位、平滑跟随、锚点、无限重复衔接。现在场景层只有平移加旋转加缩放（`ui/scene/layer.go` 的 `TransformLayer`：`TX,TY,Rotation,SX,SY,CX,CY`，另有 `OffsetLayer`），不是相机，只能每个示例自己手算。要做：引擎里给一套镜头，位置、缩放、角度、震动、限位、平滑、锚点一次说清；视差层支持重复镜像，长路不露缝。
- 1.4 梯形贴图对齐：`render/m4_extensions.go` 的 `DrawImageQuad`，显卡能画梯形，CPU 直接拉成方块。要做：CPU 补真透视采样，或者明确不支持时报错，不要静默画错。

### 缺口2. 小图一次多画

很多树人石头一次交给显卡，游戏叫精灵合批和图集。

- 2.1 精灵合批：入口现在是逐个排队（`render/vertices.go` 的 `DrawAtlas` 对每个块调一次 `QueueImageDraw`），会话层只合并连续同图同透明度同过滤同视口的块（`render/internal/gpu/image_pipeline.go` 的 `canMergeImageDraw` 共用绑定组一次画多个四边形，另有 `canMergeGPUTextureDraw`）。跨图合批、旋转染色、自动排序都没有。要做：同一张大图的一批小块，一次提交，少几次调用。
- 2.2 旋转和染色：现在 `AtlasSprite` 里只有位置和透明度，转不了换不了颜色。要做：加旋转角度、缩放中心、整体染色，外加翻转、轴心、单图过滤（近的清、远的糊各管各）。
- 2.3 前后排序：按高低自动决定谁盖谁，外加层号、世界和界面分层（特效别飘到界面上）。现在没有，只有渲染层内部给图层排号（`render/render/layers.go` 的 `zOrder`，管图层不管精灵），游戏精灵全靠手写顺序。要做：按世界高度排序再画，层号大的盖住小的。
- 2.4 序列帧播放器：几张图轮着放形成动作，外加多套动作库（待机跑跳各一套）。现在没有，得手写计时切换。要做：帧号、时长、循环 ping-pong、事件回调，多动作切换。

### 缺口3. 大图和显存

大图片怎么存、远了糊不糊、加载卡不卡。

- 3.1 游戏压缩图：ASTC、ETC、Basis、KTX。读图入口现在只有 PNG、JPG、WebP（`render/internal/image/io.go` 的 `LoadPNG/LoadJPEG/LoadWebP/LoadImage/Decode`），图片格式也只有 8 位系（`render/internal/image/format.go`：`Gray8/Gray16/RGB8/RGBA8` 系，无浮点无压缩块格式）。显卡格式枚举里虽有 BC1-7、ETC2、ASTC 常量（`gpu/types/texture.go`），但没接解码上传链路。要做：至少支持一种显卡直接读的压缩格式。
- 3.2 远处防闪：CPU 有多级缩小链 `render/internal/image/mipmap.go`，显卡贴图主采样器是双线性放大加双线性缩小加最近多级（`render/internal/gpu/image_pipeline.go`：`MagFilter=Linear,MinFilter=Linear,MipmapFilter=Nearest`，没写各向异性即默认 1），其它管线多处写死 `Anisotropy:1`，只有文字管线给了 4（`render/internal/gpu/text_pipeline.go`）。要做：显卡补多级渐远，各向异性可调，CPU 和显卡缩出来一致。
- 3.3 后台边玩边加载：现在读图是同步卡住读。要做：异步解码、流式进显存、大地图不卡顿。

### 缺口4. 让东西动起来

动作自然不僵硬那套，游戏叫动画系统。

- 4.1 变速曲线：快慢变化。现在只有 `Tick(dt)`，时间大力还会被钳住，见 `ui/scheduler/ticker.go`。要做：常用缓动曲线库。
- 4.2 关键帧时间线：按时间轴摆动作，外加事件軌（到第几帧出刀光、播声音、调方法、显隐）。现在没有。注意 `ui/kit/timeline` 是界面时间线组件，不是动画时间线，别被名字误判为已有。要做：时间轴、关键帧、插值、循环、事件回调。
- 4.3 骨骼蒙皮：人物走跑跳换动作，对齐 Spine 结构：骨头、插槽、皮肤、网格附件、权重、绘制次序、IK和约束、物理惯性。现在源码搜骨骼、Spine、IK 全是空，搜到带骨架字样的只是文字描边测试（`hbSkeleton`、`Skeleton` 指字形骨架）。要做：骨骼、蒙皮权重、反向带动、动作混合、状态机，保证真美术资源能直接用。

### 缺口5. 火烟水气和后期

氛围感那套，游戏叫粒子和全屏后期。

- 5.1 粒子发射器：火烟雨雪落叶，外加发射形状（点锥盒环）、乱流、子发射器。现在没有。要做：发射数量、速度、寿命、重力、颜色变化、形状、乱流、子发射。
- 5.2 显卡粒子和拖尾：几千个小点一起动不卡，拖尾带宽窄变化、颜色渐变、拐角接头。现在没有。要做：显卡算位置，拖尾按轨迹画，宽窄颜色跟走。
- 5.3 扭曲热浪：水面晃、空气抖。现在没有。要做：按噪声偏移采样。
- 5.4 发光暗角调色：现在 `render/filter_ops.go` 只有模糊、阴影、颜色矩阵等 6 种，搜发光、暗角、调色、色调映射都是空。要做：发光、暗角、颜色查找表、色调映射，外加自定义材质钩子（美术要特殊描边溶解能接进来）。
- 5.5 自定义材质钩子（新补）：特效不够时美术自己写一段小着色器。现在没有。要做：统一钩子，只碰游戏层，不碰界面主路。

### 缺口6. 灯光

白天黑夜、手电光影。

- 6.1 2D 灯光：哪里亮哪里暗，外加灯片（手电形状）、强度颜色、只照哪层、全局夜色。现在没有。要做：点光、方向光、范围衰减、灯片、强度颜色、分层照、分层不照、全局调制。
- 6.2 法线受光：平面图有立体感。现在没有。要做：法线贴图参与受光计算。
- 6.3 投影和环境遮挡：影子、角落变暗。现在没有。要做：投影、遮挡系数。

### 缺口7. 大地图

大世界怎么装下，游戏叫瓦片和分区。

- 7.1 瓦片地图和斜 45 度地图：含 TMX 解析，外加对象层（摆怪摆箱）、碰撞导航遮挡层、自动拼（路沿自己接上）、无线大图分区、预加载半径。要做：瓦片装载、斜 45 度投影、对象层、自动拼、碰撞层。
- 7.2 分区加载和视野剔除：只加载只画镜头里那块。现在没有。要做：按块装卸，镜头外不画。
- 7.3 远近切换：远了用简单版，近了用精细版。现在没有。要做：按距离换细节等级。

### 缺口8. 帧节奏和内存

长时间跑不崩不慢。

- 8.1 固定步长加补间：快慢机器动作一致。现在时间步长不固定。要做：逻辑固定步长，渲染按余量插值。
- 8.2 路径顶点复用池：路径对象现在靠示例自己 `Reset`（`render/path.go` 有 `NewPath/Reset/Clone`，无对象池），顶点侧上下文有草稿复用（`render/vertices.go`、`render/context.go` 的 `vertDevScratch` 等），不是完整的池化。要做：路径、顶点缓冲复用。
- 8.3 动态世界局部更新：`ui/scene/picture.go` 的留存只认矩形路径文字图片，`DrawAtlas`、`DrawVertices`、`DrawMesh` 进不了留存；`ui/rendering/boundary_cache.go` 只会存静的；脏区超 16 块就合并，见 `render/context.go`。会动的东西现在每帧全重画。要做：动态层脏区细分，精灵层独立更新。

### 缺口9. 颜色和两边一致

换机器画面不变。

- 9.1 高动态颜色和亮度流程：主链路还是 8 位。图片格式只有 8 位系，主渲染目标是 `RGBA8Unorm/BGRA8Unorm`（`render/render/target.go`、`layers.go`），`gpu/types` 里虽有 `R16Float/RGBA16Float` 等常量但主链路没用。要做：浮点目标、正经亮度流程、色调映射。
- 9.2 CPU 回退画质：`render/vertices.go` 的渐变在 CPU 是取平均填纯色，图集在 CPU 是逐个画。要做：CPU 和显卡像素对齐，或者回退时明确降级标记。

## 参考引擎（Godot 4 是唯一的玩法实现参考目标，MIT 可商用只抄做法不搬代码）

本地参考（2026-09-16 已 clone 到 gogpu 目录，不提交进 gpui 仓库）：
- 引擎：`/home/yanghy/app/projects/gogpu/godot`（MIT，`dfa06ca`，只读参考，不编译不改）
- 示例：`/home/yanghy/app/projects/gogpu/godot-demo-projects`（MIT，`a3b5c11`，小游戏照抄对象）
- 关系定死：底层渲染引擎只用自家 `render/`（Go 重写接到现有接口），玩法层（镜头、精灵、动画、粒子、地图、灯影、步长）以 Godot 4 节点为唯一实现参考目标；Skia 只管画布思想（矩阵、合批、滤镜链、采样），Cocos 只做第二意见防走偏。

第一阶段目标（P1 小游戏，能完整玩起来）：
- 照抄 `godot-demo-projects/2d/dodge_the_creeps`（躲避小怪）：玩家 400 像素/秒八向走，AnimatedSprite2D 按方向切 right/up 动画，怪从 Path2D 随机点按垂直加减 45 度随机方向、150–250 速度刷出来，撞上 hit 进 game_over，ScoreTimer 每秒加 1 分，HUD 显示 Get Ready/加分/结束，Music/DeathSound 两路音。
- 用到我们哪块：input 动作（上下左右）、sprite 序列帧（待机跑两套）、physics 碰撞（Area2D 碰即死）、audio 两路（音乐＋死亡音）、world 开局（StartPosition）、save 最高分存盘。
- 验收：和 Godot 原版并排玩 2 分钟，操作手感一致（速度 400 分毫不差），怪速 150–250 随机分布一致，撞了必死、死了有音、分加得对；真窗 `examples/game_dodge` 自动 120 秒 presents6000＋（hitch见§5）＋人工玩 2 分钟收 JSON。
- 进阶对照（P2 排期用，不在本阶段做）：platformer（平台跳＋敌人＋音乐）、role_playing_game（格子移动＋战斗对话）、lights_and_shadows（灯影 night＋cookie）、kinematic_character（斜坡平台）。

| 参考谁 | 用来定什么 | 怎么用 |
|---|---|---|
| Godot 4（Camera2D、ParallaxBackground、Sprite2D、AnimatedSprite2D、AnimationPlayer、AnimationTree、CPUParticles2D、GPUParticles2D、Line2D、TileMap、TileSet、Skeleton2D、PointLight2D、LightOccluder2D、CanvasModulate） | 镜头、视差、前后排序、序列帧、变速曲线、时间轴、状态机、粒子、拖尾、瓦片、骨头、2D 灯光影子、固定步长 | 唯一玩法参考。只看节点有哪些属性、数怎么算，Go 重写接到 game/ 包，不搬 C++ 代码，参数名默认值对齐 |
| Skia（含 SkCanvas、SkParticles、SkImageFilter） | 透视矩阵思想、贴图多级渐远加各向异性、合批思路、滤镜链、粒子模块思想 | 只看接口和算法，Go 重写一遍接到现有 `render/` 接口上 |
| Cocos Creator（含 Sprite、Animation、TiledMap、Spine 挂接） | 同一份玩法的第二意见，防只看一家走偏 | Godot 看不懂的地方拿它对一遍 |
| Spine 数据格式（含骨骼、插槽、蒙皮、IK、权重） | 骨头数据长啥样 | 只认 JSON 结构，保证以后美术资源能直接用 |
| Tiled TMX 数据格式（含图块、图层、对象） | 瓦片地图数据长啥样 | 只认 TMX 结构，保证地图资源能直接用 |
| Basis Universal、KTX2、ASTC 公开格式（含 Khronos 采样器规范） | 压缩图和远处防闪 | 只看头和块怎么摆，解码找现成 Go 库包一层 |
| Fix Your Timestep（Gaffer 经典做法） | 固定步长加渲染插值 | 逻辑固定步长跑，画画按余量插值，快慢机器动作一致 |

## 还差的另一半：工业级支撑（8 块）

画画那 9 块只是半张表。工业级是拿来做项目、养美术、长期跑的，还差下面 8 块。现状一句话：界面那套底子有，游戏那套活没有。`ui/animation` 只有界面动画的控制器和曲线，`ui/input` 加手势加焦点是给按钮输入框用的，`ui/scheduler` 加 `ui/raster` 是按需画界面的，不是游戏固定步长的。音频、碰撞、寻路、存档、脚本这些包名全搜不到，就是没有。

### 缺口10. 世界和对象

游戏里的关卡、人、怪、箱子怎么放、谁是谁的爹、开局读档怎么一次装出来，游戏叫实体和场景。

- 10.1 实体挂件：一个东西挂几个能力（位置、贴图、碰撞、脚本）。现在界面那棵 `RenderObject` 树只能画静的，不管游戏命。要做：实体号、挂件增删、父子变换继承。
- 10.2 预制和场景：策划摆好的东西存成文件，开局一次装出来。现在没有。要做：预制文件、场景文件、加载存盘。
- 10.3 生命周期：出生、激活、休眠、销毁，跨关不漏不炸。现在没有。要做：统一开关，销毁清资源。

### 缺口11. 主循环和时间（`game/step` 的另一半）

逻辑跑多快、卡了追不追、暂停慢放加速怎么做。

- 11.1 固定步长：快慢机器动作一致。画画那块 8.1 只管插值，这里管逻辑节拍。要做：逻辑按固定步长跑，画画按余量插值（主抄 Fix Your Timestep）。
- 11.2 时间控制：暂停、慢放、加速、时缩放。现在 `Tick(dt)` 没有这套。要做：全局时间倍率，分层时间（世界停了界面还能动）。

### 缺口12. 资源管线

美术图、地图、骨头、声音怎么进来、压缩、拼大图、边玩边下、用完谁扔，游戏叫资产管线。

- 12.1 资产管理：异步加载、引用计数、用完释放、预算淘汰，外加依赖跟踪（骨头换图、地图换块知道重载谁）、导入管线、版本哈希。现在只有单张读 PNG、JPG、WebP，有图片池和显存清理，没有整套账。要做：统一资产号，后台加载，前台不卡，依赖对得上。
- 12.2 图集打包：小图拼大图，省提交次数。现在只有运行时按块取，没有打包工具。要做：离线打包工具，加旋转留白。
- 12.3 热重载：美术改完不用重启游戏直接看。现在没有。要做：文件一变就重载，只换那一块。

### 缺口13. 输入映射

手柄摇杆、多点触摸、按键改键、连招缓存、录像回放，游戏叫动作映射。

- 13.1 动作映射：跳跃、攻击这种动作，不绑死某个键，外加死区、手柄震动、捏放手势。现在界面输入只认点和键，不认动作。要做：动作名对多键多柄多触摸，可改键，死区可调。
- 13.2 输入缓冲：连招按快了不丢，多点触摸不打架。现在没有。要做：按压队列，触摸号跟踪。

### 缺口14. 碰撞物理

地面斜坡、跳和落、谁撞谁、触发开门。2.5D 用 2D 碰撞就够，不用上大物理引擎。

- 14.1 碰撞盒：谁和谁撞、撞完弹开还是穿过，外加射线检测（跳跃探头、子弹判线）、移动平台。现在完全没有。要做：盒和圆，层和分组，触发器，射线，移动平台带着走。
- 14.2 平台和斜坡：站得住、滑不滑、跳得上。现在没有。要做：斜坡站立，平台单向上跳。

### 缺口15. 音频

脚步远近大小、音乐切换、混音不爆。

- 15.1 2D 位置音：离得远声音小，左右有别。现在没有。要做：位置、范围、衰减。
- 15.2 音乐和混音：切换淡入淡出，多声不炸，外加总线、闪避（爆炸压音乐）。现在没有。要做：音乐栈，混音上限，总线闪避。

### 缺口16. 存档和设置

存盘读盘、画质档、语言。

- 16.1 存档：进度、背包、关卡星，外加槽位、版本迁移。现在没有。要做：存档文件，版本号，坏档不崩，升版能迁。
- 16.2 画质分档：高低端机器各跑各的。现在没有。要做：高、中、低三档，粒子、灯光、分辨率跟档走。

### 缺口17. 工具和看病

美术导表卡了谁看、掉帧是谁的锅、线上崩了怎么查。

- 17.1 编辑器预览：摆关卡、调粒子、看瓦片。现在没有。要做：`tools/` 下先给关卡预览和粒子预览。
- 17.2 性能和崩溃：帧率、画次数、显存、长跑不涨、崩了留日志，外加帧分解（谁吃时间）、过绘、着色器编译耗时。现在只有界面那套指标和真窗，没有游戏这套。要做：`game/debug` 统一出数，接长跑和崩溃上报。

看情况再加（不急）：联机同步回滚、策划脚本（Go 加 Lua 二选一，带热重载）。要联机、要改逻辑不编译，这两样才做。

## 包设计（新能力全放 `game/`，跟 `render/`、`ui/` 平级）

原则：`game/` 只算数，算完调现有画图接口。`render/` 只修画错的地方，不放大改。

```text
game/camera/camera.go      镜头位置缩放角度震动限位平滑锚点（对 1.3，主抄 Godot Camera2D）
game/camera/project.go     上层透视投影 helper（对 1.1，老 6 数矩阵不动，另算好再喂梯形三角）
game/camera/parallax.go    远近景错开加重复镜像（对 1.3，主抄 Godot ParallaxBackground）
game/sprite/batch.go       同图合批一次提交（对 2.1，主抄 Godot Sprite 配 Skia 合批思路）
game/sprite/atlas.go       图集扩展旋转染色翻转轴心单图过滤（对 2.2，主抄 Skia drawAtlas 加旋转）
game/sprite/ysort.go       按脚底高度排序加层号分层（对 2.3，主抄 Godot YSort）
game/sprite/flipbook.go    序列帧多动作库帧号时长循环回调（对 2.4，主抄 Godot AnimatedSprite2D）
game/tex/compressed.go     压缩图加载（对 3.1，主抄 Basis、KTX2、ASTC 格式）
game/tex/stream.go         后台边玩边加载（对 3.3，主抄 Godot 后台加载）
game/tex/mipmap.go         远处防闪开关（对 3.2，主抄 Skia 采样做法）
game/anim/easing.go        变速曲线（对 4.1，主抄 Godot 缓动曲线）
game/anim/timeline.go      关键帧时间轴加事件軌（对 4.2，主抄 Godot AnimationPlayer；注意 ui/kit/timeline 是界面组件，不是这个）
game/anim/skeleton.go      骨骼插槽皮肤网格权重IK约束物理惯性（对 4.3，主抄 Spine 数据格式加 Godot Skeleton2D）
game/anim/statemachine.go  动作混合状态机（对 4.4，主抄 Godot AnimationTree）
game/particle/emitter.go   发射数量速度寿命重力颜色形状乱流子发射（对 5.1，主抄 Godot 粒子节点）
game/particle/cpu.go       普通粒子（对 5.1，同上）
game/particle/gpu.go       显卡粒子（对 5.2，主抄 Godot GPUParticles2D）
game/particle/trail.go     拖尾轨迹宽窄渐变接头（对 5.2，主抄 Godot Line2D）
game/fx/bloom.go           发光（对 5.4，主抄 Skia 滤镜链）
game/fx/vignette.go        暗角（对 5.4，同上）
game/fx/lut.go             调色颜色查找表（对 5.4，同上）
game/fx/warp.go            扭曲热浪噪声偏移（对 5.3，主抄游戏引擎扭曲做法）
game/fx/custom.go          自定义材质钩子（对 5.5，主抄 Godot CanvasItemMaterial）
game/light/light.go        点光方向光范围衰减灯片强度分层全局夜色（对 6.1，主抄 Godot PointLight2D）
game/light/normal.go       法线受光（对 6.2，主抄游戏引擎法线做法）
game/light/shadow.go       投影遮挡（对 6.3，主抄 Godot LightOccluder2D）
game/tilemap/tilemap.go    瓦片对象层碰撞导航自动拼（对 7.1，主抄 Tiled TMX 加 Godot TileMap）
game/tilemap/iso.go        斜 45 度投影（对 7.1，同上）
game/tilemap/chunk.go      分区加载视野剔除（对 7.2，主抄游戏引擎分区做法）
game/tilemap/lod.go        远近切换（对 7.3，同上）
game/step/fixed.go         固定步长加补间（对 8.1 和 11.1，主抄 Fix Your Timestep）
game/step/pool.go          路径顶点复用（对 8.2，不抄谁，纯工程池化）
game/step/time.go          时间倍率暂停慢放（对 11.2，主抄 Godot 时间缩放）
game/world/entity.go       实体挂件父子变换（对 10.1，主抄 Godot 节点做法）
game/world/prefab.go       预制加载（对 10.2，主抄 Godot 预制做法）
game/world/scene.go        场景加载存盘生命周期（对 10.2 和 10.3，同上）
game/asset/asset.go        资产号异步加载引用计数依赖版本（对 12.1，主抄 Godot 后台加载）
game/asset/atlaspack.go    图集离线打包（对 12.2，主抄游戏引擎打包做法）
game/asset/hotreload.go    文件一变就重载（对 12.3，同上）
game/input/action.go       动作映射改键死区震动手势（对 13.1，主抄 Godot 输入映射）
game/input/buffer.go       输入缓冲多点跟踪（对 13.2，主抄游戏引擎做法）
game/physics/body.go       碰撞盒层分组触发器射线移动平台（对 14.1，主抄 Godot 碰撞做法）
game/physics/platform.go   斜坡平台单向上跳（对 14.2，同上）
game/audio/positional.go   2D 位置音范围衰减（对 15.1，主抄 Godot 位置音）
game/audio/music.go        音乐栈混音上限总线闪避（对 15.2，同上）
game/save/save.go          存档槽位版本迁移坏档不崩（对 16.1，主抄游戏引擎存档做法）
game/save/quality.go       高中低画质分档（对 16.2，同上）
game/debug/stats.go        帧率画次数显存帧分解长跑（对 17.2，主抄游戏引擎看病做法）
tools/levelprev/main.go    关卡预览（对 17.1，先顶一个能看的）
tools/particleprev/main.go 粒子预览（对 17.1，同上）
```

`render/` 里只小修，不放大改：

```text
render/m4_extensions.go    修梯形 CPU 回退真透视采样（对 1.4，即 R1）
render/vertices.go         修 CPU 渐变真渐变（对 9.2，即 R2）
render/internal/gpu/image_pipeline.go  加按图采样开关（对 3.2 的 render 底，即 R3）
render 新分支 AtlasSprite 加 rot/flip/pivot/tint（对 2.2 的 render 底，即 R4，老路不动）
render 新分支 ysort 深度比较（对 1.2 的 render 底，即 R5，隔离新分支）
```

暂缓不动（动了全崩，先放着）：

```text
6 数矩阵改透视（对 1.1 底层）
全链路浮点（对 9.1）
```

实施顺序：以“开发依赖顺序”一节的 W0–W24 为准（W 是开工波次，S 是单会话序号，一次只做一个按 S00→S112 拿号）。

## 开工底座 game/core（先立起来，谁都靠它）

一句话：不先立底座，17个包各写一套数，后面合不上。P0开工前先把这个包立住。

```text
game/core/vec.go      Vec2、Rect、Mat2D（只管2D数，不管渲染矩阵）
game/core/color.go    Color（浮点存、8位出，和render对得上）
game/core/time.go     Duration、Step（毫秒存，秒浮点算，防漂）
game/core/rand.go     可播种随机（测试能重放，粒子不玄学）
game/core/asset.go    AssetID、Handle（字符串ID加引用计数，资源对得上）
game/core/result.go   统一错（缺文件、坏数据、显存不够，分得清）
game/core/version.go  数据版本号（存档地图骨头升版能迁）
```

铁规矩：game下所有包只许用core的数，不许自己再定一套Vec2、Color、AssetID。render里老类型不动，game调render时在边界转一次。

## 数据格式冻结（真资源能接，不冻美术白做）

| 数据 | 认谁 | 冻到哪 | 文件放哪 |
|---|---|---|---|
| 骨头 | Spine JSON子集 | 骨头、插槽、皮肤、网格附件、权重、绘制次序、IK和约束先冻，物理惯性后冻 | `game/anim/testdata/spine_*.json` |
| 地图 | Tiled TMX子集 | 图块、图层、对象层、碰撞导航遮挡先冻，自动拼规则后冻 | `game/tilemap/testdata/*.tmx` |
| 图集 | 自定JSON | 图名、大图XY宽高、轴心、九宫格先冻 | `game/asset/testdata/atlas_*.json` |
| 预制场景 | 自定JSON | 实体、挂件、父子、引用资产号先冻 | `game/world/testdata/scene_*.json` |
| 存档 | 自定JSON | 槽位、版本、升版迁移先冻 | `game/save/testdata/save_*.json` |
| 压缩图 | KTX2/Basis头 | 先只冻一种，后加一种冻一种 | `game/tex/testdata/*.ktx2` |

没冻的格式，代码不许写死解析，先报错占位。

## 接口冻结规则（并行不分叉）

- 每个能力开工第一步先冻接口：结构体长啥样、函数进啥出啥、错怎么报，写进对应`doc.go`，评审过才写逻辑。
- 接口只加不改：要改先升版，老调用还能编过。render老接口一个不许改，新分支走新函数。
- game只调render现有公开接口：要新画法先走`game/fx/custom.go`钩子，不直接改render主路。

## 一整张工业级表（9 块画画加 8 块支撑）

| 块 | 管啥 | 包 | 现状 |
|---|---|---|---|
| 1 镜头远近 | 近大远小推拉震动视差 | `game/camera` | 1.1已完成，其余未做 |
| 2 小图多画 | 合批旋转染色排序序列帧 | `game/sprite` | 只有单张和按块取，要新做 |
| 3 大图显存 | 压缩后台加载防闪 | `game/tex` | 只有三种格式同步读，要新做 |
| 4 动作 | 变速时间轴骨头状态机 | `game/anim` | 只有一声 Tick，要新做 |
| 5 粒子后期 | 火烟水气发光暗角调色扭曲加材质钩子 | `game/particle`、`game/fx` | 只有 6 种滤镜，要新做 |
| 6 灯光 | 亮暗法线影子 | `game/light` | 没有，要新做 |
| 7 大地图 | 瓦片斜 45 度分区远近切换 | `game/tilemap` | 没有，要新做 |
| 8 帧内存 | 步长复用局部更新 | `game/step` | 只有跟随双缓冲复用池清理，要补 |
| 9 颜色一致 | 高动态两边对齐 | `render/` 小修 | 主路 8 位分叉，要小修 |
| 10 世界对象 | 实体预制场景生命周期 | `game/world` | 没有，要新做 |
| 11 时间 | 步长时缩放暂停 | `game/step` | 没有，要新做 |
| 12 资源 | 资产打包热重载 | `game/asset` | 没有，要新做 |
| 13 输入 | 动作改键缓冲 | `game/input` | 只有界面点键，要新做 |
| 14 碰撞 | 碰撞盒平台斜坡 | `game/physics` | 没有，要新做 |
| 15 音频 | 位置音音乐混音 | `game/audio` | 没有，要新做 |
| 16 存档设置 | 存档画质档 | `game/save` | 没有，要新做 |
| 17 工具看病 | 预览性能崩溃 | `game/debug`、`tools/` | 只有界面指标真窗，要新做 |

## 开发依赖顺序（先做啥、谁卡谁、谁能并行）

看法一句话：上一波没绿，下一波别开。P 是记忆分类（只帮助记忆，不挂开工不挂门禁，能力表里看到 P 只当身份证看）；W 是开工波次（W0→W24，谁卡谁，开工拿活只看 W 块）；S 是单会话序号（一次只做一个，按 S00→S112 拿号，新会话不被干扰）。同一波里标【可并行】的可以一起开也可以一个一个来，标【串行】的必须一个做完再做下一个。标【波内部分串行】的按写明的先后做，其余可并行。W11 的 S 号不连续（S55–S61＋S74–S79），拿活按 W11 块拿，别按 S 号顺序拿。S78 躲避小怪的前置写死等 W11 关门（含 S79 在内全绿），故归 W13。拿活按 W 块拿。 V1的S00–S51分到G01–G11，详情见 `docs/2.5D/G01.md`、`docs/2.5D/G02.md`、`docs/2.5D/G03.md`、`docs/2.5D/G04.md`、`docs/2.5D/G05.md`、`docs/2.5D/G06.md`、`docs/2.5D/G07.md`、`docs/2.5D/G08.md`、`docs/2.5D/G09.md`、`docs/2.5D/G10.md`、`docs/2.5D/G11.md`（原话已搬家，状态以本文件能力行为准）。

R 对号：R1=1.4 本体，R2=9.2 本体，R3 是 3.2 的 render 底，R4 是 2.2 的 render 底，R5 是 1.2 的 render 底；R 不占能力行，状态落在对应能力行。8.1/11.1 同一份文件合测，开工一次、状态两行同时改。

（09-15 两次排期修订已并入文末修订 log：52 行按前置重排＋S41 从 W6 前移 W5。）

```text
W0 已绿复核（S00，不写代码，只复跑单测）
  S00: 0.0 core ＋ 1.1上层透视投影【已完成】

W1 只靠底座【已绿；波内可并行；单会话一次做一个，按S01→S15顺序拿号】
  S01 1.4/R1梯形CPU（后5连串的头，最先做），S02 3.1压缩图认头，S03 8.1+11.1固定步长合测（开工一次、两行同时改），
  S04 4.1变速，S05 7.1瓦片斜45度，S06 14.1a碰撞体纯算，S07 13.1动作映射，
  S08 1.3a镜头纯算，S09 1.3b视差画出，S10 2.3排序，S11 2.4序列帧，S12 8.2复用池，
  S13 15.1位置音，S14 15.2音乐混音，S15 16.1存档

W2 靠W1【已绿；波内可并行（跨波前置R2等W1的R1），按S16→S23顺序拿号】
  S16 R2/9.2 CPU渐变【串行：等S01】，S17 4.2时间轴（等S04），S18 11.2时间倍率（等S03），S19 12.1资产管理（等S02），
  S20 7.2分区（等S05+S08视口），S21 14.1b射线移动平台（等S06+S05），S22 14.2平台斜坡（等S06+S05），S23 13.2输入缓冲（等S07）

W3 靠W2【已绿；波内可并行（跨波前置R3等W2的R2），按S24→S29顺序拿号】
  S24 R3采样按图开关（3.2的render底【串行：等S16】，状态落3.2行），S25 10.1实体挂件（等S03+S18），S26 7.3远近切换（等S20＋S08视口），
  S27 12.2拼大图（等S19），S28 12.3热重载（等S19），S29 3.3边玩边加载（等S19）

W4 靠W3【波内可并行（跨波前置R4等W3的R3），按S30→S34顺序拿号】
  S30 R4图集加字段（2.2的render底【串行：等S24】，状态落2.2行），S31 2.1合批（等S24），S32 3.2防闪开关本体（等S24），
  S33 10.2预制场景（等S25＋S08视口），S34 10.3生命周期（等S25）

W5 靠W4【波内部分串行：S41串行等S35，其余可并行，按S35→S41顺序拿号】
  S35 R5深度分支（1.2的render底【串行：等S30】，状态落1.2行），S36 2.2图集旋转染色本体（等S30），S37 8.3动态更新（等S31），
  S38 5.1粒子发射（等S31），S39 5.3扭曲（等S31），S40 5.4发光暗角调色（等S31），S41 1.2前后遮挡本体（等S35）

W6 靠W5【波内可并行，按S42→S45顺序拿号】
  S42 4.3骨骼（等S19+S36），S43 5.2显卡粒子拖尾（等S38），
  S44 5.5材质钩子（等S40），S45 6.1灯光夜色（等S40）

W7 靠W6【波内可并行，按S46→S48顺序拿号】
  S46 4.4状态机（等S17+S11+S42），S47 6.2法线（等S45），S48 6.3影子（等S45）

W8 靠W7单项全过（组合追车未关，留W10回炉）【波内2个可并行，按S49→S50顺序拿号】
  S49 17.2性能崩溃 ＋ S50 17.1编辑器预览【可并行】

W9 靠W8【串行收尾】
  S51 16.2画质分档（等S49）

P5冻结不动（V1口径）：9.1高动态。V2部分解冻见S66，行状态仍记冻结，解冻只做可选管线。
```

分期关门条件（每波必须全绿才进下波；S序号见能力表开工列，一次只做一个）：

本表读数规矩：W0–W9“已绿”均为V1口径（2026-09-15前后单台940MX非release历史实测），不是§5结论；门槛定义唯一见§5，矛盾以§5为准，未达§5的一律留W10回炉（S52–S54）。

| 波 | 装啥（能力号） | 并行标记 | 关门线 | 关门证据 |
|---|---|---|---|---|
| W0 | 0.0底座，1.1透视 | 已绿 | 单测过，无窗口要求 | 见各行 |
| W1 | 1.3a，1.3b，1.4/R1，2.3，2.4，3.1，4.1，7.1，8.1+11.1合测，8.2，13.1，14.1a，15.1，15.2，16.1 | 已绿 | 单测过，无窗口要求（原P0+P1a首步+P2先头部） | 见各行 |
| W2 | R2/9.2，4.2，11.2，13.2，7.2，14.1b，14.2，12.1 | 已绿；波内可并行（跨波前置R2等W1的R1） | 单测过，老路金图不红 | 见各行 |
| W3 | R3，10.1，7.3，12.2，12.3，3.3 | 已绿；波内可并行（跨波前置R3等W2的R2） | 单测过，主能力有独立真窗意向（窗随P2建） | 见各行 |
| W4 | R4，2.1，3.2，10.2，10.3 | 已绿；波内可并行（跨波前置R4等W3的R3） | 每个能力有单测，显卡和CPU画出来只差抗锯齿 | 见各行 |
| W5 | R5，2.2，8.3，5.1，5.3，5.4，1.2（S41已从W6前移，见修订2） | 已绿；波内部分串行：R5串行等R4，S41串行等S35，其余可并行 | 同上，两边齐 | 已绿（2026-09-15：七能力行全已完成；单测见各行＋真卡parity三组0差＋四窗自动判全PASS＋离屏金图0差；无卡环境parity记SKIP原因不假绿；W5关） |
| W6 | 4.3，5.2，5.5，6.1（1.2已前移W5） | 已绿；波内可并行 | 真窗看得出盖对、动作顺、气氛对 | 已绿（2026-09-15：四能力行全已完成；单测见各行＋game_anim/game_particle/game_fx/game_light四窗自动判全PASS＋离屏金图0差；4.3窗线画跨后端像素比不硬比见该行；W6关） |
| W7 | 4.4，6.2，6.3 | V1口径已绿（§5待W10回炉）；波内可并行 | 真窗看切换脆、立体、影子对 | 已绿（2026-09-16：三能力行全已完成；S46单测6项＋fsm自动presents472 hitch0 switches4 双金0差＋人工60秒presents3526 switches30 ptr391 双金0差；S47单测6项＋整包＋vet净＋normal自动presents452 parity0% 穹顶行0.975＞0.602＞0.193 双金0差＋人工60秒presents3541 hitch0 p95约17.5 双金0差；S48单测6项＋shadow自动presents470 moved639 双金0差＋人工60秒presents3524 ptr178 双金0差 hitch10疑桌面负载；切换脆立体影子对三样亲眼对；环境940MX/580.178.04/1920x1080/DISPLAY=:0非release换机重测；W7关） |
| W8 | 17.2，17.1 | 已绿；2个可并行 | 长跑在线，崩留日志，预览所见即所得。注：关的是单项窗，组合追车P3未关门留W10 | 已绿（2026-09-16：两能力行全已完成；17.2报表＋长跑在线＋崩溃上报，17.1单测10项＋自动三窗＋人工三窗收JSON；W8关） |
| W9 | 16.2 | 已绿；串行收尾，等17.2 | 三档帧率有数，切档不闪 | 已绿（2026-09-16：16.2行已完成；单测12项＋自动窗＋人工窗收JSON切14次零失败；W9关） |
| 冻结 | 9.1 | V1冻结不开；V2的S66部分解冻做可选管线 | 先不动 | — |

一句话记：P0 算数，P1 画出来，P2 立起来，P3 有气氛（单项窗绿，组合追车留W10），P4 养得住，P5 先放着。（P是归属分类帮助记忆，开工顺序看W0–W24）

## 每个能力全场景测试和完成状态

一句话：开工以能力为单位，一个能力一行，按所属模块加分期加前置加实现方式指定的地方写，接口先冻，测试与真窗按命名来，6种场景加严版验收全过才算完成，少一样就是没完。

6 种场景是啥：

- A 功能对：正常输入，输出数对得上。
- B 边界错：空、零、超大、坏数据进来，不崩不卡死。
- C 两边齐：显卡和 CPU 画出来只差抗锯齿边缘，不能一个梯形一个方块、一个渐变一个纯色。不画画的包（比如存档音频）这条记不适用。
- D 跑得动：数量、帧率、显存有数，比如多少精灵多少帧。
- E 跑得久：长时间跑显存不涨，不漏，坏档坏图不崩。
- F 真窗认：主能力有独立真窗，先自动判留下 JSON，再留着给人看。纯算数的包（比如变速曲线）只留离屏对比，不强制真窗。

状态有四种：未开工、进行中、已完成、冻结。写已完成必须带日期加证据（测试文件加真窗名），不许口头说完。

本表读数规矩（门槛单源声明，2026-09-17；与§5矛盾以§5为准）：本表状态列里的帧数全是2026-09-15前后单台940MX非release历史实测，每行已注明环境与“换机需重测”，是证据不是门槛定义；门槛定义唯一见§5（数不重写）。本表“已完成·V1口径”=V1口径完成，不等于§5完成；凡实测未达§5的行，一律留W10回炉（S52–S54）正式包三遍取最差重测，回炉绿了才算过§5。

| 编号 | 能力 | 所属模块 | 归属(P) | 开工(S/W) | 前置 | 接口约定 | 实现方式 | 测试与真窗 | 全场景测啥 | 状态 |
|---|---|---|---|---|---|---|---|---|---|---|
| 0.0 | 共用底座core | game/core（7文件） | P0最前 | S00/W0 | 无（已绿，可开工） | Vec2/Rect/Color/Time/Rand/AssetID/Result/Version | game新包，render老类型边界转 | 测core_test；窗免 | A数对，B空零超大不崩，D万次耗时有数，E长跑不漂 | 已完成·V1口径（2026-09-14；game/core/core_test.go 16项全PASS＋CGO_ENABLED=0可构建；D万次约10.5ns/op；窗免-纯算数，边界往返等价即离屏依据） |
| 1.1 | 上层透视投影 | game/camera/project.go | P1b | S00/W0 | 0.0 | Project(world)→screen，DepthToScale | game新包只算数，算好喂现有梯形三角 | 测camera_project_test＋离屏；窗免 | A 算出的屏坐标对，B 深度零和负不崩，C 两边齐，D 56 段路全量重投影帧率有数，E 环线跑千圈闭合，F 离屏对比 | 已完成·V1口径（2026-09-14；game/camera/camera_project_test.go 6项全PASS＋CGO_ENABLED=0可构建；D56段路114点约48.7ns/op远窄于近单调收敛；E千圈逐位一致＋往返1e-9闭合；C边界往返无损＋重放一致即两边同数；F离屏金比对project_cases.json梯形近宽远窄；窗免-纯算数） |
| 1.2 | 前后遮挡 | game/sprite/ysort.go（深度扩展）+ render | P1b | S41/W5（R5底S35/W5先行） | core＋R5深度分支 | SetDepth＋Sort()，远先近后 | game排序为主，需动显卡走隔离新分支 | 测ysort_depth_test；窗game_sprite--case=depth | A 远先近后盖对，B 同深不闪，C 两边齐，D 百个遮挡帧率有数，E 长跑顺序不乱，F 真窗看盖对 | 已完成·V1口径（2026-09-15 R5底＋本体＋真卡＋正式窗全过：render/depth_r5.go+depth_r5_test.go 7项＋depth_r5_cases.json真卡far_first 0/1024 mean0＋three_layers 0/1024 mean0.022 max2；本体game/sprite/depth.go+depth_test.go 6项全PASS＋depth_cases.json 10组＋D百遮挡约5.6us/sort＋E万次重放一致；窗examples/game_sprite--case=depth自动10秒presents589 fps58.2/58.9 p95 17.5 parity0% sorted1 probe1 golden0% PASS，真机截图上红盖绿+中红盖绿+下蓝同深三组全亮；环境940MX/580.178.04/1920x1080/DISPLAY=:0，非release，换机需重测） |
| 1.3a | 镜头纯算 | game/camera/camera.go | P0 | S08/W1 | 0.0 | Camera{pos,zoom,rot,shake,limit}＋View() | game新包只算数，不碰渲染 | 测camera_test＋离屏；窗免 | A位置缩放角度震动限位平滑锚点数对，B零负钳住，D万次耗时有数，E长跑不漂 | 已完成·V1口径（2026-09-15；game/camera/camera_test.go 7项全PASS＋CGO_ENABLED=0可构建；D万次约260ns/op；E十万步跟随收敛重放一致；C边界往返无损即离屏依据；F离屏金比对camera_cases.json 7视图冻结；窗免-纯算数） |
| 1.3b | 视差画出 | game/camera/parallax.go＋project.go | P1b | S09/W1 | 0.0＋1.1 | Parallax＋Project(world)→screen | game新包算好喂梯形三角 | 测parallax_test；窗game_camera--case=follow | A错开重复镜像对，B零负不崩，C两边齐，D全屏帧率有数，E跑千米不飘无缝，F真窗看跟随长路 | 已完成·V1口径（2026-09-15；game/camera/parallax_test.go 6项全PASS＋CGO_ENABLED=0可构建；D 7层x13点x8机728屏约45ns/op；E 10公里101步缝1e-9内＋千圈重放逐位一致；C边界往返无损＋重放一致即两边同数；F离屏金比对parallax_cases.json四角梯形近宽远窄＋双镜像缝；窗免-W1纯算数，跟随窗game_camera后建） |
| 1.4 | 梯形贴图对齐 | render/m4_extensions.go | P1a-R1 | S01/W1 | 0.0 | DrawImageQuad透视回退，错报错 | render小修CPU回退 | 测quad_cpu_test＋离屏；窗game_quad | A 梯形贴对，B 退化四边形不崩，C 两边齐（重点），D 单图帧率有数，E 长跑不漏，F 真窗加离屏对比 | 已完成·V1口径（2026-09-15；render/quad_cpu_test.go 8项全PASS：A梯形/矩形探针全对＋F离屏golden 0%/C parity nearest逐位一致bilinear 0%/D单图约0.2ms/E500次交替终绿＋B退化/自交/非有限哨兵错不崩；真窗examples/game_quad RUN_SECONDS=8自动判OK presents约470 parity 0% outside_white=1 golden 0% gpu_ops>0；前置0.0复核绿；R1接口冻结QuadKind/QuadDrawOptions/DrawImageQuadEx/ClassifyQuad＋哨兵错5个，老DrawImageQuad签名不动） |
| 2.1 | 精灵合批 | game/sprite/batch.go | P1b | S31/W4 | 0.0＋R3采样开关 | Batch{Add,Flush}，同图一次交 | game新包攒批，只调现有接口 | 测batch_test；窗game_sprite--case=batch千树 | A 同图一批画对，B 空批零尺寸跳过，C 两边齐，D 千个精灵帧率调用次数有数，E 长跑不涨，F 真窗看千树 | 已完成·V1口径（2026-09-15；game/sprite/batch_test.go 6项全PASS＋sprite整包18项PASS＋go vet净＋CGO_ENABLED=0可构建；D千精灵Flush约550us/2次调用；E5000次重放一致200轮回零；C分组无损重放一致即两边同数；F离屏金比对batch_cases.json 4组冻结；窗意向game_sprite--case=batch随P2建；前置S24复核绿） |
| 2.2 | 图集旋转染色翻转轴心 | game/sprite/atlas.go + render扩展 | P1b | S36/W5（R4底S30/W4先行） | 0.0＋R4图集扩展 | AtlasSprite＋rot/flip/pivot/tint/filter | render加新分支+game包，老路不动 | 测atlas_test；窗game_sprite--case=rot | A 角度颜色翻转轴心单图过滤对，B 空块跳过，C 两边齐，D 百块帧率有数，E 长跑不漏，F 真窗看转和换色转身脚底对齐 | 已完成·V1口径（2026-09-15 R4底＋本体＋真卡＋正式窗全过，染色已上显卡逐顶点颜色混画根因修后复验三卡齐亮：game/sprite/atlas_test.go 6项全PASS＋atlas_cases.json 13组＋R4真卡identity 0/1024 rot90 0/1024 mean0.003 max1＋本体真卡双0/1024；半透明三组显卡/CPU最大差1；D百块转换约23us/批；窗examples/game_sprite--case=all自动10秒presents591 fps58.3/59.0 p95 17.5 cpuFb0 parity0% probe1 golden0% PASS，真机截图ROT白底+红块+橙染色+红条+梯形+绿批量全亮；环境940MX/580.178.04/1920x1080/DISPLAY=:0，非release，换机需重测） |
| 2.3 | 按高低排序加层号分层 | game/sprite/ysort.go | P1b | S10/W1 | 0.0 | YSort(feetY)＋Layer{world,fx,ui} | game新包只算数 | 测ysort_test；窗game_sprite--case=ysort | A 脚底高度加层号排对、界面层不被盖，B 同高稳定不闪，C 两边齐，D 百个排序耗时有数，E 长跑不乱，F 真窗看前后盖特效不飘界面 | 已完成·V1口径（2026-09-15；game/sprite/ysort_test.go 6项全PASS＋CGO_ENABLED=0可构建；D百精灵2000次约38.9us/sort；E万次重放逐位一致＋走格往返不粘；C输入不改＋结果新片＋重放一致即两边同数；F离屏金比对ysort_cases.json 7组冻结＋world<fx<ui＋Y递增断言；窗免-W1纯算数，排序窗game_sprite后建） |
| 2.4 | 序列帧多动作库 | game/sprite/flipbook.go | P1b | S11/W1 | 0.0 | Flipbook{Play,Update,OnEvent}＋动作库 | game新包只算数 | 测flipbook_test；窗game_sprite--case=anim | A 帧号时长循环回调多套动作切换对，B 缺帧不崩，C 两边齐，D 多路播放帧率有数，E 长跑不漂，F 真窗看待机跑跳切换 | 已完成·V1口径（2026-09-15；game/sprite/flipbook_test.go 6项全PASS＋sprite全包12项全PASS＋go vet净＋CGO_ENABLED=0可构建；A六序列全对（循环/单次/乒乓/切换/单帧）回调与返回逐事件一致；B坏clip七类invalid-arg零入库、坏Play不动位、零负dt静默、百万毫秒级dt收敛单事件、nil接收器不崩；D千路x200次约40.6ns/op；E十万步双重放逐位一致＋单次播完恒守尾帧；C纯算数不画画以库拷贝隔离＋重放逐位一致即两边同数；F离屏金比对flipbook_cases.json循环回0/单次守22/乒乓双端＋帧号归属断言；窗免-W1纯算数，动画窗game_sprite后建；前置0.0复核绿core 16项全PASS） |
| 3.1 | 压缩图 | game/tex/compressed.go | P2长链首 | S02/W1 | 0.0 | LoadKTX2/Basis→上传，坏报Result错 | game新包独立解码，不碰老8位路 | 测compressed_test＋离屏；窗免 | A 解码上传对，B 坏文件报错不崩，C 画出来和原图差在容差内，D 大图显存有数，E 反复加载释放不涨，F 离屏对比 | 已完成·V1口径（2026-09-15；game/tex/compressed_test.go 6项全PASS＋CGO_ENABLED=0可构建；A三文件头尺寸块数上传像素全对＋红绿蓝白象限逐像素一致＋13处At抽点一致；B空零截断坏头零维非4倍类型错层长错9类bad-data＋vk超压3D数组立方mipmap9类unsupported＋16384超限out-of-memory＋Basis保留unsupported缺文件not-found＋At越界闭合；C容差0下65616像素maxDiff0 bad0＋At与字节片逐像素一致即两边同数；D256图20次约0.5ms每parse上传32768B解码262144B比8；E千次重放逐位一致＋块像素拷贝隔离＋复读文件一致；F离屏金比对compressed_cases.json三文件＋13 spots＋块缝锐边＋4倍网格断言；窗免-纯解码不建窗；前置0.0复核绿core/camera/sprite全PASS；只动game/tex新包render老8位路未碰） |
| 3.2 | 远处防闪开关 | game/tex/mipmap.go + render/image_pipeline.go | P1b | S32/W4（R3底S24/W3先行） | 0.0＋R3采样开关 | SetFilter(near/far,aniso)，按图开关 | render按图开关+game开关 | 测mipmap_test；窗game_tex--case=far | A 近清远稳，B 超小尺寸不崩，C 两边齐（重点），D 远景帧率有数，E 长跑不抖，F 真窗看远树 | 已完成·V1口径（2026-09-15；game/tex/mipmap_test.go 5项全PASS＋tex全包17项PASS＋render金图quad/verts parity绿＋CGO_ENABLED=0可构建；D千图set+get约0.65ms、2万次取级约75ns/op；E千次重放一致Clear归零；C压缩tol0下maxDiff0＋sampler_contract5组全对＋零值命令合并逐位一致；F离屏金比对mipmap_cases.json；窗意向game_tex--case=far随P2建；前置0.0+S16复核绿；只动mipmap+doc+单测+金数据+image_pipeline新分支老路不动） |
| 3.3 | 后台边玩边加载 | game/tex/stream.go + game/asset | P2 | S29/W3 | 0.0＋12.1 | Stream{Request,Poll}，缺占位 | game新包后台链 | 测stream_test；窗game_tex--case=stream | A 到边加载不卡，B 缺图占位不崩，C 两边齐，D 加载时帧率有数，E 跑大地图不涨，F 真窗看边走边出 | 已完成·V1口径（2026-09-15；game/tex/stream_test.go 6项全PASS＋compressed老6项回归绿＋race无竞争＋CGO_ENABLED=0可构建；D64块小图后台约1.6ms前台Poll约0.8us/次、大图约1.1ms/张；E千次重放一致200块装卸回基线；C后台与同步逐位一致；F离屏金比对stream_cases.json 8冻结＋品红占位16像素全对；窗意向game_tex--case=stream随P2建；前置0.0+S19复核绿；只动stream+doc+单测+金数据老同步路未碰） |
| 4.1 | 变速曲线 | game/anim/easing.go | P0 | S04/W1 | 0.0 | Ease(t)→v，曲线库冻名 | game新包只算数 | 测easing_test＋离屏；窗免 | A 曲线值对，B 时间越界钳住，C 不适用，D 万次求值耗时有数，E 长跑不漂，F 离屏对比 | 已完成·V1口径（2026-09-15；game/anim/easing_test.go 6项全PASS＋CGO_ENABLED=0可构建；D万次约153ns/op；E20万x31重放逐位一致＋端点精确；C重放一致＋Name/Parse往返无损即两边同数（C不适用-纯算数）；F离屏金比对easing_cases.json 31种冻结＋单调/超调/镜像形状断言；窗免-纯算数） |
| 4.2 | 关键帧时间轴加事件軌 | game/anim/timeline.go | P1b | S17/W2 | 0.0＋4.1 | Timeline{AddKey,Sample,OnEvent} | game新包只算数 | 测timeline_test；窗game_anim--case=tl | A 插值循环事件回调（出刀光播声调方法显隐）对，B 空轨不崩，C 两边齐，D 多轨帧率有数，E 长跑不漂，F 真窗看动作事件准时 | 已完成·V1口径（2026-09-15；game/anim/timeline_test.go 6项全PASS＋CGO_ENABLED=0可构建＋go vet净；D八轨20万采样约156.8ns/op＋2.5万次Update约52ns/op；E十万步双重放一致循环落回0＋单次播完守尾；C纯算数以双重放逐位一致＋输入不改即两边同数；F离屏金比对timeline_cases.json三序列冻结＋键位命中/线性中点/回绕反弹事件序断言；窗免-W2纯算数，真窗game_anim--case=tl随P2建；前置0.0＋S04复核绿） |
| 4.3 | 骨骼蒙皮对齐 Spine | game/anim/skeleton.go | P2长链 | S42/W6 | 0.0＋12.1＋2.2 | Skeleton{Pose,Skin,IK}，认Spine子集 | game新包+认Spine JSON | 测skeleton_test；窗game_anim--case=sk真资源 | A 骨头插槽皮肤网格权重绘制次序IK约束物理惯性对，B 缺骨不崩，C 两边齐，D 单角色帧率有数，E 长跑不错位，F 真窗看走跑跳真资源 | 已完成·V1口径（2026-09-15；game/anim/skeleton_test.go 6项全PASS，主会话复核过＋本包easing/timeline回归绿＋前置core/asset/sprite复核绿＋CGO_ENABLED=0可构建＋go vet净；D单角色姿态+IK+两块蒙皮2万次约73.3ms约3666ns/op；E5万步双重放一致IK吸住误差约5e-15；C双重放逐位一致＋边界往返无损即两边同数；F离屏金比对spine_cases.json＋spine_hero.json冻结7骨姿态＋绘制次序；窗examples/game_anim--case=sk已建并改成双腿人形（2026-09-15改：窗自有13骨直立人，头顶圆脸＋衬衫躯干＋双臂＋双腿裤管＋鞋＋地面线，走并腿跑分腿跳收腿三态可分；引擎7骨链 hero 未动，窗13骨走同一套 Pose/Skin/IK/Transform 接口，双腿 IK 各自吸住误差约1e-9；主会话复跑自动8秒门禁EXIT0 presents454 fps52.9/56.6 p95 17.5 parity真 golden0 当窗金图0，切4次；首遍基线重冻后第二遍即0差；另有负载高抖动遍已注明环境原因；C为纯算数三姿态逐位重放＋5槽次序对即两边同数，窗线画走path/fill跨后端像素比归9.2管故不硬比；环境940MX/580.178.04/1920x1080/DISPLAY=:0非release换机重测）；物理惯性占位报Unsupported未写死） |
| 4.4 | 动作混合状态机 | game/anim/statemachine.go | P2长链 | S46/W7 | 4.2＋2.4＋4.3 | State{CanTo,Blend}，混合时长 | game新包只算数 | 测statemachine_test；窗game_anim--case=fsm | A 切换混合对，B 非法切换不崩，C 两边齐，D 切百次耗时有数，E 长跑不卡死，F 真窗看切换 | 已完成·V1口径（2026-09-16；game/anim/statemachine_test.go 6项全PASS＋statemachine_cases.json三态六序列＋fsm_golden.png离屏基线；D settled切100次共16us约160ns/次；E20万步双机重放逐位一致＋500次非法风暴错码稳定；C六序列双重放逐位一致＋拷贝隔离即两边同数；F窗examples/game_anim--case=fsm自动 presents474 fps59.1/57.0 p95 18.1 hitch0 离屏金0% 128000像素 窗金0% 559840像素 切换4次 PASS，三卡＋交叉曲线＋活条亲眼验对；环境DISPLAY=:0 940MX/580.178.04 integrated后端 gpu_ops16748 1920x1080非release换机重测；前置S17/S11/S42各6项复核绿＋sk旧窗重跑EXIT0 presents424双金0差切换3次未红（sk_final.png快照顺手重写像素0差，主会话定夺留不留）；render未碰；主会话复跑go test ./game/anim整包PASS；人工常驻2026-09-16 presents3296 switches32 ptr515 双金0差 backend=x11，用户关窗收JSON，三窗同开fps约50.8偏低待闲时单窗复测） |
| 5.1 | 粒子发射形状乱流子发射 | game/particle/emitter.go + cpu.go | P3 | S38/W5 | 0.0＋2.1 | Emitter{Spawn,Update}＋形状乱流子发射 | game新包+靠sprite画 | 测emitter_test；窗game_particle--case=fire | A 数量速度寿命重力颜色形状乱流子发射对，B 零发射不崩，C 两边齐，D 千粒子帧率有数，E 长跑不涨，F 真窗看火锥烟飘 | 已完成·V1口径（2026-09-15 引擎混画根因已修＋染色上显卡后复验全过：emitter_test.go 6项全PASS＋emitter_cases.json 7例；窗examples/game_particle--case=fire自动10秒presents578 fire447 smoke559 batch1 fps57.1/57.7 p95 18.2 raster 8.9ms cpuFb0 hitch2 PASS，真机截图火锥亮黄红上升＋烟灰缓飘乱流摆动；根因render混合提交错目标致全黑，修render/context_pass_scratch.go+gpu提交+scene两record接线＋新单测pass_scratch 2项；环境940MX/580.178.04/1920x1080，非release，换机需重测） |
| 5.2 | 显卡粒子拖尾宽窄渐变 | game/particle/gpu.go + trail.go | P3 | S43/W6 | 5.1＋render新管线 | GPUPool＋Trail{width,grad,joint} | game新包+新管线隔离做 | 测gpu_trail_test；窗game_particle--case=trail | A 轨迹宽窄颜色接头对，B 空轨不崩，C 两边齐，D 数千点帧率有数，E 长跑不涨，F 真窗看刀光由宽到窄 | 已完成·V1口径（2026-09-15；game/particle/gpu_trail_test.go 6项全PASS，主会话复核过＋老emitter 6项回归绿＋sprite整包绿＋CGO_ENABLED=0可构建＋go vet净；D3000粒子6000点spawn约2.1ms/update约0.9ms/append约5.7ms/flush约2.8ms一次提交draws=1；E2000次重放一致200轮装满清空回零；C同种子池子与CPU发射器逐位一致即两边同数；F离屏金比对gpu_trail_cases.json 8轨迹＋3池子；窗examples/game_particle--case=trail自动8秒主会话复跑probes 3/3 golden changed=0 presents467 fps57.4/58.3 p95 17.9 hitch0 gpu_ops12903 cpu回退0 alive45 trails45 points494 batch1 PASS，真机刀光头宽尾窄亮黄拖红；环境940MX/580.178.04/1920x1080/DISPLAY=:0，非release，换机需重测；render主路未碰，只加trail case＋main.go 8行门放行） |
| 5.3 | 扭曲热浪 | game/fx/warp.go | P3 | S39/W5 | 0.0＋2.1 | Warp{strength,noise}，偏移采样 | game新包+新中间图隔离做 | 测warp_test；窗game_fx--case=warp | A 偏移方向对，B 强度零不崩，C 两边齐，D 全屏帧率有数，E 长跑不花，F 真窗看水晃 | 已完成·V1口径（2026-09-15 数算+单测+正式窗全过，混画修后复验仍全亮：warp_test.go 6项全PASS＋fx整包12项PASS；D全屏1920x1080约179ms最优、单点约67ns/次；金数据warp_cases.json 9偏移+2整图逐字节命中；窗examples/game_fx--case=all自动10秒presents591 parity0% moved32499 corner0.07 halo0.33 fps57.3/58.9 p95 17.4 PASS，真机截图左原图+中水晃+右胶片三卡齐亮；环境940MX/580.178.04/1920x1080，非release，换机需重测） |
| 5.4 | 发光暗角调色 | game/fx/bloom.go + vignette.go + lut.go | P3 | S40/W5 | 0.0＋2.1 | Bloom/Vignette/LUT，接滤镜链 | game新包+新中间图隔离做 | 测bloom_lut_test；窗game_fx--case=grade | A 发光暗角表对，B 空表不崩，C 两边齐（重点），D 全屏帧率有数，E 长跑不漏，F 真窗看气氛 | 已完成·V1口径（2026-09-15 引擎层+单测+正式窗全过：bloom_lut_test.go 6项全PASS＋fx整包12项PASS＋金数据bloom_lut_cases.json；C真卡parity 0% mean0；窗examples/game_fx--case=all自动10秒presents591 parity0% corner0.07 halo0.33 fps57.3/58.9 p95 17.4 PASS＋基线fx_final_base.png落盘，真机截图三卡齐亮；环境940MX/580.178.04/1920x1080，非release，换机需重测） |
| 5.5 | 自定义材质钩子 | game/fx/custom.go | P3 | S44/W6 | 0.0＋5.4 | Custom{name,params}，只碰游戏层 | game新钩子只碰游戏层 | 测custom_test；窗game_fx--case=custom | A 钩子进出数对、只碰游戏层，B 坏着色器报错不崩主路，C 两边齐（CPU 明确降级标记），D 单钩子全屏帧率有数，E 反复装卸不漏，F 真窗看描边溶解 | 已完成·V1口径（2026-09-15；game/fx/custom_test.go 6项全PASS，主会话复核过＋本包warp/bloom_lut回归共18项全PASS＋CGO_ENABLED=0可构建＋go vet净；D1080p一遍identity约70ms/outline最优约26ms纯CPU参考路；E200轮x16装卸回零2000次重放一致500次坏图错码稳定；C纯算数同一套数重放一致＋identity不降级outline/dissolve标CPU降级即两边约定；F离屏金比对custom_cases.json 7组identity＋outline＋dissolve；窗examples/game_fx--case=custom自动约8秒主会话复跑EXIT0 presents471 fps57.0/58.8 p95 18.2 parity0% golden0% 当窗金图差0 kept3138/edge3710/gone31552 outline356 PASS，真机左原片＋中红圈描边＋右橙边溶解三卡齐亮；回归all窗复跑EXIT0 golden0% hitch0老窗未红；环境940MX/580.178.04/1920x1080/DISPLAY=:0，非release，换机需重测；render主路未碰，老case走custom分支外默认不动；入库fx_custom_golden.png＋fx_custom_base.png，fx_custom_last.png为运行快照不入库） |
| 6.1 | 2D 灯光灯片分层夜色 | game/light/light.go | P3 | S45/W6 | 0.0＋5.4 | Light{pos,range,cookie}＋分层＋夜色 | game新包盖在fx之后 | 测light_test；窗game_light--case=torch | A 位置范围衰减灯片强度颜色分层照全局夜色对，B 零灯不黑屏，C 两边齐，D 多灯帧率有数，E 长跑不闪，F 真窗看手电只照人不照背景 | 已完成·V1口径（2026-09-15；game/light/light_test.go 6项全PASS，主会话复核过＋CGO_ENABLED=0可构建＋go vet净；D8灯打64x64一次约1.7ms约52.9ns/像素/灯；E手电组2000遍重放一致坏图500次错码全对；C同一套数CPU与显卡受光系数一致即两边同数；F离屏金比对light_cases.json 8组系数＋7组整图；窗examples/game_light--case=torch自动8秒主会话复跑EXIT0 presents474 fps57.4/59.2 p95 17.7 hitch0 parity0% golden0% 当窗金图34万像素差0 lit2059 gain0.66 bg与夜色逐位同 夜最暗0.069不黑屏 PASS，真机左白天＋中夜色＋右手电只照人不照背景；环境940MX/580.178.04/1920x1080/DISPLAY=:0，非release，换机需重测；render主路未碰，新包＋独立窗） |
| 6.2 | 法线受光 | game/light/normal.go | P3 | S47/W7 | 6.1 | NormalMap＋光向，受光系数 | game新包盖在fx之后 | 测normal_test；窗game_light--case=normal | A 受光方向对，B 缺法线不崩，C 两边齐，D 单图帧率有数，E 长跑不花，F 真窗看立体 | 已完成·V1口径（2026-09-16；game/light/normal_test.go 6项全PASS＋normal_cases.json 5组整图＋8组系数＋6组点积＋窗小金图normal_dome_golden.png；D 8灯打64x64约2.5ms约75.5ns/像素/灯；E2000遍重放逐位一致＋500次坏图错码稳定＋穹顶左亮右暗形状保持；C双重放逐位一致＋存取往返无损即两边同数；F窗examples/game_light--case=normal自动 presents476 fps56.9/59.3 p95 18.2 hitch0 parity0% 探针朝灯0.984>平0.880>背灯0 穹顶行0.975>0.609>0.193 lit17228像素44.9% 增益0.78 离屏金0% 6144像素 窗金0% 343360像素 PASS；环境940MX/580.178.04/1920x1080/DISPLAY=:0非release换机重测；首遍窗探针取数小错修窗后第二遍即绿引擎数未动；前置S45 light_test 6项复核绿＋torch旧窗重跑PASS；render主路＋light.go本体未碰；主会话复跑go test ./game/light整包18项全PASS＋vet净；人工常驻2026-09-16 presents2240 ptr194 双金0差 backend=x11，用户关窗收JSON，三窗同开fps约44.2偏低待闲时单窗复测；人工2026-09-16发现窗表现问题待修：中卡底是均匀0.75灰、无穹顶轮廓无底纹，右暗部沉到与背景同色，人眼看不见穹顶形状，只能看见一团亮晕化开，方向用户2026-09-16确认先放着后面再说，人工待定，W7暂不关门，引擎数不动；窗表现真实修2026-09-16（只动examples/game_light/main.go的makeDomeNormals：圈内起坡、圈外贴平，底仍均匀灰、灯仍侧光，game/light/normal.go引擎数未动）：normal单测6项全PASS＋整包PASS＋vet净，自动窗第二遍presents474 fps57.2/59.1 p95约17.2 hitch0 parity0% 探针朝灯0.984>平0.880>背灯0 穹顶行0.975>0.609>0.193 lit33557像素87.4% 增益0.78 离屏金0% 6144像素 窗金0% 343360像素 PASS，中卡穹顶圈已露、右暗部浮在亮底上不再沉底，小金图normal_dome_golden.png＋窗基线normal_final_base.png因窗法线重冻一次（改谁为啥哪天已记），torch/shadow旧窗重跑双0未红，人工常驻90秒2026-09-16 presents5316 p95约17.3 hitch0 parity0% 双金0差 backend=x11 ptr0 key0 rs0 已留人工看；圆边锯齿修2026-09-16（只动examples/game_light/main.go：圈口2像素滑到平＋中卡4倍超采样再缩回，game/light/normal.go引擎数未动）：单测6项＋整包PASS＋vet净，自动窗第二遍presents471 p95约17.5 hitch1 parity0% 穹顶行0.975＞0.602＞0.193 lit33762像素87.9% 增益0.78 双金0差 PASS，三倍放大看圆边已是软过渡，小金normal_dome_golden.png＋窗基线normal_final_base.png因窗法线重冻一次，torch/shadow重跑双0未红；以上两次“暂不关门/仍不关门”为修前记录，2026-09-16 W7已关，见W7关门行） |
| 6.3 | 投影遮挡 | game/light/shadow.go | P3 | S48/W7 | 6.1 | Occluder＋Shadow，挡变暗 | game新包盖在fx之后 | 测shadow_test；窗game_light--case=shadow | A 影子方向对，B 无挡不崩，C 两边齐，D 多挡帧率有数，E 长跑不错位，F 真窗看影子 | 已完成·V1口径（2026-09-16；game/light/shadow_test.go 6项全PASS＋shadow_cases.json 11组系数＋3组整图＋窗小金图shadow_wall_golden.png；D单手电32墙打64x64约14.87ms约114ns/像素/挡；E2000遍重放逐位一致＋500次坏图错码稳定；C同一套数CPU与显卡逐位一致；F窗examples/game_light--case=shadow自动 presents453 fps约54/57 p95 18.3ms parity0% 小金图6144像素0差 窗金图343360像素0差 PASS，真机右卡人左半亮右半黑影子方向对；环境940MX/1920x1080/DISPLAY=:0非release hitch7系桌面并行构建偏高换机重测；前置S45 light_test 6项复核绿＋torch旧窗重跑EXIT0全0未红；render主路未碰；主会话静态复核JSON合法＋6测齐＋图落盘，S47完工后补跑go test ./game/light整包18项全PASS＋vet净三窗分支共存无干扰；人工常驻2026-09-16 presents1672 ptr276 moved639 双金0差 backend=x11，用户关窗收JSON，三窗同开fps约40.8＋启动时深度图OOM降档各一次待闲时单窗复测） |
| 7.1 | 瓦片斜45度对象自动拼 | game/tilemap/tilemap.go + iso.go | P2 | S05/W1 | 0.0 | Tilemap＋对象层＋自动拼，认TMX子集 | game新包只算数+认TMX | 测tilemap_test；窗game_tilemap--case=map | A 格子号对象摆位碰撞导航自动拼对，B 缺块占位不崩，C 两边齐，D 大图帧率有数，E 反复进出不涨，F 真窗看地图摆怪路沿自接 | 已完成·V1口径（2026-09-15；game/tilemap/tilemap_test.go 6项全PASS＋CGO_ENABLED=0 go build ./...过；D128x128 2000次约0.2us/op；E万次掩码重放一致＋200次重解析一致；C边界往返无损＋重放一致即两边同数；F离屏金比对tilemap_cases.json正交4x3＋斜45度3x3冻结＋形状断言；窗免-W1纯算数，地图窗game_tilemap后建） |
| 7.2 | 分区加载剔除 | game/tilemap/chunk.go | P2 | S20/W2 | 7.1＋1.3a视口 | Chunk{Load,Unload}＋视口剔除 | game新包只算数 | 测chunk_test；窗game_tilemap--case=chunk | A 镜头内外装卸对，B 跳跃镜头不崩，C 两边齐，D 快移帧率有数，E 跑大圈不涨，F 真窗看边走边装 | 已完成·V1口径（2026-09-15；game/tilemap/chunk_test.go 6项全PASS＋老tilemap 6项回归绿＋camera 13项绿＋go vet净＋CGO_ENABLED=0可构建；D六四图八块二千次约23.5us/op可见7988块次；E千来回二千次装载集恒等重建一致；C纯算数以块原点边界逐位无损＋逐位重放即两边同数；F离屏金比对chunk_cases.json格属包络六视口装卸跳跃冻结＋map_chunk16.tmx十六图全1；窗免-W2纯算数，真窗game_tilemap--case=chunk随P2建；前置S05＋S08复核绿，本体只读未写） |
| 7.3 | 远近切换 | game/tilemap/lod.go | P2 | S26/W3 | 7.2＋1.3a | LOD{near,far}，临界回滞 | game新包只算数 | 测lod_test；窗game_tilemap--case=lod | A 远简近精切换对，B 临界不闪，C 两边齐，D 切换耗时有数，E 长跑不抖，F 真窗看远近 | 已完成·V1口径（2026-09-15；game/tilemap/lod_test.go 6项全PASS＋tilemap/chunk/camera回归绿＋CGO_ENABLED=0可构建＋go vet净；D2万次选档约25.8ns/op近28561远11439；E带内5000次不翻＋远沿抖动1000次只切1次＋2000步重放一致；B192算近320算远同距抱老档；C边界往返无损＋双算一致；F离屏金比对lod_cases.json；窗意向game_tilemap--case=lod随P2建；前置S05+S08复核绿） |
| 8.1 | 固定步长补间 | game/step/fixed.go | P0 | S03/W1 | 0.0 | Fixed{Step,Interp}，步长冻死 | game新包只算数 | 测fixed_test＋离屏；窗免 | A 快慢机动作一致，B 大步钳住，C 不适用，D 步数耗时有数，E 长跑不漂，F 离屏对比 | 已完成·V1口径（2026-09-15；game/step/fixed_test.go 6项全PASS＋go vet净＋CGO_ENABLED=0可构建；A八组冻结步数余数blend全对＋48ms两种切分一致＋6组blend冻结；B零负dt拒收nil池不崩＋超大钳MaxFrame＋NaN/Inf blend钳住＋Reset保Dt；D 6万帧约4.6ns/frame＋万次blend约9.3ns/op；E十万帧双重放逐位一致＋台账steps*dt对账；C纯算数不画画以core.Step往返＋重放逐位一致即两边同数；F离屏金比对fixed_cases.json整除无余/半步50/钳制余数冻结；窗免-W1纯算数；前置0.0复核绿core全PASS；只动game/step新包render未碰） |
| 8.2 | 路径顶点复用 | game/step/pool.go | P0 | S12/W1 | 0.0 | Pool{Get,Put}，路径顶点复用 | game新包纯工程池化 | 测pool_test＋离屏；窗免 | A 复用画对，B 空池不崩，C 两边齐，D 复用率内存有数，E 长跑不涨，F 离屏对比 | 已完成·V1口径（2026-09-15；game/step/pool_test.go 6项全PASS＋go vet净＋CGO_ENABLED=0可构建；A三尺寸族取放写满读逐位一致＋双活不混＋边界往返无损；B空零负超限nil池错码分清（InvalidArg/OutOfMemory）＋NaN/Inf原样过池；D 2000次x256三族约11.6us/rep命中率1.000留存14336B；E万次1/1/1稳态7168B零增长＋计数器3x对账；C纯池化不画画以边界往返无损＋20轮重放逐位一致即两边同数；F离屏金比对pool_cases.json限额公式＋首样稳定＋LIFO稳命中冻结；窗免-W1纯算数；前置0.0复核绿core全PASS＋camera/sprite/tex/tilemap/anim回归绿；只动game/step新包render未碰） |
| 8.3 | 动态局部更新 | ui/scene + game/step（动态层） | P2 | S37/W5 | 0.0＋2.1 | DirtyLayer，动哪更哪 | scene动态层+game包 | 测dirty_test；窗game_step--case=dirty追车 | A 动哪更哪，B 全动退整屏不崩，C 两边齐，D 脏块数帧率有数，E 长跑不漏，F 真窗看追车 | 已完成·V1口径（2026-09-15 引擎层+单测+正式窗全过，混画修后复验仍亮：ui/scene/game/step各dirty_test.go 6项全PASS＋两边dirty_cases.json逐字节一致；窗examples/game_step--case=dirty自动10秒presents414 moved1383 dirty_max1 full3 fps40.8/41.3 PASS，真机截图青绿静块+黄追车框+计数器全亮；当时桌面负载高帧率偏低，闲时重测；环境940MX/580.178.04/1920x1080，非release，换机需重测） |
| 9.1 | 高动态亮度流程 | — | P5冻结 | 冻结 | — | 冻结不动 | 暂缓不动 | 冻结 | 暂缓，先不动，状态冻结 | 冻结 |
| 9.2 | CPU 回退画质 | render/vertices.go | P1a-R2 | S16/W2 | 0.0 | CPU真渐变，降级标记 | render小修CPU渐变 | 测vertices_cpu_test＋离屏；窗免 | A 渐变真渐变，B 空 mesh 跳过，C 两边齐（重点），D 回退帧率有数，E 长跑不漏，F 离屏对比 | 已完成·V1口径（2026-09-15；render/vertices_cpu_test.go 10项全PASS真机＋core绿＋S01 quad parity真机PASS＋oom 2项绿＋button showcase绿＋M1两项门禁真机转绿＋go vet净＋CGO_ENABLED=0可构建；D六四三角二百次约138.4us/次；E五百次交替终hub124/0/131冻结；C真机parity 0/4096（0.000%）mean0.015 max1只差抗锯齿＋金图verts_golden.png零改动；F离屏金比对verts_cases.json五组探针冻结；真机2026-09-15：X11 DISPLAY=:0＋NVIDIA 940MX 580.178.04＋Intel HD520＋WGPU_NATIVE_PATH指lib库＋DiscreteGPU探针绿，S01 nearest逐位0/bilinear mean0.019 max2全过；收敛修parity防塌断言取错形心改近红点14,50，逻辑金数据零改；窗免；前置0.0＋S01复核绿，老Draw签名不动） |
| 10.1 | 实体挂件 | game/world/entity.go | P2 | S25/W3 | 0.0＋8.1/11.2 | Entity＋Comp＋父子变换继承 | game新包只算数 | 测entity_test＋离屏；窗免 | A 父子变换继承对，B 删爹不崩，C 不适用，D 千实体耗时有数，E 反复生灭不涨，F 离屏对比 | 已完成·V1口径（2026-09-15；game/world/entity_test.go 6项全PASS＋go vet净＋CGO_ENABLED=0可构建；D千实体20万矩阵约457.7ns/op；E500轮生灭活数回基线逐位一致；C纯算数以双重放逐位一致＋core边界往返无损即两边同数；F离屏金比对entity_cases.json四链冻结；窗免-纯算数；前置0.0+S03+S18复核绿；只动game/world四文件） |
| 10.2 | 预制场景 | game/world/prefab.go + scene.go | P2 | S33/W4 | 10.1＋1.3a视口 | Prefab/Scene{Load,Save}，认场景JSON | game新包+认预制场景文件 | 测scene_test；窗game_world--case=open | A 摆好装出来对，B 缺文件报错不崩，C 两边齐，D 大场景加载时长有数，E 反复进出不涨，F 真窗看开局 | 已完成·V1口径（2026-09-15；game/world/scene_test.go 6项全PASS＋world整包18项PASS＋go vet净＋CGO_ENABLED=0可构建；A4实体开局slime世界[32,42]对；B缺截断错爹重名错版分清；D2000实体解析约10.7ms开局约1.9ms；E200轮进出回基线；F离屏 scene_open.json逐字节一致；金数据scene_cases.json等8个；有意偏离SceneFile避S34重名用户已确认；窗意向game_world--case=open随P2建；前置S25复核绿） |
| 10.3 | 生命周期 | game/world/scene.go | P2 | S34/W4 | 10.1 | Spawn/Sleep/Dispose，清资源 | game新包只算数 | 测lifecycle_test＋离屏；窗免 | A 生灭休眠对，B 重复销毁不崩，C 不适用，D 万次生灭耗时有数，E 跨关不漏，F 离屏对比 | 已完成·V1口径（2026-09-15；game/world/lifecycle_test.go 6项全PASS＋world整包12项PASS＋go vet净＋CGO_ENABLED=0可构建；D万次约3.26ms约325.8ns/次；E60关x40暂存回基线发号只涨；C双建逐位一致；F离屏金lifecycle_cases.json活3/激活1/休眠2；窗免；前置S25复核绿） |
| 11.1 | 逻辑步长 | game/step/fixed.go（同8.1） | P0 | S03/W1 | 同8.1 | 同8.1合测 | game新包只算数，同8.1合测 | 同8.1合测 | 同 8.1，合测 | 已完成·V1口径（2026-09-15；同8.1一次开工一次合测，game/step/fixed_test.go 6项全PASS，窗免-纯算数） |
| 11.2 | 时间倍率暂停 | game/step/time.go | P0 | S18/W2 | core＋8.1 | TimeScale＋Pause，分层时间 | game新包只算数 | 测time_test；窗game_step--case=pause | A 暂停慢放加速对，B 倍率零负钳住，C 不适用，D 切换耗时有数，E 长跑不漂，F 真窗看暂停 | 已完成·V1口径（2026-09-15；game/step/time_test.go 6项全PASS＋CGO_ENABLED=0可构建＋go vet净；D八万次切换约15.2ns/op＋万次Split约14.2ns/op；E十万帧双重放一致暂停世界恒0；C纯算数以双重放逐位一致即两边同数；F离屏金比对time_cases.json分流17/钳位9/序列4冻结；窗免-W2纯算数，真窗game_step--case=pause随P2建；前置core＋S03复核绿） |
| 12.1 | 资产管理依赖版本 | game/asset/asset.go | P2长链 | S19/W2 | 0.0＋3.1 | Asset{Load,Ref,Unload}＋依赖版本 | game新包后台链 | 测asset_test＋离屏；窗免 | A 异步引用计数依赖跟踪版本哈希对，B 缺资产占位不崩，C 两边齐，D 百资产内存有数，E 反复加载不涨，F 离屏对比 | 已完成·V1口径（2026-09-15；game/asset/asset_test.go 6项全PASS＋CGO_ENABLED=0可构建＋go vet净；D百资产5200B约3.0us/次Count对总字节对；E千次Load/Unload逐位一致字节不动坏档可盖；C同文件双管家重放逐位一致即两边同数；F离屏金比对asset_cases.json总账＋双KTX2＋双raw＋坏截断冻结；窗免-纯管线；前置0.0＋S02复核绿，compressed.go本体未碰） |
| 12.2 | 图集打包 | game/asset/atlaspack.go | P2 | S27/W3 | 12.1 | AtlasPack工具，拼图出JSON | 离线工具+game包 | 测atlaspack_test＋离屏；窗免 | A 拼图号对，B 超大图报错，C 画出来差在容差内，D 打包时间有数，E 重复打包稳定，F 离屏对比 | 已完成·V1口径（2026-09-15；game/asset/atlaspack_test.go 6项全PASS＋asset老6项回归绿＋CGO_ENABLED=0可构建；D64图50遍约106.6us/pack Encode 6341B进256x86；E500遍逐位一致；C容差0下maxDiff0 bad0/544缝边全透明；F离屏金比对atlas_cases.json+atlas_small.json area544 sheet27x27 util74.6%；窗免-纯打包；前置S19复核绿） |
| 12.3 | 热重载 | game/asset/hotreload.go | P2 | S28/W3 | 12.1 | Watch＋Reload，只换那块 | game新包文件监听 | 测hotreload_test；窗game_asset--case=reload | A 一改就换，B 坏文件不崩，C 两边齐，D 重载耗时有数，E 反复改不涨，F 真窗看改完即看 | 已完成·V1口径（2026-09-15；game/asset/hotreload_test.go 6项全PASS＋asset整包+core+atlaspack回归绿＋CGO_ENABLED=0可构建＋go vet净；D2000次轮询约14.2us/次、136B换图约38us；E百次来回字节恒定引用不动；C看门人与直装逐位一致；F离屏金比对hotreload_cases.json；窗意向game_asset--case=reload随P2建；前置S19复核绿） |
| 13.1 | 动作映射改键死区震动 | game/input/action.go | P0 | S07/W1 | 0.0 | Action＋改键＋死区震动 | game新包只算数 | 测action_test；窗game_input--case=remap | A 动作触多键死区震动手势对，B 空绑不崩，C 不适用，D 百次输入耗时有数，E 长跑不丢漂移可调，F 真窗看改键摇杆不漂 | 已完成·V1口径（2026-09-15；game/input/action_test.go 6项全PASS＋go vet净＋CGO_ENABLED=0可构建；A19快照4向量全对（键/任意柄/摇杆半偏0.375/捏放0.125/ Mean 全死区重映射改键＋死区重调＋震动时序）；B空绑静默＋坏动作/坏绑定10类/坏键坏震动invalid-arg＋NaN/Inf/零负不改旧值＋nil接收器不崩；D20000次x10动作22万求值约225.6ns/op；E万次漂移门住＋死区0放行0.1＋万次捏合钳1重放一致＋百步震动双重放一致＋重置清零留注册；C不适用-纯算数以复算逐位一致＋绑定拷贝隔离即两边同数；F离屏金比对action_cases.json 19快照4向量改键3探针重调2探针冻结＋数字满格/漂移门住/单侧互斥/捏放分向/对角归一形状断言；窗免-W1纯算数，输入窗game_input后建；前置0.0复核绿core全PASS；只动game/input新包render未碰） |
| 13.2 | 输入缓冲多点 | game/input/buffer.go | P0 | S23/W2 | 13.1 | Buffer＋多点跟踪 | game新包只算数 | 测buffer_test；窗game_input--case=combo | A 连招缓存多点跟踪对，B 断触不崩，C 不适用，D 压力输入有数，E 长跑不乱，F 真窗看连招 | 已完成·V1口径（2026-09-15；game/input/buffer_test.go 6项全PASS＋CGO_ENABLED=0可构建＋go vet净；D五千轮十六万次约58.1ns/op命中全中＋触摸五千轮约76ns/op终态归零；E万次双重放一致满64不涨终态0；C纯算数以双建重放逐位一致即两边同数；F离屏金比对buffer_cases.json限额64/10/150ms冻结＋过期未来剪枝幸存者断言；窗免-W2纯算数，真窗game_input--case=combo随P2建；前置S07复核绿，action.go语义未碰） |
| 14.1a | 碰撞体纯算 | game/physics/body.go | P0 | S06/W1 | 0.0 | Body＋层分组＋触发器 | game新包只算数 | 测body_test＋离屏；窗免 | A撞和触发对，B同位不崩，D百盒耗时有数，E长跑不穿 | 已完成·V1口径（2026-09-15；game/physics/body_test.go 6项全PASS＋go vet净＋CGO_ENABLED=0可构建；A11组相交＋4组掩码＋4组查询全对（边触算碰触发标记层过滤静默）；B空nil单体零点同位不崩＋坏构造invalid-arg＋坏体Query错nil不改输入；D百盒2000次约351.1us/query；E万次重放逐位一致＋实体触发两路走格往返不粘；C不适用像素以边界往返无损＋重放一致即两边同数；F离屏金比对body_cases.json 18体11交4掩码4查询冻结＋形状断言；窗免-W1纯算数，碰撞窗game_physics后建；前置0.0复核绿core 16项全PASS；只动game/physics新包render未碰） |
| 14.1b | 射线移动平台画出 | game/physics/body.go | P2 | S21/W2 | 14.1a＋7.1 | Ray＋移动平台带着走 | game新包只算数 | 测body_ray_test；窗game_physics--case=hit | A射线平台对，B同位不崩，D百盒耗时有数，E长跑不穿，F真窗看探头子弹线 | 已完成·V1口径（2026-09-15；game/physics/body_ray_test.go 6项全PASS＋同包body 6项＋platform 6项整包18项全PASS＋go vet净＋CGO_ENABLED=0可构建；D二千次x百盒约9.9us/cast命中全中；E万次逐位一致上下车往返翻转对万步链不断触；C纯算数以边界往返无损＋逐位重放即两边同数；F离屏金比对body_ray_cases.json13体13线14casts7站3携冻结＋边触中/体内0/掩码静默/换序取近断言；窗免-W2纯算数，真窗game_physics--case=hit随P2建；前置S06＋S05复核绿，14.1a语义只加未改） |
| 14.2 | 平台斜坡 | game/physics/platform.go | P2 | S22/W2 | 14.1a＋7.1 | Slope＋单向平台，站滑跳 | game新包只算数 | 测platform_test；窗game_physics--case=jump | A 站滑跳单向上跳对，B 卡角不崩，C 不适用，D 长坡耗时有数，E 长跑不掉，F 真窗看跳 | 已完成·V1口径（2026-09-15；game/physics/platform_test.go 6项全PASS＋同包整包18项全PASS＋go vet净＋CGO_ENABLED=0可构建；D五千高＋二千步约0.3us/op落48和360906.2冻结；E万次逐位一致往返不粘单向穿过再落住；C纯算数以边界往返无损＋逐位重放即两边同数；F离屏金比对platform_cases.json六坡十八高六角八站六滑二脚七贴十一单步冻结＋端点进/半格外/缓站陡滑/Y恒正断言；窗免-W2纯算数，真窗game_physics--case=jump随P2建；前置S06＋S05复核绿；收敛：同包helper重名改platSameFloat，逻辑金数据零改） |
| 15.1 | 位置音 | game/audio/positional.go | P0 | S13/W1 | 0.0 | PosSound{pos,range}，远小近大 | game新包独立音频链 | 测positional_test；人工听 | A 远近左右对，B 无声不崩，C 不适用，D 多声 CPU 有数，E 长跑不爆，F 人工听 | 已完成·V1口径（2026-09-15；game/audio/positional_test.go 6项全PASS＋go vet净＋CGO_ENABLED=0可构建；A16混音10立体声全对（中点满格/半程半响/边缘静默/对角/二次衰减/零衰减满响/零范围永响/单声道/强声像/头顶居中/偏置听者）；B坏构造坏听者撕裂结构invalid-arg占位静默＋MixAll坏槽不哑全场＋Stereo坏数归零＋1e308巨距量远静默不爆响；D256声x2000次约23.5us/rep；E万次重放逐位一致＋走近走远往返＋满刻度5000次不爆；C不适用-纯算数以边界往返无损＋逐位重放即两边同数；F离屏金比对positional_cases.json 16混音10立体声冻结＋近响远轻/左右镜像/衰减快慢/波形限幅形状断言＋满刻度不削波；窗免-纯算数＋人工听说明（中置居中/右偏右/远处微弱可定位）；前置0.0复核绿core全PASS；只动game/audio新包render未碰） |
| 15.2 | 音乐混音总线闪避 | game/audio/music.go | P0 | S14/W1 | 0.0 | Music＋Bus＋Duck，爆炸压音乐 | game新包独立音频链 | 测music_bus_test；人工听 | A 切换淡入淡出总线闪避对，B 缺曲不崩，C 不适用，D 混音上限有数，E 长跑不爆，F 人工听爆炸压音乐 | 已完成·V1口径（2026-09-15；game/audio/music_bus_test.go 6项全PASS＋go vet净＋CGO_ENABLED=0可构建；A15变换5总线7闪避7混音5叠乘全对（1s交叉0/0.25/0.5/0.75/1单调和为1＋淡入淡出＋推2层弹回1层＋同曲重定无缝＋接线xfade0.5*typical0.8*hold0.5=ducked0.2）；B空曲invalid-arg留栈＋空弹not-found＋超8层out-of-memory＋坏音量/上限/深度/强度invalid-arg保旧值＋nil静默＋坏声忽略输入不改＋叠乘NaN归零超限幅1；D256声x2000次约27.6us/rep留最响32限幅；E万次复读一致＋百步淡入淡出单调收敛0/1＋闪避1000ms回1＋5000次满刻度不削波；C不适用-纯算数以双建重放＋输入不改＋逐位重放即两边同数；F离屏金比对music_bus_cases.json冻结＋形状断言＋满刻度波形不削波；窗免-纯算数＋人工听说明（交叉无咔哒/爆炸压半响可懂0.5s爬回/静音音乐走hits留）；前置0.0复核绿core全PASS＋同包positional 6项全PASS；只动game/audio新包（music.go/doc.go/单测/金数据）render未碰；收敛2026-09-15：淡入淡出收拢settleFading/armFade＋限幅收拢clampGain复用（删clamp01）＋Blend空栈分支压平＋闪避死分支去release<=0＋混音满员免排序加Slice去Stable＋单测手写itoa改strconv＋离屏自比改双建重放＋长跑万次重建改同体复读＋边界死断言去台账/探针，6项仍全PASS，金数据未动） |
| 16.1 | 存档槽位迁移 | game/save/save.go | P0 | S15/W1 | 0.0 | Save{slot,ver,Migrate}，坏不崩 | game新包文件存读 | 测save_test＋离屏；窗免 | A 存读槽位版本迁移对，B 坏档不崩，C 不适用，D 大档时长有数，E 反复存读不坏，F 离屏对比 | 已完成·V1口径（2026-09-15；game/save/save_test.go 6项全PASS＋go vet净＋CGO_ENABLED=0可构建；A三档文件加API重建加编解码往返加迁后一致全对（0空新局/1中断/2满贯）＋旧0.9迁1.0只升版数不动；B空零超限五坏档错码分清（bad-data/version-mismatch）＋坏构造invalid-arg＋nil接收器不崩；D 256物64星200次约473us/rep 8773B远小于MaxBytes；E千次编解码逐位一致＋星升降往返不粘＋200次存读同字节；C不适用-纯文件以ms边界无损＋逐位重放即两边同数；F离屏金比对save_cases.json三档冻结＋槽 distinct/新局空/满贯单调/预算内形状断言；窗免-纯文件；前置0.0复核绿core全PASS＋camera/sprite/tex/tilemap/anim/audio/input/physics/step回归绿；只动game/save新包render未碰） |
| 16.2 | 画质分档 | game/save/quality.go | P4 | S51/W9 | 17.2 | Quality{高/中/低}跟档走 | game新包跟档走 | 测quality_test；窗game_save--case=q123 | A 高中低跟档走，B 切档不闪崩，C 两边齐，D 各档帧率有数，E 长跑不掉档，F 真窗看三档 | 已完成·V1口径（2026-09-16；game/save/quality_test.go 6项全PASS＋老save_test 6项回归绿＋前置debug 6项复核绿＋core整包绿＋go vet净＋gofmt净＋CGO_ENABLED=0可构建；A三档高1000粒子8灯1.00/中500/4/0.75/低200/2/0.50单调递减文件与API一致；B 900次轮切回起点一致＋同档切空操作＋8类坏名invalid-arg保旧档＋4类坏文件错码分清；D SpecFor约16~24ns/op＋Switch约7~8ns/对＋编解码34B约3us/次；E万次编解码一致＋万次轮切跟手＋200次存读同字节；C不适用-纯映射数一致像素差归各管；F自动窗EXIT0 presents464 switches3 errors0 fps58/59/60 golden0 dots4000/2000/800 bars1024/768/512；主会话复跑save12项PASS＋debug PASS＋自动窗PASS；只动quality新文件＋examples/game_save老save三件未碰render/ui未碰；人工常驻2026-09-16用户28秒点588次收JSON（backend=x11 presents1672 switches14 errors0 continuous1 probe1 fps约59/59.5/59.5 frames1482/1371/985，用户未报闪与分不清问题）；主会话复跑save12项PASS＋debug PASS＋自动窗PASS；只动quality新文件＋examples/game_save老save三件未碰render/ui未碰；环境940MX/580.178.04/1920x1080/DISPLAY=:0非release换机重测；W9关） |
| 17.1 | 编辑器预览 | tools/levelprev + particleprev | P4 | S50/W8 | W7单项全过 | 关卡/粒子预览，所见即所得 | tools新工具 | 人工看预览；窗tools预览 | A 摆关调粒子所见即所得，B 空工程不崩，C 两边齐，D 大关卡打开时长有数，E 反复开不涨，F 人工看 | 已完成·V1口径（2026-09-16；tools/levelprev/preview_test.go 6项全PASS＋tools/particleprev/preview_test.go 4项全PASS＋go vet净＋gofmt净＋CGO_ENABLED=0可构建；A关卡小17瓦片3实体/大14627瓦片2000实体按层设色对象黄框实体红点＋粒子火cone16/16/0烟box15/15/0走真引擎即调即看；B空工程2notes占位照开窗＋坏图保3实体坏场景保17瓦片＋缺特效占位坏特效BadData；C只比数VerifyCounts/VerifyReplay与引擎直解零差＋同工程离屏重画逐字节一致＋三金第二遍changed=0；D大关卡探针约13~18ms窗内重开约18ms＋小工程fps约59；E大工程连开5次零漂移＋粒子连开20次零漂移＋整窗5次maxRSS走低无增长；F自动三窗EXIT0（level小presents472 fps59 p95约17 hitch2 golden0、fire presents474 alive54、smoke presents474 alive51）＋离屏金level_preview_golden.png＋fire/smoke双金；人工常驻2026-09-16用户三窗收JSON全backend=x11探针1：level presents107 ptr49 tiles17 entities3、火 presents438 alive256 spawned512 rate15360 key10 ptr109、烟 presents709 alive256 spawned2427 key10 ptr160，用户亲眼静态方块系布局预览/火锥往上窜/烟盒乱流飘三样全对＋加减倍率即看（rate按到15360活数顶满256符合Max设计）；主会话复跑双包PASS＋三窗自动PASS＋tilemap/world/particle/core回归绿；只动tools两目录game只读render/ui未碰；*_last.png运行快照不入库已删＋proj_empty加.keep；环境940MX/580.178.04/1920x1080/DISPLAY=:0非release换机重测；W8关） |
| 17.2 | 性能崩溃帧分解 | game/debug/stats.go | P4 | S49/W8 | W7单项全过 | Stats{帧/次/存/分解}＋上报 | game新包只出数 | 人工看报表＋长跑在线 | A 帧率画次显存帧分解过绘编译耗时出数对，B 无数据不崩，C 不适用，D 上报耗时有数，E 长跑在线，F 人工看报表定位到谁吃时间 | 已完成·V1口径（2026-09-16；game/debug/stats_test.go 6项全PASS＋go vet净＋gofmt净＋CGO_ENABLED=0可构建；A 10帧冻结fps47.62 avg21ms p95 50ms slow2 画次1440 过绘1.395x 显存65/67MB 分解render114ms占53.3%居首 着色器177ms全对＋编解码往返一致；B nil零值/空零超限/坏文件错码分清坏帧不污染台账；D上报encode+parse约35.1us/次624B远小于1MB＋shader约55.7ns/次＋存读约261us；E 5000帧环只留3600总数照计显存钉住＋10秒后台钳250ms记1gap＋超512MB记1OOM旧值不动＋50次存读同字节；C不适用-纯算数以毫秒边界无损＋版本往返＋双建逐位一致即两边同数；F离屏金比对stats_cases.json/stats_report.json逐字节一致＋stats_report_golden.txt逐字节对＋stats_crash.json原因码对＋形状断言render居首；窗免-报表＋长跑即证据（stats_report_golden.txt人看＋stats_report.json机器读）；前置P3复核绿particle12项＋fx18项＋light18项＋step24项＋core16项；只动game/debug新包render/ui/tools未碰；环境940MX/580.178.04/1920x1080/DISPLAY=:0非release纯算数与release无关换机重测） |

规矩：这张表是唯一状态源，做完一项就把那行改成已完成，写清日期、测试文件、真窗名。口头说完不算。 V1行分到G01–G11，详情见 `docs/2.5D/G01.md`、`docs/2.5D/G02.md`、`docs/2.5D/G03.md`、`docs/2.5D/G04.md`、`docs/2.5D/G05.md`、`docs/2.5D/G06.md`、`docs/2.5D/G07.md`、`docs/2.5D/G08.md`、`docs/2.5D/G09.md`、`docs/2.5D/G10.md`、`docs/2.5D/G11.md`。

## 单能力五步与回归红线（做到哪算完）

开工以能力为单位，一个能力走完五步才许标已完成：

```text
1 接口：doc.go冻住结构体加函数，评审过。
2 单测：*_test.go单跑过，A/B/D/E先绿，数据进testdata。
3 接线：指标接上，C两边齐或记不适用，坏路不崩。
4 窗口：独立真窗先自动判留JSON，再人工看；纯算数只留离屏对比。
5 回归：本包全测加关联真窗重跑，界面金图性能基线不红。
```

回归红线（碰render/ui/scene必须跑，不过不许合）：

```text
- 界面金图：kit/showcase类对比图不花，花的先查代码不许直接更新图。
- 性能基线：scheduler/raster/embedder基线不掉，掉先定位谁吃时间。
- 显存清理：oom_purge链不断，反复加载释放不涨。
- 回滚：红了先回滚游戏层新分支，render主路不动。
```

## 验收怎么算过（已并入§5，新人只看§5；本节只留一句话）

一句话：数没写死、环境没锁、跑法没定、图没管住、坏路没测，都不算过。细数（每波总数、每点单数）看各分期天花板拆分节，开工前先看那节。

## 窗口清单（名字先冻，窗没建不许标绿）

一句话：V1已建9窗（game_anim/game_fx/game_light/game_particle/game_quad/game_save/game_sprite/game_step/game_stage_chase），未建7窗（game_camera/game_tex/game_tilemap/game_world/game_asset/game_input/game_physics，W11补）。空目录不算数，主能力必须独立窗，组合窗不许顶单能力。本节是V1建窗原表，V2新窗见V2真窗表。

套路照 V2 真窗怎么做那节（wrkit 搭壳＋wrgate 判门＋wrsoak 探针，两步自动加人工），不重造。

| 窗 | 验哪个能力 | 自动判啥 | 人工看啥 | 状态（2026-09-16） |
|---|---|---|---|---|
| `examples/game_camera` | 1.3镜头跟随震动视差限位平滑 | 跟随不飘、限位钳住、长路无缝JSON | 推镜头跟，抖不晕 | 未建，W11的S55补 |
| `examples/game_quad` | 1.4梯形 | 梯形两边齐JSON | 梯形不变方块 | 已建 |
| `examples/game_sprite` | 2.1合批、2.2图集、2.3排序、2.4序列帧、1.2遮挡（5个case） | 千树帧率、转染色对、盖对、切换对JSON | 树多不卡，转身盖对 | 已建（--case=anim待S74补） |
| `examples/game_tex` | 3.2防闪、3.3后台加载 | 远树不抖、边走边出JSON | 远近都清 | 未建，W11的S56补 |
| `examples/game_anim` | 4.2时间轴、4.3骨头、4.4状态机 | 事件准时、真资源不错位、切换不卡JSON | 动作顺，切换脆 | 已建（--case=tl待S75补） |
| `examples/game_particle` | 5.1发射、5.2拖尾 | 千粒子帧率、刀光宽窄JSON | 火像火，刀有锋 | 已建 |
| `examples/game_fx` | 5.3扭曲、5.4后期、5.5钩子 | 全屏帧率、气氛JSON | 水晃光晕对味 | 已建 |
| `examples/game_light` | 6.1灯、6.2法线、6.3影子 | 手电只照人、影子对JSON | 夜里看得清 | 已建 |
| `examples/game_tilemap` | 7.1瓦片、7.2分区、7.3远近 | 摆怪对、边走边装、切换不闪JSON | 地图大走不丢 | 未建，W11的S57补 |
| `examples/game_step` | 8.3动态更新、11.2暂停 | 动哪更哪、暂停即停JSON | 追车顺，暂停灵 | 已建dirty（--case=pause待补） |
| `examples/game_world` | 10.2预制场景 | 开局装对JSON | 开局齐 | 未建，W11的S58补 |
| `examples/game_asset` | 12.3热重载 | 一改即换JSON | 改完即看 | 未建，W11的S59补 |
| `examples/game_input` | 13.1改键、13.2连招 | 改键即用、连招不丢JSON | 按着顺手 | 未建，W11的S60补 |
| `examples/game_physics` | 14.1碰撞射线、14.2平台 | 撞门准、跳得上JSON | 跳不穿，落得住 | 未建，W11的S61补 |
| `examples/game_save` | 16.2三档 | 三档帧率JSON | 切档不闪 | 已建 |
| `examples/game_dodge` | 躲避小怪P1关（S78） | 120秒自动＋人工2分钟JSON | 和原版手感一致 | 未建，W13的S78建 |
| 音频15.1/15.2、纯算数（1.1/4.1/8.1/8.2/9.2/10.1/10.3/11.1/12.1/12.2/16.1）、3.1压缩 | 窗免 | 只留离屏对比加人工听（音频） | — |

## 组合大窗（单项全绿合起来红，最贵；数全在 S52/S67 卡里，本节只留结论）

追车关`examples/game_stage_chase`：2026-09-16 实测 120 秒 hitch25（统一超 20ms 口径约 12.5 次/分），超每分 3 次线，P3 组合未关门，留 W10 的 S52 回炉（减分配＋闲时 3 遍取最差）。夜战第二关等追车绿了再定，见 S68。

## 长跑接谁（不重造；架子照抄`examples/wrsoak/soak.go`＋`examples/ui_wr_r15_soak/main.go`，数进`wrgate.BuildReport`同一套 JSON，判线见§5）

## 门禁挂开工波（已并入上一节分期关门条件表＋§5；原P分期只做归属分类，不挂门禁）

每波跑啥、谁判，看上一节分期关门条件表；怎么算过，看§5。R1/R2/R3/R4/R5 小修回归界面金图那几行，关门证据不变。

红了谁负责：游戏层红改game/tools，render主路红先回滚游戏分支，不动主路。

## 不在这份文档里做的（范围以§0非目标为准，明细看 V2 不含清单 N1–N8＋V3 平台口子；只剩两条线外事）

- 具体某个示例的画法、美术资源、关卡设计。
- 窗口、输入法、文字排版本身的问题，那是别的文档管的。

## V2：工业级立体细腻版（需求定稿＋开发计划，2026-09-16）

一句话：V1 是把架子搭对（算数绿），V2 是把东西做像（眼睛绿）。框架目标你定，里面细节我定，你只拍行不行。
框架目标：小场景里木箱像木箱、人像人、石头像石头，远看有前后，近看有鼓包，放大不糊，暗部不死黑。不到照片封顶，但一眼能认。

### V2 需求定稿（在 V1 基础上补什么）

V1 现状一句话（2026-09-16 实查）：W0–W9 账面已绿，但证据全是单台 940MX 非正式包，追车 hitch 超线 P3 未关门，7 个窗未建。细数见 V3 规模对照表，开工 V3 前先看那张。

V2 细节我定死（7 块）：

1. 样板定死（拿什么验像不像）：
- 木箱 40 厘米见方，木纹看清，边角磨损；人物 1 米 7，脸和衣服分开，手电一照脸亮衣稍暗；石头半米，鼓包最高比边上亮一半以上；小场景把三样加一棵树一面墙摆一起，前后盖对。
- 每样三张图配齐：底色管颜色，法线管鼓包方向，粗糙管亮不亮（木头哑光、铁件亮、皮肤中间）。小对比图不用 6 千像素那种小的，直接按窗大小验。
2. 灯影定死：环境底色管夜里不黑死，方向光管白天方向，点光做手电，亮度能超 1 进光晕但不曝成一片。影子柔边用三档里中间那档，大灯按盖多少像素收费，带阴影真灯不超 10 盏。
3. 细腻定死：资源按 1080p 备，缩小斜看默认开多级渐远加三线性，斜 45 度地面开 4 倍各向异性，图集留白 2–4 像素、扩边 1–2，像素风才用最近邻，写实全用线性。像素差超 1% 打回，梯形变方块、渐变变纯色直接打回。
4. 颜色定死：先在线性里算对，最后一步再转屏幕。8 位主路保流畅，亮暗都留得住做成可选项（9.1 部分解冻），CPU 跑不动明确标降级，不许偷偷变灰。
5. 压缩定死：至少认两种，底图用高压缩，人物法线用高质量，头不对直接拒收给原因，不硬读。转码按显卡走（本线只做桌面三端，安卓/苹果系只当格式知识备查，不开工），转坏给粉块兜底不闪退。
6. 性能定死（帧门见§5，加 V2 量）：正式包，量见§5起步行，V2 量为：静态合批精灵数千级，动态骨骼上百，CPU 粒子千级内，GPU 粒子万级（大型档见规模对照一节）。
7. 养活定死：图集 2048 或 4096，改一张图只重载该图集，断链报哪条链。关卡和粒子预览所见即所得，卡了能录一帧定到谁吃时间。平台写死跑哪几端，集显独显各测一遍。联机脚本先拍板再干，首版只留口（定点数加确定性随机加帧号加快照，脚本随包走不下发）。

对标口径（只抄做法，不搬代码）：镜头视差排序状态机粒子拖尾瓦片骨头灯影抄 Godot 4 参数名和默认值；精灵四模式加淡入 0.2–0.3 秒加骨骼 JSON 最低子集加地图子集加压缩头抄 Cocos/Spine/TMX/Basis/KTX2/ASTC；全局透视加合批加滤镜顺序（先模糊后染色）加固定步长加渲染插值抄 Skia/Fix Your Timestep。

### Godot 全量参考清单（2.5D 完整引擎照抄目录，2026-09-16 实盘本地 godot 只读）

一句话：Godot 官网 2D 文档列 15 件，本地源码实盘 40＋文件，全列出来，一个不漏对应到开发计划。玩法层 Godot 是唯一实现参考，底层渲染只用自家 render/。

玩法 13 件（scene/2d＋scene/animation＋scene/resources）：

| # | Godot 叫什么 | 本地源码在哪 | 管什么＋抄哪几个函数 | 对我们哪包 | 落哪个 S |
|---|---|---|---|---|---|
| G01 | CanvasItem（所有 2D 的爹） | scene/main/canvas_item.h | 显隐混色材质重画：queue_redraw、z_index/z_as_relative/y_sort_enabled、texture_filter/texture_repeat、set_material、get_canvas_transform | game/sprite＋game/world 分层口径 | 已在S10/S25落实，无新窗（分层规矩，不另开S） |
| G02 | Camera2D | scene/2d/camera_2d.h | 看向缩放跟随：get_camera_transform、zoom、limit 四边＋limit_smoothing、drag_margin＋drag 开关、position_smoothing＋speed、reset_smoothing；AnchorMode 两档 | game/camera/camera.go | S55/W11 新窗 |
| G03 | ParallaxBackground/ParallaxLayer/Parallax2D | scene/2d/parallax_background.h＋parallax_layer.h＋parallax_2d.h | 远慢近快：scroll_scale、repeat_size/times、autoscroll、limit_begin/end、follow_viewport | game/camera/parallax.go | S55/W11 同窗 |
| G04 | Sprite2D | scene/2d/sprite_2d.h | 贴图：set_texture、region 切图＋防串色、hframes/vframes/frame 切帧、flip、is_pixel_opaque 点选 | game/sprite/atlas.go＋batch.go | S31/S36 已绿，W10 正式重测 |
| G05 | AnimatedSprite2D＋SpriteFrames | scene/2d/animated_sprite_2d.h＋scene/resources/sprite_frames.h | 帧库＋播放器分离：play/backwards/pause/stop、animation/frame/speed_scale、animation_finished/looped 信号、add_animation/get_frame_duration、LOOP_NONE/LINEAR/PINGPONG、MINIMUM_DURATION | game/sprite/flipbook.go | 补 S74/W11（--case=anim 新窗，原计划漏排） |
| G06 | AnimationPlayer＋AnimationMixer | scene/animation/animation_player.h＋animation_mixer.h | 单轨＋混音总线：play/queue/seek/play_section、blend_time/default_blend_time、capture 抢拍防跳、advance/_blend 四步管线 | game/anim/timeline.go | 补 S75/W11（--case=tl 新窗，原计划漏排） |
| G07 | AnimationTree（含 Blend2/3、BlendSpace1D/2D、StateMachine） | scene/animation/animation_tree.h＋animation_blend_tree.h＋animation_blend_space_1d/2d.h＋animation_node_state_machine.h | 节点图求值：tree_root、blend_position、travel/start、switch_mode 三档、xfade_time、priority、add_blend_point/add_triangle/auto_triangles、MAX_BLEND_POINTS=64 | game/anim/statemachine.go | S46 已绿，W10 S53 单窗重测 |
| G08 | Tween（程序化补间） | scene/animation/tween.h | 代码写动画：tween_property/callback/method/interval、set_trans/set_ease/set_loops、TransitionType/EaseType 表 | game/anim/easing.go | S04已绿＋S78复用，无新S |
| G09 | CPUParticles2D | scene/2d/cpu_particles_2d.h | 小量精确：amount/lifetime/explosiveness/randomness、direction/spread/gravity/初速/阻尼、color＋ramp、发射形状 point/sphere/rect/ring、draw_order | game/particle/emitter.go＋cpu.go | S38 已绿，W10 正式重测 |
| G10 | GPUParticles2D＋ParticleProcessMaterial | scene/2d/gpu_particles_2d.h＋servers 粒子存储 | 大量＋数量行为分离：amount/amount_ratio（改比例不重启）、fixed_fps＋interpolate、preprocess 预热、visibility_rect 裁剪、trail 开关时长段数、sub_emitter 子发射、seed 可复现 | game/particle/gpu.go＋trail.go | S43已绿；V2万级量在S43复测补数，不新开S |
| G11 | Line2D（拖尾 ribbon 同源） | scene/2d/line_2d.h（＋line_builder.h） | 可变宽折线：points、width＋width_curve、gradient、joint 三档＋sharp_limit、begin/end_cap、closed、texture tile/stretch、antialiased（掉合批要标） | game/particle/trail.go | S43 已绿，W10 正式复测 |
| G12 | TileMapLayer＋TileSet（新） | modules/tilemap/tile_map_layer.h＋tile_set.h（注：本快照 scene/resources/2d/tile_set 缺文件，以 modules 为准） | 三段寻址＋分块合批：source_id/atlas_coords/alternative、set_cell/get_cell、rendering_quadrant_size=16（256 块一批）、physics_quadrant 16 合并、地形自动拼（只编辑器跑，运行时只按 gid 画） | game/tilemap 四文件 | S57/W11 新建三 case |
| G13 | Skeleton2D＋Bone2D＋ModificationStack（IK/抖动/物理骨） | scene/2d/skeleton_2d.h＋scene/resources/2d/skeleton/各 modification 头 | 骨皮分离：rest/apply_rest、local_pose_override＋strength、modification_stack、TwoBoneIK/CCDIK/FABRIK/LookAt 按需抄一个、EXECUTION_MODE 时机 | game/anim/skeleton.go | S42 已绿，W10 S53 重测＋W12 S62 粗糙 |

灯影 3 件：

| # | Godot 叫什么 | 本地源码在哪 | 抄哪几个 | 对我们哪包 | 落哪个 S |
|---|---|---|---|---|---|
| G14 | PointLight2D/DirectionalLight2D | scene/2d/light_2d.h | color/energy（可超 1）/height、z_range/layer_range、item_cull/shadow_cull_mask、shadow_enabled/color/filter（NONE/PCF5/PCF13）；BlendMode ADD/SUB/MIX | game/light/light.go | S45 已绿，W12 S63 柔边收费 |
| G15 | LightOccluder2D＋OccluderPolygon2D | scene/2d/light_occluder_2d.h | polygon/closed、cull_mode 三档、occluder_light_mask、as_sdf_collision | game/light/shadow.go | S48 已绿，W10 S53 重测 |
| G16 | CanvasModulate＋CanvasTexture | scene 等（环境＋纹理头） | 全场底色线性先乘；diffuse/normal/specular＋shininess | game/light/light.go Night＋normal.go | S45/S47，W12 S66 可选管线 |

渲染 6 件（servers/rendering，只抄口径，底层仍用自家 render/）：

| # | Godot 叫什么 | 本地源码在哪 | 抄什么 | 对我们哪处 | 落哪个 S |
|---|---|---|---|---|---|
| G17 | RendererCanvasCull 可见排序 | servers/rendering/renderer_canvas_cull.h | canvas_item_set_parent/z_index、ItemYSort、ysort_xform/index、_item_queue_update | game/sprite/ysort.go＋depth.go 口径 | S10/S41 已绿 |
| G18 | RendererCanvasRender 命令口 | servers/rendering/renderer_canvas_render.h | add_rect/primitive/polygon/particles、CanvasRectFlags（REGION/TILE/FLIP/CLIP_UV）、Light 结构、texture_filter/repeat 默认值 | render 新分支口径 | R3/R4/R5 已绿 |
| G19 | RendererCanvasRenderRD 合批真身 | servers/rendering/renderer_rd/renderer_canvas_render_rd.h＋.cpp | 先按 clip→material→command 断批再一缓冲多实例：_record_item_commands/_render_batch/_new_batch、Batch{start,instance_count,material,clip}、item_buffer_size | game/sprite/batch.go 思路 | S31 已绿 |
| G20 | CanvasItemMaterial 材质 key | scene/resources/canvas_item_material.h | BlendMode 六档（MIX/ADD/SUB/MUL/PREMULT/DISABLED）、LightMode 三档（NORMAL/UNSHADED/LIGHT_ONLY）、particles 翻书 h/v＋loop | game/fx/custom.go | S44 已绿 |
| G21 | 采样器＋格式＋着色器采样宏 | rendering_server_enums.h＋rendering_device.h＋samplers_inc.glsl＋canvas.glsl | TextureFilter 六档（NEAREST/LINEAR/双 MIPMAP＋各向异性）、Repeat 三档、SamplerFilter/RepeatMode、textureSampler 封装、mipmap_lod | game/tex/mipmap.go＋render 采样开关 | S32/S65，W12 S65 写死 |
| G22 | Image/CompressedTexture/TextureStorage/Environment 尾巴 | core/io/image.h＋scene/resources/compressed_texture.h＋texture_storage.h＋environment.h＋tone_mapper.h | Format（RGBA8/RF/RGBAH/DXT/ETC/BPTC/ASTC）、CompressMode/Source、generate_mipmaps、DataFormat（IMAGE/PNG/WEBP/BASIS）、use_hdr、ToneMapper 五档（LINEAR/REINHARDT/FILMIC/ACES/AGX）、glow 口径 | game/tex/compressed.go＋S66 可选管线 | S02 已绿（一），S64 第二种，S66 管线 |

物理 5 件（scene/2d/physics＋navigation，最值得抄 CharacterBody2D）：

| # | Godot 叫什么 | 本地源码在哪 | 抄哪几个 | 对我们哪包 | 落哪个 S |
|---|---|---|---|---|---|
| G23 | CollisionObject2D/Shape/Polygon | scene/2d/physics/collision_object/shape/polygon_2d.h | layer/mask、shape_owner_add_shape、one_way＋margin、BUILD_SOLIDS/SEGMENTS、凸分解 | game/physics/body.go | S06/S21 已绿，W11 S61 新窗 |
| G24 | CharacterBody2D（2.5D 必抄） | scene/2d/physics/character_body_2d.h | move_and_slide、apply_floor_snap、is_on_floor/wall/ceiling、floor_normal、floor_max_angle、snap_length、up_direction、MOTION_MODE 两档 | game/physics/platform.go 扩展对齐 | S22 已绿，对齐补 S76/W11（snap＋mode 口径） |
| G25 | Area2D 触发区 | scene/2d/physics/area_2d.h | overlapping_bodies/areas、gravity/damp 覆盖、priority、monitoring/monitorable、audio_bus_override | game/physics/body.go 触发口径 | S06已绿＋S78复用 |
| G26 | RayCast2D＋KinematicCollision2D | scene/2d/physics/ray_cast/kinematic_collision_2d.h | target_position、force_update、is_colliding、point/normal、travel/remainder、collider_velocity | game/physics/body.go ray | S21 已绿 |
| G27 | NavigationAgent/Region/Link/Obstacle＋NavigationPolygon | scene/2d/navigation/各头＋scene/resources/2d/navigation_polygon.h | target/next_path、desired_distance/max_speed、avoidance、bake_polygon、enter/travel_cost | game/nav 新包（S80已作废并入本项） | S87转正/W17新窗game_nav（V2关门不含，见§0.1与N1去向） |

音频 4 件（servers/audio＋scene/audio）：

| # | Godot 叫什么 | 本地源码在哪 | 抄哪几个 | 对我们哪包 | 落哪个 S |
|---|---|---|---|---|---|
| G28 | AudioServer 总线混音真源 | servers/audio/audio_server.h | playback_stream、bus volume/mute/solo/send、Bus/Channel 双层、generate_bus_layout | game/audio/music.go | S14 已绿 |
| G29 | AudioStreamPlayer2D 位置音 | scene/2d/audio_stream_player_2d.h | max_distance/attenuation/panning_strength/area_mask、_update_panning、_get_actual_bus | game/audio/positional.go | S13已绿＋S78复用 |
| G30 | AudioListener2D | scene/2d/audio_listener_2d.h | make_current 单例，跟相机走 | 同上扩展 | 补 S77/W11（听者跟相机口径） |
| G31 | AudioStream/Playback/重采样 | scene/resources/audio/audio_stream.h | instantiate_playback、mix/seek、三次插值重采样 | game/audio 底层口径（S13/S14已对齐） | 口径已对齐，无新S；验收只认人工听＋波形（曲目设备声压见S13/S14行） |

输入存档 4 件（core/input＋scene/resources）：

| # | Godot 叫什么 | 本地源码在哪 | 抄哪几个 | 对我们哪包 | 落哪个 S |
|---|---|---|---|---|---|
| G32 | InputMap＋Input 单例 | core/input/input_map.h＋input.h | add_action/action_add_event、deadzone、is_pressed/just_pressed/get_strength/get_axis/get_vector、ALL_DEVICES=-1、joy_vibration、键鼠触屏互顶 | game/input/action.go＋buffer.go | S07/S23 已绿，W11 S60 新窗 |
| G33 | SpriteFrames 资源容器 | scene/resources/sprite_frames.h | add_animation/frame、speed/loop 三档、MINIMUM_DURATION | 同 G05 | 补 S74 同 |
| G34 | PackedScene＋SceneState（存档主通道） | scene/resources/packed_scene.h | pack/instantiate、add_node/property/connection 三表 | game/world/scene.go＋prefab.go 口径对齐 | S33 已绿 |
| G35 | ResourceFormatText（tscn/tres 文本格式） | scene/resources/resource_format_text.h | load/save、FORMAT_VERSION=4、ext/int_resources 两表、依赖改名 | game/asset＋world 存盘口径 | 工具链对齐用，不新开 S |

补排 9 个号＋1作废（5口径窗归W11＋1小游戏关归W13＋1真2.5D窗归W11＋3扩展对照窗归W14，S80占位作废并入S87；波内可并行；S78等W11全窗故归W13）：
- S74 帧动画窗（等 flipbook，game_sprite--case=anim，G05/G33，归W11）。
- S75 时间轴窗（等 timeline，game_anim--case=tl，G06，归W11）。
- S76 角色体对齐（等 platform，snap＋MOTION_MODE 口径，G24，归W11）。
- S77 听者跟相机（等 positional，AudioListener2D 口径，G30，归W11）。
- S78 dodge 小游戏关（等 W11全窗含S79，照抄 dodge_the_creeps，G08/G25/G29/G32 直接用；真窗 examples/game_dodge 自动 120 秒＋人工 2 分钟，归W13）。
- S79 真2.5D基向量窗（等 S08/S09/S10，misc/2.5d，归W11）。
- S81 横版对照窗（等 W11全绿＋S76，照抄platformer，归W14）。
- S82 灯影对照窗（等 S63，照抄lights_and_shadows，归W14）。
- S83 粒子对照窗（等 S43，照抄particles，归W14）。

修订 2026-09-16（Godot 全量对齐）：G01–G35 全列入计划，S74–S77＋S79归W11、S78归W13，W11由7窗变12项（7窗＋5口径窗），S78为P1小游戏关（dodge，归W13，前置写死等W11关门），S79为真2.5D基向量窗（归W11），S81–S83为扩展对照窗（归W14），S80作废并入S87（见能力行作废行），G27导航V2关门不含、V3转正S87，G35文本格式只对齐不新开S，底层渲染仍自家 render/ 不搬 C++。
修订 2026-09-16（2）（评审补齐）：补真2.5D基向量验收S79、Godot默认值对照表D01–D20、门禁数字统一见§5（数不重写）、W11/W14按W块拿活、V2不含清单、真窗对照demo-projects扩展到1＋4（S78＋S79/S81–S83）。

### Godot 默认值对照表（源码实盘，Go 重写逐项对，差多少写多少，2026-09-16 补）

一句话：抄 Godot 不能只抄名字，要把默认值抄对，否则手感全变。下表抄自本地 godot 只读源码头文件与 `_bind_methods` 默认值，只抄做法不搬代码。每个 S 验收加一行和本表差多少，对不上不许绿。

| # | 管谁 | Godot 默认值（抄数用） | 落哪个 S | 怎么验对 |
|---|---|---|---|---|
| D01 | 镜头 Camera2D | zoom=(1,1)、offset=(0,0)、anchor=拖中、ignore_rotation=true、enabled=true、limit=±10000000且开关开、平滑关（开后speed=5.0）、拖拽边距0.2四边且开关关 | S55 | 跟随收敛曲线与 Godot 同参同形，限位钳住值逐位对 |
| D02 | 视差 Parallax | motion_scale=(1,1)、motion_offset=(0,0)、mirroring=(0,0)；新 Parallax2D scroll_scale=(1,1)、repeat_times=1 | S55 | 远慢近快比例逐点对，长路镜像缝 1e-9 内 |
| D03 | 精灵 Sprite2D | centered=true、offset=(0,0)、flip=false、region关、hframes=vframes=1、frame=0 越界钳住 | S31/S36 | 切帧索引语义一致，越界不崩且钳住 |
| D04 | 帧动画 SpriteFrames | speed=5.0、默认动画名 default、单帧时长1.0、LOOP_NONE/LINEAR/PINGPONG 三档 | S74 | dodge 走跑帧按 5fps 播，循环/乒乓行为一致 |
| D05 | 状态机混合 | active=true、默认混合0、BlendSpace2D三角重心权重归一、MAX_BLEND_POINTS=64 | S46/S53 | 切换不跳变，权重和为1 |
| D06 | 缓动 Tween | parallel=false、speed=1、loops=1、TRANS×EASE 查 easing_equations 原样搬 | S04/S78 | 曲线逐点对，镜头推拉手感一致 |
| D07 | CPU 粒子 | emitting=false、lifetime=1.0、one_shot=false、explosiveness=0、randomness=0、local=false、draw_order=INDEX、direction=(1,0)、spread=45、发射形状POINT | S38 | 火锥形状与 Godot 同参同形 |
| D08 | GPU 粒子 | emitting=false、one_shot=false、draw_order=LIFETIME、local=false、interpolate=true、trail关（时长0.3/段8/分4）、超 visibility_rect 被裁 | S43 | 万级量不重启改比例行为一致 |
| D09 | 拖尾 Line2D | width=10、closed=false、joint=SHARP、caps=NONE、color白、sharp_limit=2、round_precision=8、antialiased=false | S43 | 宽窄渐变接头形状一致 |
| D10 | 瓦片 TileSet/Layer | tile_size=(16,16)、INVALID_SOURCE=-1、INVALID_ATLAS=(-1,-1)、rendering_quadrant=16（256块一批）、physics_quadrant=16、y_sort_origin=0 | S57 | 三段寻址错位0容忍，分块批数一致 |
| D11 | 骨骼 Bone2D | length=16、autocalculate=true、angle=0、skeleton_index=-1、rest链式变换 | S42 | 真资源骨头不断，姿态逐点对 |
| D12 | 灯光 Light2D | enabled=true、color=(1,1,1)、energy=1（可超1）、height=0、z∈[-1024,1024]、layer∈[0,0]、mask=1、shadow关、filter=NONE、smooth=0、shadow_color=(0,0,0,0)、blend=ADD | S45/S63 | 手电只照人，超1进光晕不过曝 |
| D13 | 遮挡 Occluder | light_mask=1、无 occluder 就无影、SDF碰撞可选开 | S48 | 影子方向对，无挡不崩 |
| D14 | 全场染色 CanvasModulate | color=(1,1,1,1)、同场唯一激活者（后进/可见者胜） | S45 | 昼夜切换仲裁语义一致 |
| D15 | 排序 Cull | y-sort按全局Y再按子序、z_relative叠加父z、clip_children裁剪栈 | S10/S41 | 盖对关系位级一致 |
| D16 | 材质 key | blend=MIX、light=NORMAL；LIGHT_ONLY/UNSHADED决定吃不吃灯 | S44 | 描边溶解只碰游戏层 |
| D17 | 采样器 | 默认线性过滤、像素风才 Nearest＋整数坐标、斜45度4x各向异性 | S32/S65 | 远树不抖，缝边全透明 |
| D18 | 角色体 CharacterBody2D | motion=GROUNDED、floor_max_angle=45度、snap=0.1、safe_margin=0.08、move_and_slide无参读velocity | S22/S76 | 斜坡站得住，贴地不断 |
| D19 | 碰撞层掩码 | layer=1、mask=1、1–32层位原样对、单向面direction=(0,1)、margin=0 | S06/S21/S61 | 层过滤静默，边触算碰体内0 |
| D20 | 音频输入 | 位置音max_distance=2000、attenuation=1、panning=1；动作死区约0.5、exact_match=false、ALL_DEVICES=-1 | S13/S14/S60/S77 | 中置居中右偏右，改键不改码 |

### 真 2.5D 基向量验收（Godot misc/2.5d 路线，2026-09-16 补，落 S79/W11）

一句话：真2.5D不是把2D堆多点，而是3D的数、2D的画。照抄 `godot-demo-projects/misc/2.5d` 的 Node25D 做法：3D坐标经三组基向量压成2D，6种视角（45度/等距/正视/俯视/两种斜视）只换基向量，排序走 YSort25D，影子走 ShadowMath25D。
- 做什么：新建 `examples/game_25d_basis`，调 game/camera 真包（只读接口不动引擎）；6种基向量矩阵值与 demo 逐位一致，投影公式逐点对。
- 自动看什么：6种矩阵值逐位一致、投影点集逐点对、遮挡序对、影子位置对，JSON 判 PASS/FAIL。
- 人工看什么：一切换视角就像，前后盖对，影子不飘。
- 指标：正式包三遍最差；presents400＋单窗fps57＋，帧门见§5；矩阵逐位0差，投影点误差1e-9内，双金0差。
- 和1.1的关系：1.1上层透视投影只算数不认视角，S79认6视角矩阵，两者数必须同源，矛盾先停下对齐。

### V2 开发计划（W10–W15，S52 起，一次只做一个）

看法：上一波没绿，下一波别开。W 是开工波，S 是单会话号（S52→S83拿号是V2，S84→S90拿号是V3大型化，S91→S97拿号是V4玩法机制，S98→S105拿号是V5基础设施，S106→S112拿号是V6联机多人，新会话不干扰）。同一波标可并行的可一起开，标串行的必须一个完再开下一个。W11/W14 的 S 号不连续，拿活按 W 块拿，别按 S 号顺序拿。

```text
W10 回炉（S52–S54，靠 V1，串行收尾，先把虚绿变实绿）
  S52 追车减分配＋闲时 3 遍取最差（等追车窗现有，帧门见§5才许关 P3）
  S53 W7 三窗闲时单窗重测正式包（等 S47/S48 修后，normal/shadow/fsm 各单窗 60 帧）
  S54 弱证据重测（等 S53，8.3 那 40 帧＋16.2 三档＋S46，正式包＋换机口径补数）

W11 补 12 项（S55–S61七窗＋S74–S77/S79五口径，靠 W10，波内可并行，一个一会话，按W块拿活别按S号顺序）
  S55 game_camera（等 1.3a/1.3b，跟随不飘限位钳住长路无缝；完整卡见V2新人S55卡）
  S56 game_tex 双 case（等 3.2/3.3，远树不抖边走边出）
  S57 game_tilemap 三 case（等 7.1/7.2/7.3，摆怪对边走边装切换不闪）
  S58 game_world（等 10.2，开局装对）
  S59 game_asset（等 12.3，一改即换）
  S60 game_input 双 case（等 13.1/13.2，改键即用连招不丢）
  S61 game_physics 双 case（等 14.1/14.2，撞门准跳得上）
  S74 帧动画窗（等 S11，game_sprite--case=anim）
  S75 时间轴窗（等 S17，game_anim--case=tl）
  S76 角色体对齐（等 S22，snap＋mode口径，窗随S61）
  S77 听者跟相机（等 S13，AudioListener口径，窗随S55）
  S79 真2.5D基向量窗（等 S08/S09/S10，game_25d_basis，6视角矩阵逐位对）

W12 立体细腻 C+（S62–S66，靠 W11，波内可并行，S66 串行等前 4 个）
  S62 材质两张图（法线＋粗糙，木箱人物石头三样板先行，game/light 扩展，render 主路不动）
  S63 灯影柔边＋大灯收费（等 6.1，三档柔边＋覆盖像素记账，真灯不超 10 盏）
  S64 压缩第二种＋转码（等 3.1，头校验＋按卡转码＋粉块兜底）
  S65 过滤写死＋图集留白扩边（等 3.2/12.2，mipmap＋三线性默认＋4x 各向异性＋留白 2–4 扩边 1–2）
  S66 亮暗可选管线（等 S62–S65，9.1 部分解冻：浮点存亮度＋曝光可查＋映射可选＋CPU 降级标记）

W13 合验（S67/S68/S78，靠 W12，串行）
  S67 追车 2 分钟关门（镜头＋排序＋拖尾＋分区＋动态一起上，帧率显存像素三数全过）
  S68 夜战第二关立项（等 S67 绿，灯＋后期，追车不绿不开；只冻接口，不建窗）
  S78 躲避小怪关（等 W11全项绿含S79，照抄dodge_the_creeps；真窗examples/game_dodge自动120秒＋人工2分钟）

W14 养得住＋扩展对照（S69–S72＋S81–S83七项，靠 W13，可并行，按W块拿活别按S号顺序）
  S69 图集工具定死（2048/4096＋旋转反算对＋描边无黑线）
  S70 资产轻量＋断链上报（改一张只重载该集，线上不许边玩边换逻辑）
  S71 预览＋录帧埋点（关卡/粒子所见即所得＋帧分解定位到节点）
  S72 平台矩阵（集显独显各一遍＋X11/Wayland 按真源表一次标齐）
  S81 横版对照窗（等 W11全绿＋S76，game_platformer，照抄platformer手感300/-725/700）
  S82 灯影对照窗（等 S63，gfx_light2d，照抄lights_and_shadows开关＋三档柔边）
  S83 粒子对照窗（等 S43，gfx_particles2d，照抄particles火/烟/爆炸＋子发射）

W15 拍板（S73，靠 W14，串行收尾）
  S73 联机脚本接口先冻（只冻接口写 doc.go，逻辑不动；联机留定点数口，脚本随包走）
  注：S80占位号作废并入S87，W15只剩S73。

注：前置以W块为准（S16等S01、S26/S33等S08视口，能力表漏写处按块补齐）。非Godot单点可无G：S00/S01/S03/S12/S15/S16/S18/S19/S27/S28/S29/S34/S37/S39/S40/S49/S50/S51/S52–S54/S62–S73/S79/S81–S83（核心/梯形/步长/复用/资产/存档/动态/扭曲/后期/工具/回炉/合验/养活/对照窗；S80已作废见能力行），其余G/S已对齐。S78前置写死等W11关门（含S79）。
```

分期关门（每波全绿才进下波）：

| 波 | 装啥 | 并行标记 | 关门线 | 关门证据 |
|---|---|---|---|---|
| W10 | 回炉三项 | 串行收尾 | 最差那遍也过（帧门见§5） | 正式包 JSON＋三遍全贴＋换机一遍（换机=集显独显各一遍，见S72） |
| W11 | 12项（S55–S61＋S74–S77/S79） | 波内可并行，按W块拿活 | 每个窗双 JSON | 自动 JSON＋人工 JSON＋金图进 testdata（金图按窗大小，1920x1080窗不拿小金顶） |
| W12 | 立体细腻五项 | 波内可并行；S66 串行等前 4 | 样板一眼像，数全过 | 四样板并排图＋三方对比＋降级标记 |
| W13 | 追车＋夜战立项＋躲避关 | 串行（S67→S68→S78） | 2 分钟三数全过；S68只冻接口；S78和原版手感一致 | 120 秒 JSON＋2 小时报表另起＋dodge双JSON |
| W14 | 养活＋对照七项（S69–S72＋S81–S83） | 波内可并行，按W块拿活 | 美术能干活，卡能定位，对照窗手感一致 | 预览人工看＋报表＋断链日志＋对照双JSON |
| W15 | 拍板（S73） | 串行 | 文档不再留糊涂账 | 接口 doc＋版本号＋老调用能编过 |

### V2 真窗怎么做（每个窗统一套路，不重造架子）

一套：wrkit 搭壳（顶栏加图例加身体加 HUD），wrgate 判门（present 加 fps 加 p95），wrsoak 管探针加对比图。游戏窗照抄 ui_wr_r15_soak，不另起。
两步：先自动判（RUN_SECONDS 跑完收 JSON 判 PASS/FAIL），再留窗给人看（常驻收事件改标题，关窗出汇总 JSON）。跑完即关的一次性不能顶人工窗。
对比图全进各窗 testdata，容差写死，改基准记谁为啥哪天。先查代码，不许直接更新图装绿。下表“同上帧门”=“帧门见§5”，V3/V4/V5各节同此。

| 窗 | 验谁 | 自动判啥 | 人工看啥 | 指标 |
|---|---|---|---|---|
| game_camera（新建） | S55 | 跟随不飘、限位钳住、长路无缝 JSON | 推镜头跟，抖不晕 | 1080p正式包60帧（57＋容抖），presents400＋，帧门见§5，贴住不超0.5秒，10公里缝1e-9内 |
| game_tex（新建双 case） | S56 | 远树不抖、边走边出 JSON | 远近都清 | 同上帧门；缩到0.3倍无闪，接缝不断，缺图品红占位，parity0%双金0差 |
| game_tilemap（新建三 case） | S57 | 摆怪对、边走边装、切换不闪 JSON | 地图大走不丢 | 同上帧门；万级视口外裁，256块一批60帧，临界回滞不闪 |
| game_world（新建） | S58 | 开局装对 JSON | 开局齐 | 同上帧门；2000实体解析10ms量级，大关卡连开5次零漂移 |
| game_asset（新建） | S59 | 一改即换 JSON | 改完即看 | 同上帧门；只重载该集，断链报哪条链，坏文件保旧 |
| game_input（新建双 case） | S60 | 改键即用、连招不丢 JSON | 按着顺手 | 同上帧门；按键到逻辑不到1帧，满64不涨 |
| game_physics（新建双 case） | S61 | 撞门准、跳得上 JSON | 跳不穿落得住 | 同上帧门；同位不崩，长跑不穿 |
| game_sprite--case=anim（新建） | S74 | 帧号时长循环回调 JSON | 待机跑跳切换顺 | 同上帧门；六序列全对，双金0差 |
| game_anim--case=tl（新建） | S75 | 插值循环事件准时 JSON | 动作事件准时 | 同上帧门；三序列冻结对 |
| 角色体snap/mode（随S61） | S76 | 贴地吸附＋两档motion JSON | 斜坡站得住 | 同上帧门；单向穿过再落住 |
| 听者跟相机（随S55） | S77 | 距离衰减＋听者跟随 JSON | 左右远近对 | 同上帧门；中置居中右偏右 |
| game_25d_basis（新建真2.5D窗） | S79 | 6视角矩阵逐位对＋投影逐点对 JSON | 一切换视角就像，盖对影子不飘 | 同上帧门；矩阵0差，投影误差1e-9内 |
| game_dodge（新建P1关） | S78 | 120秒自动＋人工2分钟 JSON | 和原版手感一致 | presents6000＋（帧门见§5），速度400分毫不差 |
| game_light 三 case 重验 | S62/S63/S66 | 只照人、鼓包行、影子对 JSON | 白天鼓包见，夜里只照人，暗部和背景分开 | 白灯 2 倍出晕不过曝，暗场有层次 |
| game_tex 远近重验 | S65 | 远近双 0 差 JSON | 斜地面无糊无纹 | 斜 45 度无闪，边无黑线 |
| game_fx 全屏重验 | S62/S66 | 全屏帧率＋气氛 JSON | 水晃光晕对味，UI 一点不动 | 坏特效只黑游戏层，不崩主路 |
| game_stage_chase | S52/S67 | 2 分钟三数 JSON | 追车顺，车不穿 | 帧门见§5 |
| tools 双预览 | S71 | 所见即所得 JSON | 摆完和运行一致 | 卡能定到节点级 |
| game_platformer（新建扩展对照） | S81 | 步速跳速坠落上限＋二段＋松键减速 JSON | 跳得顺落得住 | 同上帧门；300/-725/700对，二段只一次 |
| gfx_light2d（新建扩展对照） | S82 | 开关＋三档柔边 JSON | 光斑影子形状对 | 同上帧门；开关前后金图对比对 |
| gfx_particles2d（新建扩展对照） | S83 | 火/烟/爆炸＋子发射＋碰撞 JSON | 火像火烟像烟 | 同上帧门；发射器全创建帧率不崩 |

### V2 真窗对照 demo-projects（1必做＋4扩展，2026-09-16 补）

| 窗 | 照抄哪个 demo | 验引擎哪块 | 自动看什么＋人工看什么 |
|---|---|---|---|
| game_dodge（S78/W13必做） | 2d/dodge_the_creeps | 输入/移动/刷怪/碰撞/加分/HUD/音频/存盘整环 | 人速400±5%、怪速150–250、0.5秒刷一只、1秒加1分、撞必死；人工和原版并排手感一致 |
| game_25d_basis（S79/W11必做） | misc/2.5d | 6视角基向量＋YSort＋影子 | 矩阵逐位对＋投影逐点对；人工一切换就像 |
| game_platformer（S81/W14扩展） | 2d/platformer | 加速度/跳跃/二段/急坠/镜头/分屏 | 步速300/跳-725/坠落≤700、松键减速、二段只一次；人工跳跃高度顺落地不硬 |
| gfx_light2d（S82/W14扩展） | 2d/lights_and_shadows | 点光＋方向光＋阴影三档 | 开关状态位翻转、shadow_filter 0–2轮转；人工光斑影子形状对 |
| gfx_particles2d（S83/W14扩展） | 2d/particles | GPU粒子全家桶＋子发射＋碰撞 | 发射器全创建帧率不崩；人工火烟爆炸像不像 |

### V2 指标总表（已并入§5；V2 量仍是小规模档，大型档看 V3 规模对照表）

### V2 做成什么样（过线样子：白天鼓包见、晚上只照人、镜头跟得住、追车顺、策划摆完和运行一致；细数看各波卡，不另列）

### V2 全不全（已并入单能力五步＋§5；6 样就是五步加文档回写，少一样不算完）

修订 2026-09-16（V2 立项）：W0–W9 关门不动，9.1仍记冻结、V2的S66部分解冻做可选管线（主路8位不动），联机脚本由看情况再加改先冻接口，追车hitch25（旧阈值33.4ms，统一按超20ms重算）转W10 S52，11项转W11（评审补齐后现12项含S79），立体细腻转W12。

### V2 天花板拆分（阶段按依赖排，波内可并行，每点有数，每波有总数，不许假绿）

一句话：平均帧绿不算绿，小图绿不算绿，单机单遍绿不算绿，最差那遍绿才算绿。

天花板通用门禁（W10–W15 全体适用，任一条红就整波红；内容已并入§5，本节只留一句话）：平均帧绿不算绿，小图绿不算绿，单机单遍绿不算绿，最差那遍绿才算绿。

依赖总览（谁卡谁）：
```text
W10 回炉（串行）→ W11 补12项（可并行）→ W12 立体细腻（S62–S65 可并行，S66 等前 4）→ W13 合验（串行S67→S68→S78）→ W14 养活＋对照七项（可并行）→ W15 拍板一项S73（串行；S80已作废）
```

#### W10 回炉（S52–S54，串行，先把虚绿变实绿）

S52 追车减分配＋帧门打进线：
详见 `docs/2.5D/G12.md`（S52原话已搬家，详情以组文件为准；状态以本文件能力行为准，门槛只认§5）。

S53 W7 三窗单窗重测正式包：
详见 `docs/2.5D/G13.md`（S53原话已搬家，详情以组文件为准；状态以本文件能力行为准，门槛只认§5）。

S54 弱证据重测（8.3＋16.2＋S46）：
详见 `docs/2.5D/G14.md`（S54原话已搬家，详情以组文件为准；状态以本文件能力行为准，门槛只认§5）。

W10 总指标：三项全正式包，三遍全贴取最差判（帧门见§5，双金全 0 差），换机一遍照样过，每项加一行与 D01–D20 差多少（对不上不许绿），文档回写三处齐。少一项 W11 不开。

#### W11 补 12 项（S55–S61七窗＋S74–S77/S79五口径，靠 W10，波内可并行，一窗一会话，按W块拿活）

S55 game_camera（1.3a/1.3b）：
详见 `docs/2.5D/G15.md`（S55原话已搬家，详情以组文件为准；状态以本文件能力行为准，门槛只认§5）。

S56 game_tex 双 case（3.2/3.3）：
详见 `docs/2.5D/G15.md`（S56原话已搬家，详情以组文件为准；状态以本文件能力行为准，门槛只认§5）。

S57 game_tilemap 三 case（7.1/7.2/7.3）：
详见 `docs/2.5D/G15.md`（S57原话已搬家，详情以组文件为准；状态以本文件能力行为准，门槛只认§5）。

S58 game_world（10.2）：
详见 `docs/2.5D/G15.md`（S58原话已搬家，详情以组文件为准；状态以本文件能力行为准，门槛只认§5）。

S59 game_asset（12.3）：
详见 `docs/2.5D/G15.md`（S59原话已搬家，详情以组文件为准；状态以本文件能力行为准，门槛只认§5）。

S60 game_input 双 case（13.1/13.2）：
详见 `docs/2.5D/G15.md`（S60原话已搬家，详情以组文件为准；状态以本文件能力行为准，门槛只认§5）。

S61 game_physics 双 case（14.1/14.2）：
详见 `docs/2.5D/G15.md`（S61原话已搬家，详情以组文件为准；状态以本文件能力行为准，门槛只认§5）。

S74 帧动画窗（等 S11，G05/G33）：
详见 `docs/2.5D/G15.md`（S74原话已搬家，详情以组文件为准；状态以本文件能力行为准，门槛只认§5）。
S75 时间轴窗（等 S17，G06）：
详见 `docs/2.5D/G15.md`（S75原话已搬家，详情以组文件为准；状态以本文件能力行为准，门槛只认§5）。
S76 角色体对齐（等 S22，G24，窗随S61）：
详见 `docs/2.5D/G15.md`（S76原话已搬家，详情以组文件为准；状态以本文件能力行为准，门槛只认§5）。
S77 听者跟相机（等 S13，G30，窗随S55）：
详见 `docs/2.5D/G15.md`（S77原话已搬家，详情以组文件为准；状态以本文件能力行为准，门槛只认§5）。
S79 真2.5D基向量窗（等 S08/S09/S10，G01＋misc/2.5d，窗新建game_25d_basis）：
详见 `docs/2.5D/G15.md`（S79原话已搬家，详情以组文件为准；状态以本文件能力行为准，门槛只认§5）。

W11 总指标：12项全建（有目录有main.go才算建，无目录算未建），每窗自动＋人工双 JSON（人工事件0算空转不算过），金图全进 testdata容差写死（静图0容差，金图按窗大小），正式包＋三遍最差＋换机口径，（帧门见§5，双金0差）全过，每项加一行与 D01–D20 差多少（对不上不许绿）。少一项 W12 不开。

新人S55完整卡示例（照此抄，其余S同模子）：
- 做什么：新建 examples/game_camera，一个窗验跟随＋限位＋平滑＋视差长路，调 game/camera 真包（camera.go/parallax.go/project.go只读，窗里只调接口不动引擎）。
- 改哪几个文件：新建 examples/game_camera/main.go＋README.md＋testdata/（camera_follow_golden.png＋camera_cases.json）；不动 game/camera/*.go；回归跑 go test ./game/camera。
- 跑哪几个命令：go test ./game/camera -count=1；go vet ./game/camera ./...；RUN_SECONDS=10 -auto-only 收自动JSON；-manual-seconds 60 常驻收人工JSON（必须有点按/按键事件）。
- 数到多少：正式包，预热5秒跑3遍取最差；presents400＋，单窗fps57＋，帧门见§5，parity0%，离屏金0差窗金0差，10公里缝1e-9内，贴住不超0.5秒，坏限位钳住不崩。
- 参考谁：Godot scene/2d/camera_2d.h（get_camera_transform/zoom/limit/drag/position_smoothing/reset_smoothing/AnchorMode）＋parallax_background.h/parallax_layer.h/parallax_2d.h（scroll_scale/repeat/autoscroll）；分层看G01 canvas_item.h的z_index/y_sort。
- 五步：接口已冻免评审→单测→接线（两边齐）→真窗双JSON→回归（camera整包＋game_sprite旧窗重跑）；文档回写能力行＋W11关门行＋修订。

#### W12 立体细腻 C+（S62–S66，靠 W11，S62–S65 可并行，S66 等前 4）

S62 材质两张图（法线＋粗糙）：
详见 `docs/2.5D/G16.md`（S62原话已搬家，详情以组文件为准；状态以本文件能力行为准，门槛只认§5）。

S63 灯影柔边＋大灯收费：
详见 `docs/2.5D/G16.md`（S63原话已搬家，详情以组文件为准；状态以本文件能力行为准，门槛只认§5）。

S64 压缩第二种＋转码：
详见 `docs/2.5D/G16.md`（S64原话已搬家，详情以组文件为准；状态以本文件能力行为准，门槛只认§5）。

S65 过滤写死＋图集留白扩边：
详见 `docs/2.5D/G16.md`（S65原话已搬家，详情以组文件为准；状态以本文件能力行为准，门槛只认§5）。

S66 亮暗可选管线（9.1 部分解冻）：
详见 `docs/2.5D/G17.md`（S66原话已搬家，详情以组文件为准；状态以本文件能力行为准，门槛只认§5）。

W12 总指标：四样板（木箱/人物/石头/小场景）并排图亲眼对，白天鼓包见、晚上只照人、暗部和背景分开、不再是一团亮晕；远树拉远不闪，描边无黑线；显存压到 1/4–1/8；正式包＋三遍最差＋换机；双金 0 差；降级有标记。少一样 W13 不开。

#### W13 合验（S67–S68，靠 W12，串行，单项绿合起来也绿才算绿）

S67 追车 2 分钟关门：
详见 `docs/2.5D/G18.md`（S67原话已搬家，详情以组文件为准；状态以本文件能力行为准，门槛只认§5）。

S68 夜战第二关立项（等 S67 绿才开）：
详见 `docs/2.5D/G19.md`（S68原话已搬家，详情以组文件为准；状态以本文件能力行为准，门槛只认§5）。

S78 躲避小怪关（等 W11全项绿含S79，归W13）：
详见 `docs/2.5D/G20.md`（S78原话已搬家，详情以组文件为准；状态以本文件能力行为准，门槛只认§5）。

W13 总指标：120 秒 JSON＋2 小时报表＋三遍最差全贴，不许只贴最好；帧率显存像素三数全过，长跑在线，崩留日志；S78双JSON齐。P3组合关门写已绿才许进 W14。

### V2 不含清单（本版明确不做，V3另排，不许顺手在当前线绕过去）

| # | 不含谁 | Godot 对应 | 为什么本版不动 | 去哪 |
|---|---|---|---|---|
| N1 | 寻路导航全套 | G27 NavigationAgent/Region/Link/Obstacle＋NavigationPolygon | 大地图怪多了会穿墙卡死，小关可躲，大包必须有 | S87转正（S80占位号作废并入S87，W17开工） |
| N2 | 关节链条弹簧滑槽 | scene/2d/physics/joints（Pin/DampedSpring/Groove） | 绳子链条弹簧门钩索枪，没有就做不了这类玩法 | S100转正（W21开工，body.go加关节口） |
| N3 | 触屏虚拟键（手机端，不在本线） | TouchScreenButton | 框架只支持桌面三端，手机端不支持，不做 | S101作废，不开工 |
| N4 | 跨节点同步变换/空标记/进屏启停 | RemoteTransform2D＋Marker2D＋VisibleOnScreenNotifier/Enabler2D | 大世界里武器挂点跟人、出生点读档、屏外休眠的口子必须有语义，否则各包各写一套 | 并入S84/S85，不另开S（挂点随实体、出生点随场景、进屏启停随裁剪验） |
| N5 | 水折射拷贝缓冲 | BackBufferCopy（rect/copy_mode） | 水面热浪折射，没有就只有假扭曲 | S98转正（W21开工，S66管线稳了再接） |
| N6 | 音频效果器链 | servers/audio/effects十几种＋AudioStream三次插值重采样＋area_mask混响区 | 大包几十路同响时必须有复用和闪避，全套效果器等有调音台再说 | 子集已并入S89（64路复用＋混响区＋闪避），全链转正S103（W22开工），验收只认人工听＋波形 |
| N7 | 真编辑器（摆关卡所见即所得全功能） | editor/scene 210文件那套 | 策划摆怪调技能看遮挡，没有就只能半自动 | S104转正（W22开工，S50/S71预览是前置） |
| N8 | 联机逻辑/高光金属真材质 | 定点数回滚＋脚本下发＋specular/metallic全管线 | 首版只留口（S73冻接口），逻辑不动；粗糙先行（S62），金属等S66稳了再说 | 联机转正 V6（S109/S110/S112，W23–W24）；金属高光已转正S99（W21开工） |

#### W14 养得住＋扩展对照（S69–S72＋S81–S83七项，靠 W13，波内可并行，按W块拿活）

S69 图集工具定死：max2048/4096、padding2–4、extrude1–2、旋转反算对、DrawCall 随集数线性降，大关卡打开时长有数，连开 5 次零漂移。
详见 `docs/2.5D/G21.md`（S69原话已搬家，详情以组文件为准）。
S70 资产轻量＋断链上报：改一张只重载该集，依赖错报哪条链断，线上只整包热更不许边玩边换逻辑，坏档回中档不崩。
详见 `docs/2.5D/G21.md`（S70原话已搬家，详情以组文件为准）。
S71 预览＋录帧埋点：tools 关卡/粒子所见即所得，摆完和运行一致；Profiler 录帧定到节点级（采样开销不超5%，时间精度1ms，埋点字段frame/draws/batch/mem必填），关键链路有自定义帧率数量埋点；美术看到的遮挡混合和运行一致。
详见 `docs/2.5D/G21.md`（S71原话已搬家，详情以组文件为准）。
S72 平台矩阵：集显（Intel HD520）独显（940MX/580.178.04）各一遍，1920x1080＋X11必测、Wayland按真源表标，符号只用所属文档图例，没做写原因不许空；三档机（高/中/低）各测一遍，低档内存比高档省 40% 以上，切档即生效不重启。
详见 `docs/2.5D/G21.md`（S72原话已搬家，详情以组文件为准）。
S81 横版对照窗（等 W11全绿＋S76，照抄platformer，窗新建examples/game_platformer）：
详见 `docs/2.5D/G21.md`（S81原话已搬家，详情以组文件为准；状态以本文件能力行为准，门槛只认§5）。
S82 灯影对照窗（等 S63，照抄lights_and_shadows，窗新建examples/gfx_light2d）：
详见 `docs/2.5D/G21.md`（S82原话已搬家，详情以组文件为准；状态以本文件能力行为准，门槛只认§5）。
S83 粒子对照窗（等 S43，照抄particles，窗新建examples/gfx_particles2d）：
详见 `docs/2.5D/G21.md`（S83原话已搬家，详情以组文件为准；状态以本文件能力行为准，门槛只认§5）。

W14 总指标：美术能干活（摆完即看），卡能定位（录帧到节点），换端不翻车（三档机全过），对照窗手感一致（S81–S83双JSON齐），文档平台格无空白。少一项 W15 不开。

#### W15 拍板（S73，靠 W14，串行收尾）

S73 联机脚本接口先冻：
详见 `docs/2.5D/G22.md`（S73原话已搬家，详情以组文件为准；状态以本文件能力行为准，门槛只认§5）。

W15 总指标：接口 doc＋版本号进冻结表＋老调用能编过，三者齐才算 V2 关门。（S80已作废并入S87，不在这里验收。）

### V2 能力行（S52–S83，未开工，新人按此关门；V1表不动）

| 编号 | 能力 | 前置 | 窗 | 状态 |
|---|---|---|---|---|
| S52 | 追车减分配回炉 | 追车窗现有 | game_stage_chase | 进行中（2026-09-17；camera/sprite/particle/tilemap/step共17个_test.go逐个复跑全PASS＋vet净＋CGO_ENABLED=0四包可构建；减分配只动窗＋四包热路径、逻辑数画面不变：batch单图快路＋Clear复用底子、chunk范围缓存、排序计数＋300tick全量校验零失配、标签4Hz、dirty复用、emitter无子发射免deaths片、GPUPool原地sync；正式包go build -trimpath -ldflags="-s -w" 120秒×3取最差：presents7137、fps59.47、p50 16.87、p95 17.53、p99 18.06、h33 0/分、h20重算2.00/分、moved24624、vis/load12、sorted131、trail32、pool48、batch1、dirty1、full0、fb0、RSS止≈峰、离屏金0、窗金静态条33400像素0差、parity图集路0差；人工30秒key4＋ptr12（XTEST合成，同一协议路径）、presents1793、backend=x11；换机独显建窗失败（降级链high→low→software）：首遍940MX 1GB已用673MB，后松到345MB、可用623MB仍卡在session_depth_stencil、连1x1兜底都建不出来；DBG确认选卡对（NVIDIA DiscreteGPU Vulkan，能跑3帧才死）＋最小raw独显测试（自建device＋320窗＋depth＋强制恢复）0.53秒PASS＋最小完整窗game_quad独显8秒同签名失败＋原生判决测试（绕wgpu直调vkAllocateMemory 3.66MB device-local成功0.15秒、D24S8 1200x800建镜像＋绑内存成功0.13秒）——驱动堆本身可用，悬崖在wgpu-native块分配器；1x默认＋1x1闩住＋进程台账＋CreateBuffer回调补口＋探针独立设备释放＋设备就绪门（8秒）＋小规格描述符（GPUI_LOW_VRAM）七件修后，独显仍死在开窗首个3.66MB depth（low/software两级也一样），属wgpu-native侧问题、本线解不了，未过；p95红收敛到屏物理：59.93Hz同步底16.69ms、p50 16.78已贴底、p95三遍17.53/17.17/17.17，≤16在该屏不可达、非代码可解；D表差异：本项只减分配不改数值、D01–D20沿用V1实现差异0，无G号原因见总账非Godot单点注；改文件：examples/game_stage_chase/main.go＋game/sprite/batch.go＋game/tilemap/chunk.go＋game/step/dirty.go＋game/particle/emitter.go＋game/particle/gpu.go＋testdata/chase_final_base.png；G13不开） |
| S53 | W7三窗单窗正式重测 | S46–S48 | fsm/normal/shadow | 未开工 |
| S54 | 弱证据重测 | S53 | dirty/q123 | 未开工 |
| S55 | 相机窗 | S08/S09 | game_camera | 未开工 |
| S56 | 贴图窗 | S32/S29 | game_tex双case | 未开工 |
| S57 | 地图窗 | S05/S20/S26 | game_tilemap三case | 未开工 |
| S58 | 世界窗 | S33 | game_world | 未开工 |
| S59 | 资源窗 | S28 | game_asset | 未开工 |
| S60 | 输入窗 | S07/S23 | game_input双case | 未开工 |
| S61 | 物理窗 | S06/S21/S22 | game_physics双case | 未开工 |
| S62 | 材质粗糙 | S42 | game_light重验 | 未开工 |
| S63 | 灯影柔边收费 | S45 | torch/shadow重验 | 未开工 |
| S64 | 第二压缩转码 | S02 | game_tex重验 | 未开工 |
| S65 | 过滤图集写死 | S32/S27 | tex/sprite重验 | 未开工 |
| S66 | 可选亮暗管线 | S62–S65 | light/fx重验 | 未开工 |
| S67 | 追车关门 | W12 | game_stage_chase | 未开工 |
| S68 | 夜战立项 | S67 | 只接口无窗 | 未开工 |
| S69 | 图集工具 | W13 | tools | 未开工 |
| S70 | 资产断链 | W13 | game_asset | 未开工 |
| S71 | 预览录帧 | W13 | tools双预览 | 未开工 |
| S72 | 平台矩阵 | W13 | 三档机集显独显各一遍 | 未开工 |
| S73 | 联机脚本冻接口 | W14 | 只接口无窗 | 未开工 |
| S74 | 帧动画窗 | S11 | sprite--case=anim | 未开工 |
| S75 | 时间轴窗 | S17 | anim--case=tl | 未开工 |
| S76 | 角色体对齐 | S22 | 随S61 | 未开工 |
| S77 | 听者跟相机 | S13 | 随S55 | 未开工 |
| S78 | 躲避小怪关 | W11全项含S79 | game_dodge | 未开工 |
| S79 | 真2.5D基向量窗 | S08/S09/S10 | game_25d_basis | 未开工 |
| S80 | 导航占位号（作废） | — | 并入S87 | 作废（见S87） |
| S81 | 横版对照窗 | W11全绿＋S76 | game_platformer | 未开工 |
| S82 | 灯影对照窗 | S63 | gfx_light2d | 未开工 |
| S83 | 粒子对照窗 | S43 | gfx_particles2d | 未开工 |

### V2 门禁挂波（已并入前面分期关门表；每波跑啥、判什么，看那张表＋各波总指标）

口径说明（V1历史表以本节为准）：V1分期关门表为历史记录，阈值与换机口径统一见§5（数不重写）。V1历史行里写的旧数一律按§5重算（旧数作废，此处不复述）。

红了谁负责：game/tools 红改 game/tools，render 主路红先回滚游戏分支，不动主路。

### V3 大型化（S84–S90，W16–W18，大游戏档，2026-09-16 补）

修订2026-09-16（3）（大型化）：平台层归别线，本线只做引擎。S80占位号作废并入S87。N1寻路转正S87，N4挂点/出生点/进屏启停并入S84/S85不另开S，N6声音子集并入S89。W15只剩S73。（N2/N3/N5/N6全链/N7/N8金属去向已更新，见§0.1与V2不含清单。）V3共7项，上一波没绿下一波别开，拿活按W块拿。

一句话：大游戏考的不是单项有没有，是量大了还稳不稳。下面每项都按新人S55卡写满做法，不写空话。

#### 平台口子（本线只定要的数，怎么做是别线的事）

- 本线要的：帧率门见§5帧门（数不重写）；显存内存预算表（高/中/低各档上限MB）；纹理头只认冻过的两种，头不对拒收给原因；声音64路复用留最响32；输入事件到逻辑不到1帧；切后台缩窗口回来不黑屏，回来能画。
- 别线给的：窗口、显卡后端、声音后端、文件读写、打包签名、跨端重测。S72平台矩阵的跨端格由别线标，本线只认数。
- 碰线规矩：引擎只调render现有公开接口和平台事件接口，不直碰驱动；坏路（缺图坏档显存不够驱动掉线）占位加报错加日志，不崩不黑屏。

#### 规模对照（V2小规模 vs V3大型，开工前先看这张）

| 项 | V2小规模（已定） | V3大型（本节） | 在哪验 |
|---|---|---|---|
| 瓦片总量 | 万级视口外裁，256块一批60帧 | 100万瓦片（1024x1024），32x32分区约977区，视口外全裁，每帧装卸≤2ms | game_tilemap--case=large |
| 实体 | 全场千级，同屏百级 | 全场2万，同屏1500，屏外睡眠不更新，排序只排active集 | game_world--case=large |
| 碰撞 | 百盒查询350us量级 | 千盒静态＋200动态，均匀网格（格128像素），单查询≤50us、射线≤30us，每帧候选≤40 | game_physics--case=large |
| 寻路 | 无（S80已作废，见S87） | 256x256网格烘焙≤200ms，同屏100怪分摊（每帧最多算N条排队），到点容差2px，速度200 | game_nav新建 |
| 粒子 | CPU千内/GPU万级 | GPU十万级＋visibility裁剪＋改比例不重启，CPU千内管逻辑耦合 | gfx_particles2d复用＋S83 |
| 骨骼动画 | 单角色，百级同屏 | 全场千骨骼只更新视口内，同屏百骨骼全更新≤1ms，屏外停更、远半速 | game_anim复用 |
| 资产 | 百资产，2000实体解析10ms量级 | 3000张小图/50个图集2048或4096，改一张只重载该集≤50ms，大关卡打开≤3s，连开5次零漂移 | game_asset--case=large |
| 声音 | 256声压测，位置音＋音乐闪避 | 64路复用留最响32（128请求），位置音＋听者跟相机＋爆炸压音乐，满刻度不削波 | game_anim/S55窗复用＋人工听 |
| 长跑 | 2小时涨≤5% | 8小时涨≤5%，切后台缩窗口各2次回来不黑屏，录帧到节点 | game_stage_chase--case=large |

#### W16 世界做大（S84–S86，靠W15，波内可并行，一窗一会话）

S84 大地图流式与预算（等 S05/S20/S26＋S33，G12/G34，窗game_tilemap--case=large）：
详见 `docs/2.5D/G23.md`（S84原话已搬家，详情以组文件为准；状态以本文件能力行为准，门槛只认§5）。

S85 大实体裁剪与排序（等 S25/S33＋S10/S41，G01/G17，窗game_world--case=large）：
详见 `docs/2.5D/G23.md`（S85原话已搬家，详情以组文件为准；状态以本文件能力行为准，门槛只认§5）。

S86 大物理宽阶段与休眠（等 S06/S21/S22，G23/G24/G26，窗game_physics--case=large）：
详见 `docs/2.5D/G23.md`（S86原话已搬家，详情以组文件为准；状态以本文件能力行为准，门槛只认§5）。

W16总指标：三项全正式包三遍最差（帧门见§5，双金0差），每窗双JSON，金图按窗大小，少一项W17不开。

#### W17 系统做密（S87–S89，靠W16，波内可并行，一窗一会话）

S87 寻路转正（等 S05/S22，G27，窗examples/game_nav新建，S80作废并入本项）：
详见 `docs/2.5D/G24.md`（S87原话已搬家，详情以组文件为准；状态以本文件能力行为准，门槛只认§5）。

S88 大资源包与显存预算（等 S19/S27/S33，G22/G35，窗game_asset--case=large＋tools预览复用）：
详见 `docs/2.5D/G24.md`（S88原话已搬家，详情以组文件为准；状态以本文件能力行为准，门槛只认§5）。

S89 大声音复用与动画LOD（等 S13/S14/S42/S77，G28–G31/G13，窗复用不新建）：
详见 `docs/2.5D/G24.md`（S89原话已搬家，详情以组文件为准；状态以本文件能力行为准，门槛只认§5）。

W17总指标：三项全正式包三遍最差，声音加人工听＋波形，导航人工点选事件非0，少一项W18不开。

#### W18 大包关门（S90，靠W17，串行）

S90 大包长跑与录帧关门（等W16＋W17全绿，窗game_stage_chase--case=large）：
详见 `docs/2.5D/G25.md`（S90原话已搬家，详情以组文件为准；状态以本文件能力行为准，门槛只认§5）。

W18总指标：10分钟JSON＋8小时报表＋三遍最差全贴，不许只贴最好；帧率显存像素三数全过，长跑在线，崩留日志；S90双JSON齐才算V3关门。

#### V3 真窗（统一套路，不重造架子；新窗只有game_nav一个）

| 窗 | 验谁 | 自动判啥 | 人工看啥 | 指标 |
|---|---|---|---|---|
| game_tilemap--case=large | S84 | 百万索引＋每帧装卸JSON | 大地图走不丢 | 索引≤2s，每帧≤2ms，同上帧门 |
| game_world--case=large | S85 | 2万实体＋active集JSON | 开局齐，走不丢 | active1500更新≤3ms，同上帧门 |
| game_physics--case=large | S86 | 千盒＋候选上限JSON | 跳不穿落得住 | 查询≤50us射线≤30us，同上帧门 |
| game_nav（新建） | S87 | 烘焙＋分摊＋终态JSON | 点选有路无路都有终态 | 烘焙≤200ms，同上帧门 |
| game_asset--case=large | S88 | 预算＋单集重载＋断链JSON | 改完即看，断链报得出 | 单集≤50ms，打开≤3s，同上帧门 |
| game_anim复用＋听者窗复用 | S89 | 复用＋LOD JSON＋人工听 | 远近左右对，多声不爆 | 百骨骼≤1ms，满刻度不削波 |
| game_stage_chase--case=large | S90 | 10分钟＋8小时JSON | 追车顺车不穿 | 10分钟34000＋，8小时涨≤5% |

#### V3 能力行（S84–S90，未开工，新人按此关门；V1/V2表不动）

| 编号 | 能力 | 前置 | 窗 | 状态 |
|---|---|---|---|---|
| S84 | 大地图流式与预算 | S05/S20/S26＋S33 | game_tilemap--case=large | 未开工 |
| S85 | 大实体裁剪与排序 | S25/S33＋S10/S41 | game_world--case=large | 未开工 |
| S86 | 大物理宽阶段与休眠 | S06/S21/S22 | game_physics--case=large | 未开工 |
| S87 | 寻路转正 | S05/S22 | game_nav | 未开工 |
| S88 | 大资源包与显存预算 | S19/S27/S33 | game_asset--case=large | 未开工 |
| S89 | 大声音复用与动画LOD | S13/S14/S42/S77 | 复用不新建 | 未开工 |
| S90 | 大包长跑与录帧关门 | W16＋W17全绿 | game_stage_chase--case=large | 未开工 |

#### V3 门禁挂波（每波跑啥谁判）

| 波 | 跑啥 | 判 |
|---|---|---|
| W16 | S84–S86单测＋自动＋人工双JSON＋三遍最差 | 一项红整波红 |
| W17 | S87–S89单测＋自动＋人工（导航点选/声音人工听）＋三遍最差 | 一项红整波红 |
| W18 | S90的10分钟＋8小时＋录帧＋三遍最差全贴 | 三数全过才关门 |

注：非Godot单点可无G：S84（大地图工程预算）/S85（裁剪工程）/S88部分（预算工程）/S90（长跑工程）可无G，其余G已对齐（S84=G12/G34，S85=G01/G17，S86=G23/G24/G26，S87=G27，S88=G22/G34/G35，S89=G28–G31/G13/G30）。

### V4 玩法机制（S91–S97，W19–W20，任意2.5D档，2026-09-16 补）

修订2026-09-16（4）（任意游戏）：重审结论——V1管算对、V2管做像、V3管做大，但离“任意2.5D游戏都能做”还差7个玩法机制：战斗、AI、对话任务、背包、随机地牢、本地化、回放。平台跳跃/躲避/RPG/roguelike/塔防/潜行/竞速/格斗单机/音游演出，少一样就有类型做不出来。本节一次补齐，每项照S55卡写满做法。

#### 任意2.5D支撑矩阵（先看这张，缺口全落S91–S97）

| 类型 | 要的机制 | 落点 | 缺口 |
|---|---|---|---|
| 平台跳跃 | 角色体/斜坡/单向/跳板弹簧/移动平台/镜头 | S22/S76/S86/S55 | 无（跳板弹簧已进S86） |
| 躲避弹幕 | 刷怪/碰撞/计时加分/存盘 | S78 | 无 |
| 横版射击 | 子弹池/射线/伤害/火花 | entity/ray/pool＋S91 | S91伤害 |
| RPG | 走格/战斗/对话任务/商店背包/存盘 | S93/S94/S91/S15 | S91/S93/S94 |
| Roguelike | 随机地牢/种子重放/永久死亡/掉落表 | S95/S97/S94 | S95/S97 |
| 塔防 | 路径/波次/建造/伤害 | S87/S91 | S91 |
| 潜行 | 灯影/视野锥/听觉/AI | S63/S92 | S92 |
| 竞速追车 | 步长/录像回放 | S03/S97/S67 | S97 |
| 格斗单机 | 确定帧/快照/AI | S97/S92 | S92/S97 |
| 音游演出 | 节拍时钟/timeline事件/输入缓冲 | S89/S17/S23 | 无（节拍时钟已进S89） |
| 解谜经营 | 实体/场景/物品数值 | 通用数据冻结 | 无，不新开S |

#### W19 机制打底（S91–S94，靠W18，波内可并行，一窗一会话，按W块拿活）

S91 战斗伤害（等 S06/S21/S22，G23/G25，窗examples/game_combat新建）：
详见 `docs/2.5D/G26.md`（S91原话已搬家，详情以组文件为准；状态以本文件能力行为准，门槛只认§5）。

S92 AI感知决策（等 S87＋S46，G27/G26/G07，窗examples/game_ai新建）：
详见 `docs/2.5D/G26.md`（S92原话已搬家，详情以组文件为准；状态以本文件能力行为准，门槛只认§5）。

S93 对话任务（等 S33＋S78，G34，窗examples/game_rpg_grid新建--case=talk）：
详见 `docs/2.5D/G26.md`（S93原话已搬家，详情以组文件为准；状态以本文件能力行为准，门槛只认§5）。

S94 背包物品掉落（等 S15＋S33，G33，窗随game_rpg_grid）：
详见 `docs/2.5D/G26.md`（S94原话已搬家，详情以组文件为准；状态以本文件能力行为准，门槛只认§5）。

W19总指标：四项全正式包三遍最差，每窗双JSON（rpg人工玩通一单事件非0），分支/权重/切换三数全过，少一项W20不开。

#### W20 内容封口（S95–S97，靠W19，波内可并行）

S95 程序化生成（等 core/rand＋S05，窗examples/game_rogue新建--case=dungeon）：
详见 `docs/2.5D/G27.md`（S95原话已搬家，详情以组文件为准；状态以本文件能力行为准，门槛只认§5）。

S96 本地化文本与辅助设置（等 S15，窗复用不新建）：
详见 `docs/2.5D/G27.md`（S96原话已搬家，详情以组文件为准；状态以本文件能力行为准，门槛只认§5）。

S97 确定性快照回放（等 S03＋S73，G08，窗game_stage_chase复用--case=replay）：
详见 `docs/2.5D/G27.md`（S97原话已搬家，详情以组文件为准；状态以本文件能力行为准，门槛只认§5）。

W20总指标：三项全正式包三遍最差，千种子/千帧/万次三数全过，rogue人工开一局事件非0，少一项V4不开门。

#### V4 真窗（新窗4个：game_combat/game_ai/game_rpg_grid/game_rogue，其余复用）

| 窗 | 验谁 | 自动判啥 | 人工看啥 | 指标 |
|---|---|---|---|---|
| game_combat（新建） | S91 | 连击/无敌/死亡掉落JSON | 对打有手感，伤害对 | 同上帧门；同帧多命中只算一次 |
| game_ai（新建） | S92 | 发现/丢失/切换JSON | 怪巡追逃像样 | 同上帧门；丢目标3秒回巡逻 |
| game_rpg_grid（新建） | S93/S94 | 走格/战斗/对话/买卖JSON | 玩通一单不断 | 同上帧门；分支无死节点 |
| game_rogue（新建） | S95 | 连通/种子/永久死亡JSON | 每局不一样都能通 | 千种子连通100%，生成≤50ms |
| HUD复用game_dodge | S96 | 缺key回退JSON＋人工看 | 切换即生效 | 同上帧门 |
| 追车复用--case=replay | S97 | 千帧回放JSON | 回放和实跑一样 | 逐帧0差 |

#### V4 能力行（S91–S97，未开工，新人按此关门；V1/V2/V3表不动）

| 编号 | 能力 | 前置 | 窗 | 状态 |
|---|---|---|---|---|
| S91 | 战斗伤害 | S06/S21/S22 | game_combat | 未开工 |
| S92 | AI感知决策 | S87＋S46 | game_ai | 未开工 |
| S93 | 对话任务 | S33＋S78 | game_rpg_grid | 未开工 |
| S94 | 背包物品掉落 | S15＋S33 | 随S93 | 未开工 |
| S95 | 程序化生成 | core/rand＋S05 | game_rogue | 未开工 |
| S96 | 本地化文本与辅助设置 | S15 | 复用不新建 | 未开工 |
| S97 | 确定性快照回放 | S03＋S73 | 追车复用replay | 未开工 |

#### V4 门禁挂波（W19靠W18，W20靠W19）

| 波 | 跑啥 | 判 |
|---|---|---|
| W19 | S91–S94单测＋自动＋人工双JSON＋三遍最差 | 一项红整波红 |
| W20 | S95–S97单测＋自动＋人工＋三遍最差 | 三数全过才算V4关门 |

### V5 任意2.5D基础设施（S98–S105，W21–W22，2026-09-16 补）

修订2026-09-16（6）（基础设施做全）：目标改成任意2.5D游戏都能做，传奇只是其中一种。V4把玩法门打开了，本节把品质和量产的门打开：画面封顶、手感封口、声音做全、策划能干活、游戏UI有套件、工具压测封口。人和网（权威服/视野同步/服存档/社交/反作弊）单独立项，不在本节。N2/N3/N5/N7转正，N6全链转正，N8金属部分转正。

#### 任意2.5D缺口总表（先看这张，每行都有去处）

| # | 缺口 | 没有会怎样 | 去哪 |
|---|---|---|---|
| Q1 | 金属高光真材质没做 | 刀枪亮闪闪、翅膀流光做不出 | S99 |
| Q2 | 水折射真链路没做 | 水面热浪只有假扭曲 | S98 |
| Q3 | 骨骼只认子集＋8方向没做 | 换装人物8个朝向做不出 | S99（骨骼全集＋8方向） |
| Q4 | 关节链条弹簧没做 | 绳子钩索弹簧门做不了 | S100 |
| Q5 | 声音只有干声 | 山洞回音、大招压音乐做不出 | S103（全链） |
| Q6 | 脚本和数值管线没定 | 策划调数值要程序员改代码 | S102 |
| Q7 | 游戏UI没套件 | 血条背包商店每家手写一套 | S102（UI套件） |
| Q8 | 工具只有预览 | 摆怪调技能只能半自动 | S104 |
| Q9 | 压测只到集显独显各一遍 | 低端机降档、无障碍没验 | S105 |

#### W21 画面手感封口（S98–S100，靠W20，波内可并行，一窗一会话，按W块拿活）

S98 水折射真链路（等 S39/S66，扭曲＋管线，窗game_fx--case=refr，N5转正）：
详见 `docs/2.5D/G28.md`（S98原话已搬家，详情以组文件为准；状态以本文件能力行为准，门槛只认§5）。

S99 金属高光＋骨骼全集＋8方向（等 S42/S45/S62，G13/G14/G22，窗game_anim--case=hero＋game_light重验，N8金属部分转正）：
详见 `docs/2.5D/G28.md`（S99原话已搬家，详情以组文件为准；状态以本文件能力行为准，门槛只认§5）。

S100 关节链条弹簧（等 S06/S21，G23，窗game_physics--case=joints，N2转正）：
详见 `docs/2.5D/G28.md`（S100原话已搬家，详情以组文件为准；状态以本文件能力行为准，门槛只认§5）。

W21总指标：三项全正式包三遍最差，每窗双JSON（hero人工转身事件非0），金属/折射/关节三数全过，少一项W22不开。（S101已作废，手机端不在本线。）

#### W22 量产封口（S102–S105，靠W21，波内可并行）

S102 脚本内容管线＋游戏UI套件（等 S33/S73/S15，G34/G35，窗tools＋game_rpg_grid复用）：
详见 `docs/2.5D/G29.md`（S102原话已搬家，详情以组文件为准；状态以本文件能力行为准，门槛只认§5）。

S103 音频全链（等 S13/S14/S89，G28–G31，窗复用不新建，N6全链转正）：
详见 `docs/2.5D/G29.md`（S103原话已搬家，详情以组文件为准；状态以本文件能力行为准，门槛只认§5）。

S104 真编辑器（等 S50/S71/S33，G34，窗tools/editor新建，N7转正）：
详见 `docs/2.5D/G29.md`（S104原话已搬家，详情以组文件为准；状态以本文件能力行为准，门槛只认§5）。

S105 兼容压测与无障碍（等 S49/S51/S72/S96，窗复用不新建）：
详见 `docs/2.5D/G29.md`（S105原话已搬家，详情以组文件为准；状态以本文件能力行为准，门槛只认§5）。

W22总指标：四项全正式包三遍最差，表校验/声区/编辑器/降档四数全过，editor人工摆一单事件非0，少一项V5不开门。

#### V5 真窗（新分支4处＋tools/editor新窗：game_fx--case=refr、game_anim--case=hero、game_physics--case=joints、tools/editor；其余复用）

| 窗 | 验谁 | 自动判啥 | 人工看啥 | 指标 |
|---|---|---|---|---|
| game_fx--case=refr | S98 | 水晃折射JSON | 水像水，不崩主路 | 1080p一遍≤8ms，同上帧门 |
| game_anim--case=hero | S99 | 金属开关＋8向切换JSON | 亮闪闪，转身对 | 开关前后金图对比对 |
| game_physics--case=joints | S100 | 链条不断＋弹簧回位JSON | 绳子不断不漂 | 百关节≤1ms，同上帧门 |
| tools双预览＋editor | S102/S104 | 表校验＋摆运一致JSON | 策划摆完即玩 | 坏表指到行 |
| 听者窗复用 | S103 | 声区＋三器JSON＋人工听 | 山洞有回音 | 三器全开≤1ms |
| large系列复用 | S105 | 降档＋动态分辨率JSON | 低端机不崩 | 省40%以上 |

#### V5 能力行（S98–S105，未开工，新人按此关门；V1/V2/V3/V4表不动）

| 编号 | 能力 | 前置 | 窗 | 状态 |
|---|---|---|---|---|
| S98 | 水折射真链路 | S39/S66 | game_fx--case=refr | 未开工 |
| S99 | 金属高光＋骨骼全集＋8方向 | S42/S45/S62 | game_anim--case=hero | 未开工 |
| S100 | 关节链条弹簧 | S06/S21 | game_physics--case=joints | 未开工 |
| S101 | 触屏虚拟键（作废） | — | — | 作废（框架只支持桌面三端，手机端不在本线） |
| S102 | 脚本内容管线＋游戏UI套件 | S33/S73/S15 | tools＋rpg复用 | 未开工 |
| S103 | 音频全链 | S13/S14/S89 | 复用不新建 | 未开工 |
| S104 | 真编辑器 | S50/S71/S33 | tools/editor | 未开工 |
| S105 | 兼容压测与无障碍 | S49/S51/S72/S96 | 复用不新建 | 未开工 |

#### V5 门禁挂波（W21靠W20，W22靠W21）

| 波 | 跑啥 | 判 |
|---|---|---|
| W21 | S98–S100单测＋自动＋人工双JSON＋三遍最差 | 一项红整波红 |
| W22 | S102–S105单测＋自动＋人工＋三遍最差 | 四数全过才算V5关门 |

注：非Godot单点可无G：S102部分（脚本选型/表工程/UI套件工程）/S104部分（编辑器工程）/S105（压测工程）可无G，其余G已对齐（S98=G20/G16，S99=G13/G14/G16/G22，S100=G23/G24，S101=G32，S103=G28–G31，S104=G34/G35）。

### V6 联机多人（S106–S112，W23–W24，传奇奇迹档，2026-09-17 补）

修订2026-09-17（V6 联机补入）：人和网原单独立项，现补入本线。画质不重复加（底子V2 W12，封顶V5 W21 已有）。本节只补联机多人7项：技能全套、8方向玩法扩展、地图玩法层、视野同步、服存档社交、装备数值掉率、同屏压测反作弊。网同步无 Godot 现成答案处按 MMO 通用做法写并注明不标 G 号（Godot 自带多人只够小房间联机）。V5 全绿才许开 W23。

#### W23 玩法联机打底（S106–S109，靠W22，波内可并行，一窗一会话，按W块拿活）

S106 技能全套（等 S91/S46，G23/G25/G07，窗examples/game_skill新建）：
详见 `docs/2.5D/G30.md`（S106原话已搬家，详情以组文件为准；状态以本文件能力行为准，门槛只认§5）。

S107 8方向人物扩展（等 S99/S74，G05/G13，窗game_anim--case=legend）：
详见 `docs/2.5D/G30.md`（S107原话已搬家，详情以组文件为准；状态以本文件能力行为准，门槛只认§5）。

S108 地图玩法层（等 S84/S05，G12，窗game_tilemap--case=play）：
详见 `docs/2.5D/G30.md`（S108原话已搬家，详情以组文件为准；状态以本文件能力行为准，门槛只认§5）。

S109 视野同步（等 S97/S73/S85，窗examples/game_net新建--case=sync；无G号，按MMO通用做法）：
详见 `docs/2.5D/G30.md`（S109原话已搬家，详情以组文件为准；状态以本文件能力行为准，门槛只认§5）。

W23总指标：四项全正式包三遍最差，每窗双JSON（skill人工放技能事件非0，net双端对跑事件非0），命中/切换/传送/同步四数全过，少一项W24不开。

#### W24 服侧封口（S110–S112，靠W23，波内可并行）

S110 服存档与社交（等 S15/S93/S94，G33/G34，窗复用不新建）：
详见 `docs/2.5D/G31.md`（S110原话已搬家，详情以组文件为准；状态以本文件能力行为准，门槛只认§5）。

S111 装备数值与掉率（等 S94/S102，G33，窗随game_rpg_grid）：
详见 `docs/2.5D/G31.md`（S111原话已搬家，详情以组文件为准；状态以本文件能力行为准，门槛只认§5）。

S112 同屏压测与反作弊（等 S105/S90/S109，窗game_stage_chase复用--case=siege）：
详见 `docs/2.5D/G31.md`（S112原话已搬家，详情以组文件为准；状态以本文件能力行为准，门槛只认§5）。

W24总指标：三项全正式包三遍最差，存读/掉率/拦截三数全过，siege人工开一局事件非0，少一项V6不开门。

#### V6 真窗（新窗2个：game_skill/game_net，其余复用分支）

| 窗 | 验谁 | 自动判啥 | 人工看啥 | 指标 |
|---|---|---|---|---|
| game_skill（新建） | S106 | 命中/冷却/Buff/公式JSON | 放技能有手感，伤害对 | 同上帧门；冷却中放不出 |
| game_anim--case=legend | S107 | 8向切换＋换装JSON | 转身对，换装分层对 | 同上帧门；切换无跳变 |
| game_tilemap--case=play | S108 | 阻挡/传送/区域JSON | 跑图传送对 | 同上帧门；阻挡0错位 |
| game_net--case=sync（新建） | S109 | 视野/重连/定点JSON | 双端对跑一致 | 视野外0发包，逐帧0差 |
| rpg复用 | S110/S111 | 存读/掉率/归属JSON | 玩通不断，拾取对 | 同上帧门 |
| 追车复用--case=siege | S112 | 预算/拦截JSON | 几百人同屏不崩 | 拦截100%，误伤0容忍 |

#### V6 能力行（S106–S112，未开工，新人按此关门；V1/V2/V3/V4/V5表不动）

| 编号 | 能力 | 前置 | 窗 | 状态 |
|---|---|---|---|---|
| S106 | 技能全套 | S91/S46 | game_skill | 未开工 |
| S107 | 8方向人物扩展 | S99/S74 | game_anim--case=legend | 未开工 |
| S108 | 地图玩法层 | S84/S05 | game_tilemap--case=play | 未开工 |
| S109 | 视野同步 | S97/S73/S85 | game_net--case=sync | 未开工 |
| S110 | 服存档与社交 | S15/S93/S94 | 复用不新建 | 未开工 |
| S111 | 装备数值与掉率 | S94/S102 | 随S93 | 未开工 |
| S112 | 同屏压测与反作弊 | S105/S90/S109 | 追车复用siege | 未开工 |

#### V6 门禁挂波（W23靠W22，W24靠W23）

| 波 | 跑啥 | 判 |
|---|---|---|
| W23 | S106–S109单测＋自动＋人工双JSON＋三遍最差 | 一项红整波红 |
| W24 | S110–S112单测＋自动＋人工＋三遍最差 | 三数全过才算V6关门 |

注：非Godot单点可无G：S109/S110部分（服工程）/S111部分（数值工程）/S112（压测反作弊工程）可无G，其余G已对齐（S106=G23/G25/G07，S107=G05/G13，S108=G12，S110/S111=G33/G34）。S109/S112无G号处按MMO通用做法，不硬标。

### 修订 log（全文档修订只记这里一行一条，行内修订句逐步向这里归拢）

| 日期 | 改了啥 |
|---|---|
| 2026-09-15 | 52 行按前置重排，P1b 长串拆并行波，P/W/S 三号分开，能力表加开工列。 |
| 2026-09-15（2） | S41 从 W6 前移 W5（前置仅 S35）；W5 为 S35–S41 共 7 个，W6 剩 S42–S45 共 4 个。 |
| 2026-09-16（V2 立项） | W0–W9 关门不动，9.1 仍冻结（S66 部分解冻做可选管线），联机脚本先冻接口，追车 hitch 转 W10 S52。 |
| 2026-09-16（Godot 全量对齐） | G01–G35 全列入；S74–S77＋S79 归 W11，S78 归 W13；G27 导航当时暂不做（该条已取代：09-16(3)起转正S87，见§0.1与N1去向）；G35 只对齐不新开 S。 |
| 2026-09-16（2）（评审补齐） | 补 S79 真 2.5D 基向量窗、D01–D20 默认值表、门禁统一见§5、N1–N8 不含清单、真窗对照 1＋4。 |
| 2026-09-16（3）（大型化） | 平台层归别线；S80 作废并入 S87；N1 转正 S87，N4 并入 S84/S85，N6 子集并入 S89；加 V3（S84–S90，W16–W18）。 |
| 2026-09-16（4）（任意游戏） | 重审补 V4 玩法机制 7 项（S91–S97，W19–W20）：战斗/AI/对话任务/背包/随机地牢/本地化/回放；跳板弹簧进 S86，节拍时钟进 S89。 |
| 2026-09-16（5）（收敛） | 门禁归一到§5，别处只留指针；V1 台账去过程套话留证据；排期/窗表去重；行内修订向本表归拢。 |
| 2026-09-17（收敛复核） | G27改S87转正（S80作废）；卡内/口径/修订行门槛数全部指针化，数字只在§5；V1表加历史证据读数规矩（实测是证据，门槛以§5为准，未达留W10回炉）。 |
| 2026-09-17（2）（复核整改） | 09-16对齐行G27旧文标注已取代；86行/口径说明去数字复述；51行V1完成态逐行标注V1口径；S79–S83范围去S80残留；§3明确V1口径为已完成标注写法。 |
| 2026-09-16（6）（基础设施做全） | 目标改成任意2.5D都能做；N2/N3/N5/N7转正，N6全链转正，N8金属部分转正；加V5（S98–S105，W21–W22）；W扩到W22，S扩到S105；人和网单独立项不在本节。 |
| 2026-09-17（V6联机补入） | 人和网补入本线；加V6（S106–S112，W23–W24）：技能/8方向扩展/地图玩法层/视野同步/服存档社交/装备数值/同屏压测反作弊；画质不重复加；W扩到W24，S扩到S112；S109/S112无G号按MMO通用做法。 |
| 2026-09-17（2）（去手机端） | 框架只支持桌面三端，S101作废（N3同步作废，Q5删除后续Q号前移，W21改三项）；历史修订行不动，作废只记本行。 |
| 2026-09-17（3）（复核修4项） | 结论/V2–V6/号段三处过时改111项口径；压缩安卓苹果句加桌面三端口径；分组加组号与Godot号区分＋D表关门备注。 |
| 2026-09-17（4）（S52一期） | S52减分配＋卡面门限进窗：只动窗＋game四包热路径，正式包120秒×3取最差除p95外全过（最差presents7137/fps59.47/p95 17.53/p99 18.06/h33 0/分/h20 2.00/分/双金0差/parity0差），人工30秒key4＋ptr12，p95≤16红系59.93Hz屏同步底16.69ms物理不可达、换机独显显存OOM阻塞，S52记进行中，G12转进行中，W10未关门，G13不开。 |
| 2026-09-17（5）（显存七件） | 独显OOM根因收敛：1x默认（纹理set默认4→1）＋1x1闩住（120帧重探）＋进程台账（GPUI_VRAM_BUDGET_MB默认768）＋CreateBuffer回调补口＋探针独立设备释放＋设备就绪门（8秒）＋小规格描述符（GPUI_LOW_VRAM）；原生判决测试证驱动堆可用（3.66MB直分＋D24S8建镜像双PASS），悬崖在wgpu-native块分配器；独显仍死首个3.66MB depth，S52换机项继续阻塞，G13不开。 |
