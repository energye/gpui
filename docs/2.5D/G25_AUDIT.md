# G25 大型 2.5D 审计（对照 Godot，2026-10-02 六路并查汇总）

目标：以"支撑任意大型现代 2.5D 游戏"为标准，对照 Godot 参考（`gogpu/godot`，只读），查文档与实现缺口，
重审现有哪些必须推翻重写。本组只审计不定波，落地项由用户拍板后另拿 S 号。

## 1 2D 节点（Godot scene/2d 对 engine/）

必备但空白：导航寻路整套（engine 无 nav 包，S87 未开）、路径跟随 Path2D、多边形 Polygon2D、
屏幕可见通知（大世界流式开关）。常用空白：多网格同画、网格实例变形、标记点。
有号但一半未绿：镜头（S55）、视差（S55）、帧库播放器（S74）、听者（S77）、瓦片（S57）。
"有数据没行为"（见§6 推翻表）：物理、灯、骨骼、瓦片、世界。

## 2 动画系统（Godot scene/animation 对 engine/anim）

零件有（缓动数学、单值时间线、孤立状态机），组装全无：混合树、混合空间 1D/2D、
动画树、重定向 BoneMap、分层遮罩、根运动、动画事件（方法参数/播音/信号）全无；
状态机是弱占位（无 travel/条件/switch 三档/xfade），时间轴是单軌（无音频軌/方法軌/
压缩/bezier）。缓动曲线族与序列帧已满足不动。

## 3 服务器与线程模型（Godot servers/ 对我方）

Godot：约 12 进程内服（rendering、physics_2d/3d、navigation、audio、text 等）；
只有 physics 有多线程壳（`server_wrap_mt`＋命令队列），其余调用线程直调；
录制＝剔除＋建 draw list，提交＝合成＋设备执行，两段分离。
我方：UI 线串行跑全 system（`engine/loop` 注明非线程安全），光栅单 OS 线程录制提交合一单 Job，
`render/render` 声明非线程安全；实验并行池未接入主帧；总账 49 处"并行"全是开工波次，
无运行时线程模型说明。
缺口：逻辑任务池、录制提交队列解耦（提交永远单线）、物理/导航/音频独立服全无。
推翻点：`ui/raster.Loop＋pipeline_app` 的录制提交合一、`engine/loop.Frame` 串行假设。

## 4 渲染材质着色与大档（Godot scene/resources＋renderer_rd 对我方）

小档能用大档全缺：自定义着色器无（fx 只有 3 个写死名），材质混合链无，
HDR 只有半套（S66 单曲线＋降级标记，无曝光链/自动曝光/辉光阈值），
GPU 粒子万级通十万缺（改比不重启/视口裁剪/多级子发射，S90 未开），
图集小集通大包缺（3000 图/50 集，S88 未开）。
V3 规模对照（总账 1093 行：100 万瓦片/2 万实体/10 万粒子/8 小时）S84–S90 全未开工。
推翻点：`fx/custom.go` 整图 CPU 拷贝、`light` 逐像素算光、`sprite/batch.go` 按图分组。

## 5 音频输入物理（Godot servers/audio＋physics_2d＋input 对我方）

三域都是"算数有、系统层无"：多总线路由（单 Bus 不够分线＋闪避）、手柄库与键位表、
视口事件路由（UI＋玩法同屏必撞）、形状 8 种只 2 种、关节全无、寻路全无。
已对齐：位置音三旋钮、改键、手柄震动计时、捏放。
推翻点：无（纯算数包可加法补；多总线/刚体求解/事件层要另起，与旧并存）。

## 6 现有实现重审：推翻重写清单（用户拍板后另拿 S 号）

只给真会塌的（量上十倍百倍必塌），其余保留/扩展：

| 包/文件 | 结论 | 塌点 |
|---|---|---|
| engine/physics body Query/CastRay | 推翻 | O(n²)＋线性扫，千盒必爆 |
| engine/world entity＋scene | 推翻 | 4096 硬顶、无 active 集、无脏缓存 |
| engine/loop/loop.go | 推翻 | 纯串行，无并行/DAG/分摊预算挂点 |
| engine/particle emitter＋gpu | 推翻 | 8192 硬顶、全量 CPU 积分、无裁剪 |
| engine/asset/asset.go | 推翻 | 1024 硬顶、全量拷贝、无分片 |
| engine/light＋fx CPU 管线 | 推翻 | 逐像素三重循环＋每帧全图 make，必须转 GPU |
| engine/anim skeleton Pose 管线 | 推翻 | 字符串查骨＋全量更新、无裁剪无 LOD（timeline/状态机/缓动保留） |
| engine/anim statemachine | 推翻 | 与图机架构对不上，加功能只能打补丁 |
| engine/anim timeline | 推翻 | 单值軌装不下多軌/压缩/bezier，播放器须分层 |
| tilemap/sprite/batch/tex/audio/quality/debug/pool/dirty | 扩展 | 结构可留，加预算/裁剪/调 cap |
| core/camera/input/step/save/renderconv/preview | 保留 | 纯数小常数，量上百倍不塌 |

重写优先级：physics → world → loop → particle → asset → light/fx → anim（先崩的先写；
world 的 active 集是排序/物理/动画共同上游；loop 并行等 active 分片）。
单测口径备注：现全包 A–F 齐但 D 停百级小量、E 是万次重放非 8 小时、F 多离屏金非真窗，
V3 的量无覆盖——现在全绿不代表大量也绿。

## 组状态：审计完（只审计，不定波；新 S 号等用户拍板）
