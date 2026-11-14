# engine 公开 API 目录（ENGINE_API_CATALOG）

> **一句话**：这是 `engine/`（2.5D 引擎：根包 + 24 个子包）全部公开 API 的总账——逐符号列出顶层导出符号（func/type/const/var）与导出类型的导出方法，每条带中文说明，**一个不漏**（机器校验兜底）。
>
> **维护义务（硬，对齐 AGENTS.md「API 文档同步纪律」）**：每次增加、修改、删除 engine 公开 API（功能或签名），必须同步更新本文件（对应包的分类表 + §0 统计），并在主体文档 `docs/RENDER_2_5D_GAP_PLAN.md` 修订表追加一行。
>
> **判定口径**：`go/ast` 扫描 `engine/` 树全部非测试 `.go` 文件的顶层导出符号与导出类型的导出方法（与 `scripts/apidoc` 机器校验同一实现，静态快照日期见 §0；接口/结构体的成员不单列条目，随类型行说明）。
>
> **签名权威清单**：逐符号的完整签名以 `go doc` 为准（`go doc ./engine/<pkg>`），本目录的说明列不重复抄签名，只写「做什么、怎么用、边界在哪」。
>
> **说明来源**：说明基于各符号源码 doc 注释整理成中文；无源码注释的符号（建账时 72 条）已逐一对照实现补写，补写过的行不另作标记。
>
> **状态口径**：能力级接线/验收状态归 `docs/RENDER_2_5D_GAP_PLAN.md` 能力行管，本目录不逐符号标状态，只记「接口契约」（冻结口子 2.5D-IF-1.0：子包只做加法，老调用方零改编）。
>
> **核对命令（每次改动后跑，非零退出 = 文档漏了）**：
> ```bash
> go run ./scripts/apidoc
> ```

---

## 0. 包总览（2026-11-14 建账快照，go/ast 实测）

| 包 | 顶层 | 方法 | 合计 | 一句话定位 |
|----|-----:|-----:|-----:|-----------|
| `engine`（根包） | 0 | 0 | 0 | 只有包注释：冻结能力清单 2.5D-IF-1.0，无导出符号 |
| `engine/anim` | 77 | 68 | 145 | CPU 侧动效数学：帧动画、缓动、时间轴、状态机、骨骼、粒子旧接口 LOD |
| `engine/asset` | 110 | 99 | 209 | CPU 侧资产台账：种类解析、预算、依赖、状态机、图集打包工具 |
| `engine/audio` | 44 | 49 | 93 | CPU 侧 2D 位置音数学：衰减/声像、混音、听者、音乐分析 |
| `engine/camera` | 37 | 43 | 80 | 镜头与投影：视差、透视投影、真 2.5D 三件套（基向量/Y 排序/影子） |
| `engine/core` | 60 | 73 | 133 | 数与错的底座：Vec2/Vec3、Rect、Mat2D、Color、时间步、随机数、资产 ID |
| `engine/debug` | 25 | 35 | 60 | 游戏侧性能与崩溃数字：帧统计、区间打点、崩溃日志、快照编码 |
| `engine/dirty` | 6 | 12 | 18 | 逐帧重绘框跟踪（纯数字，不碰像素） |
| `engine/fx` | 50 | 34 | 84 | CPU 侧全屏特效数学：噪声/扭曲/色调映射三曲线/LUT/泛光 |
| `engine/input` | 25 | 46 | 71 | CPU 侧动作映射：设备来源、按键/手柄绑定、动作状态、输入缓冲 |
| `engine/light` | 84 | 66 | 150 | CPU 侧 2D 灯光数学：灯体、Cookie、法线/粗糙度、阴影、夜间场景 |
| `engine/loop` | 4 | 13 | 17 | 所有 2.5D 玩法跑的通用跑道：固定步循环、系统注册、联机包注入口 |
| `engine/nav` | 32 | 27 | 59 | CPU 侧网格寻路：烘焙网格、A*、Agent 队列、编码存取 |
| `engine/particle` | 42 | 48 | 90 | CPU 侧粒子发射数学：形状/发射器/更新，GPU 链路转换 |
| `engine/physics` | 46 | 42 | 88 | CPU 侧碰撞数学：盒/圆体、扫掠、宽窄相、平台吸附、台阶模式 |
| `engine/pool` | 8 | 10 | 18 | 跨帧复用 core 数字缓冲（Vec2/Color/float64），减少每帧分配 |
| `engine/preview` | 10 | 0 | 10 | 所见即所得预览帧记录与逐节点预算拆账（纯 stdlib，产出 JSON+PNG） |
| `engine/quality` | 14 | 7 | 21 | 画质分档数学：高/中/低档规格与切换（跟 save 域共用档位） |
| `engine/renderconv` | 10 | 0 | 10 | 唯一的 core↔render 类型转换口（E01 冻结：Vec2/Color/Mat2D 双向） |
| `engine/save` | 17 | 18 | 35 | 文件侧进度存取：槽位、版本迁移、背包/星级、坏档错误码 |
| `engine/sprite` | 42 | 21 | 63 | CPU 侧精灵排序：世界/特效/UI 层的盖序、批量生成、序数稳定 |
| `engine/step` | 10 | 17 | 27 | 固定步 + 分层时间：累积器、时间缩放、暂停/冻结层 |
| `engine/tex` | 77 | 46 | 123 | 压缩纹理：KTX2/Basis 解析装载、图块流送、占位图、状态机 |
| `engine/tilemap` | 38 | 88 | 126 | CPU 侧瓦片地图数学：层/物件、坐标换算、碰撞/寻路代价/遮挡、TMX 解析 |
| `engine/world` | 35 | 77 | 112 | CPU 侧实体数学：生成/回收、父子层级、变换传播、组件登记 |
| **合计** | **903** | **939** | **1842** | 公开面（导出类型的导出方法）；另 17 条导出方法名挂在未导出接收者类型上（nav 5、physics 6、world 6），包外不可达，不计入公开 API |

---

## 1. `engine/anim` — 动效数学（145 条：顶层 77 + 方法 68）

> 冻结 CPU 侧动效数学（2026-09-15/16 冻结口子，只做加法）。**只算数、不画画面**：调用方把缓动后的进度喂给既有绘制（位置/透明度/缩放）。与 `ui/animation`（界面动画，cubic-bezier 子集）刻意分开、互不 import——游戏侧要 Godot Tween 全家曲线，且曲线名要冻结进资产文件可回放。时间一律 `core.Duration`（整数毫秒），长跑不漂移；错误全走 `core.Code` 语义，nil 接收器一律不 panic。

#### 常量（40）

| 符号 | 说明 |
|------|------|
| `Linear` `InSine` `OutSine` `InOutSine` `InQuad` `OutQuad` `InOutQuad` `InCubic` `OutCubic` `InOutCubic` `InQuart` `OutQuart` `InOutQuart` `InQuint` `OutQuint` `InOutQuint` `InExpo` `OutExpo` `InOutExpo` `InCirc` `OutCirc` `InOutCirc` `InBack` `OutBack` `InOutBack` `InBounce` `OutBounce` `InOutBounce` `InElastic` `OutElastic` `InOutElastic` | 缓动曲线枚举 `Kind` 的 31 条冻结值：Godot Tween 全家（linear + sine/quad/cubic/quart/quint/expo/circ/back/bounce/elastic 各 in/out/in-out，Penner 解析式，Godot 为主参考）。新曲线只能追加在 `InOutElastic` 之后，老值永不重排 |
| `LodOff` / `LodFar` / `LodNear` | 骨骼 LOD 档位 `LodLevel`：0=视口外裁掉不更新；1=可见但远，隔帧更新；2=近处逐帧更新 |
| `DefaultLodNearDist` | 默认「近档」半径 400 游戏单位 |
| `BlendReplace` / `BlendAdditive` | 姿势混合模式 `BlendMode`：replace=按权重把 base 向 add 插值；additive=按权重把（add−setup）差值叠上 base |
| `LoopOnce` / `LoopLoop` / `LoopPingPong` | 时间轴循环模式 `LoopMode`：播完停在末帧 / 回起点循环 / 两端往返 |

#### 变量（1）

| 符号 | 说明 |
|------|------|
| `CurrentSkeletonVersion` | 引擎侧 Spine 子集契约版本（4.0）：major 为 3 或 4 的文件都能载，其余报 VersionMismatch；major 对上后 minor 按 core 规则校验 |

#### 类型（22）

| 符号 | 说明 |
|------|------|
| `Kind` | 缓动曲线枚举（int），值冻结见常量行 |
| `LodLevel` | 单骨架更新档位（off/far/near）；速率只归调度器管，不碰骨骼数学 |
| `LodPolicy` | 持有「近档半径」的策略对象；视口逐帧传入，一份策略服务所有窗口尺寸 |
| `LodFrame` | 一帧调度结论：每骨架档位 `Levels`、每骨架是否更新 `Updates`、off/far/near 计数与想要更新的总数 `Wanted` |
| `LodScheduler` | 按帧计数的 LOD 调度器：far 档靠帧号奇偶交错，一半远骨架每帧各跑一半，调用方无需自持状态 |
| `BlendMode` | 姿势混合模式枚举（replace/additive），见常量行 |
| `Bone` | 一根 setup 骨（Spine 术语）：名字、父骨名（根为空）、x/y/rotation/scale/shear/length/color；rotation 与 shear 单位是度，scale 缺省 1，color 缺省不透明白 |
| `Slot` | 一个绘制槽：绑到某根骨、指名 setup 附着物（可空）、混合关键字（normal/additive/multiply/screen）与颜色 |
| `VertexWeight` | 一根骨对一个网格顶点的影响：骨下标 + 骨空间偏移 + 权重（附着物变换在装载时已烘进偏移） |
| `Attachment` | 皮肤下一个可画物：类型只认 region/mesh（其他装载时报 Unsupported）、图集图 ID、顶点/UV/三角形索引/hull 数、逐顶点骨权重（无权重为 nil）；`Verts` 恒为骨空间静置位 |
| `Skin` | 一套命名的附着物集合：槽名 → 附着物名 → 可画物；map 归 Skeleton 所有，调用方拿到的是拷贝 |
| `IKConstraint` | 一条冻结 IK 规则：1~2 根受控骨 + 目标骨 + Mix（0 关 1 全）+ 弯肘方向 BendPositive；Compress/Stretch 只解析上报、暂不动骨（预留） |
| `TransformConstraint` | 一条变换约束：把目标骨的世界运动按各通道 Mix（rotate/x/y/scaleX/scaleY/shearY，0~1）混进受控骨 |
| `Skeleton` | 冻结的 Spine 子集整体：骨/槽保持文件序、皮肤名排序可回放、drawOrder 初始等于槽序只能经 `SetDrawOrder` 改 |
| `BoneLocal` | 一根骨的本地姿势覆盖（父骨空间），字段镜像 `Bone` 的 TRS，供 `Reset` 精确还原 setup |
| `Pose` | 活骨架姿势：每根骨的本地覆盖 + 缓存的世界矩阵；`Update` 自根向下重算世界，查询会先自动刷新，调用方永远看不到过期姿势 |
| `State` | 混合状态机里的一个动作：名字（调用方自持，引擎不写死文案）、合法跳转目标 `CanTo`（空=终态）、离场混合时长 `Blend`（零=瞬时切） |
| `Machine` | 名字动作间的线性交叉混合状态机：只算权重数字，调用方把 `Weights` 喂给骨架 BlendReplace 或逐帧动画挑选器 |
| `LoopMode` | 时间轴循环模式枚举，见常量行 |
| `Key` | 一个关键帧：`At` 时刻取值 `Value`，向下一个关键帧按 `Ease` 缓动（Ease 挂在段起点关键帧上，末帧保持） |
| `Event` | 一条提示事件：`At` 时刻越过时抛出 `Name`；名字归调用方（斩击音效/呼叫/显隐等），引擎不写死 |
| `Timeline` | 浮点轨道时间轴：固定时长 + 轨道关键帧 + 事件表 + 播放头；只算数，调用方把采样喂给绘制、把事件名映射到动作 |

#### 函数（14）

| 符号 | 说明 |
|------|------|
| `Valid` | 判断 `k` 是否是冻结曲线名 |
| `Name` | 返回 `k` 的冻结资产名，非法值返回 "unknown" |
| `Parse` | 按冻结名查曲线；空名/未知名报 `core.InvalidArg`，绝不猜曲线 |
| `AllKinds` | 按声明序返回全部冻结曲线 |
| `Ease` | 把线性进度 t 整形成曲线进度：先钳到 [0,1]（NaN 归 0），钳端点精确返回 0/1，未知曲线退化为线性，结果恒有限不 panic；back/elastic 中途越出 [0,1] 是设计内回弹，其余恒在 [0,1] 单调升 |
| `Lerp` | `a+(b-a)*Ease(kind,t)` 混合：t 处理同 Ease（NaN 停在 a），a/b 原样透传，永不 panic |
| `NewLodPolicy` | 建 LOD 策略；nearDist 必须有限且 ≥0，坏输入报 InvalidArg |
| `ShouldUpdate` | 判断某槽位本帧要不要更新姿势：near 每帧、far 按帧号奇偶交错（各跑一半）、off 永不 |
| `NewLodScheduler` | 围绕策略建调度器；坏策略落到默认半径而不让窗失败 |
| `ParseSkeleton` | 解码一份 Spine JSON 子集：空输入 InvalidArg、坏 JSON BadData、外来 major VersionMismatch、物理/路径约束与非 region/mesh 附着物 Unsupported；悬空父骨、重名、坏数字一律 BadData，绝不猜骨架 |
| `LoadSkeletonFile` | 读路径并解析：空路径 InvalidArg、文件缺失 NotFound、数据错沿用 ParseSkeleton 错码 |
| `NewPose` | 在 setup 姿势上建 Pose；nil 骨架报 InvalidArg |
| `NewMachine` | 建空状态库；动作经 `AddState` 注册、起点经 `Start` 指定 |
| `NewTimeline` | 建时长为 dur 的空时间轴；dur 必须为正（坏值报 InvalidArg 返回 nil），播放头从 0 正向播放 |

#### 方法（68，按接收者分组）

| 符号 | 说明 |
|------|------|
| `LodLevel.String` | 返回档位的稳定日志名 |
| `LodPolicy.NearDist` | 返回近档半径 |
| `LodPolicy.Classify` | 按视口给包围盒定档：视口外 off、视口内距中心近档半径内 near、其余可见 far；空/坏输入停 off 且 ok=false，不 panic |
| `LodPolicy.Batch` | 一个视口按序给全部包围盒定档；不改输入、结果每次新算，坏槽位停 off 不影响其余 |
| `LodScheduler.Policy` | 返回调度策略 |
| `LodScheduler.Tick` | 返回帧计数，nil 返回 0 |
| `LodScheduler.Reset` | 帧计数归零，nil 安全 |
| `LodScheduler.NextFrame` | 按当前帧号给全部包围盒定档并推进计数；不改输入、levels/updates 都是新切片；nil 调度器或坏视口全部停 off；帧号 2^64 自然回绕，奇偶调度不破 |
| `BlendMode.String` | 返回混合模式的稳定日志名 |
| `Skeleton.Version` | 返回解析出的 Spine major.minor，nil 骨架报 0.0 |
| `Skeleton.BoneCount` / `Skeleton.SlotCount` | 骨数/槽数，nil 报 0 |
| `Skeleton.BoneIndex` | 按名查骨下标：空名 InvalidArg、未知 NotFound |
| `Skeleton.Bone` | 返回 setup 骨拷贝，绝不别名骨架，调用方改不动 setup |
| `Skeleton.SlotIndex` / `Skeleton.Slot` | 按名查槽下标 / 返回 setup 槽拷贝 |
| `Skeleton.SkinNames` | 返回全部皮肤名（已排序，可回放） |
| `Skeleton.CurrentSkin` | 返回当前激活皮肤名，无皮肤返回 "" |
| `Skeleton.SetSkin` | 换激活皮肤：未知皮肤 NotFound 且保旧皮肤、空名 InvalidArg |
| `Skeleton.Attachment` | 返回激活皮肤里一个可画物的深拷贝；槽/附着物/皮肤未知 NotFound、空名 InvalidArg，拷贝不别名骨架 |
| `Skeleton.DrawOrder` | 按绘制序返回槽名（新切片） |
| `Skeleton.SetDrawOrder` | 整体替换绘制序：输入必须是槽名排列，否则 InvalidArg 保旧序；不保留输入切片 |
| `Skeleton.IKNames` / `Skeleton.TransformNames` | 按文件序返回 IK 规则名 / 变换约束名 |
| `Skeleton.IK` | 按名返回一条 IK 规则拷贝 |
| `Pose.Skeleton` | 返回姿势所属骨架，nil 姿势返回 nil |
| `Pose.Reset` | 每根骨回到 setup 变换，nil 安全 |
| `Pose.SetBoneLocal` | 覆盖一根骨的本地姿势：未知骨 NotFound、空名与 NaN/Inf InvalidArg，失败不动任何数 |
| `Pose.BoneLocal` | 读一根骨本地姿势拷贝，未知骨 NotFound |
| `Pose.WorldTransform` | 返回一根骨的世界矩阵（值拷贝不别名），未知骨 NotFound |
| `Pose.WorldPos` | 返回一根骨的世界原点，未知骨 NotFound |
| `Pose.ApplyIK` | 解一条冻结 IK 规则并按 Mix 混入：单骨瞄准目标、双骨解析解弯肘（方向冻结）；未知规则 NotFound、nil 姿势 InvalidArg；距目标 1e-9 内视为已解保持不动，长跑不因 NaN 抖 |
| `Pose.ApplyTransform` | 把一条变换约束按各通道 Mix 混进受控骨：世界位置经父逆矩阵插值（旋转父骨也能落准）、世界旋转按 MixRotate、本地缩放与剪切按各自 Mix |
| `Pose.ApplyPhysics` | 预留的惯性入口：恒报 Unsupported（空名先报 InvalidArg），让调用方分得清「没实现」和「参数错」；装载期 JSON "physics" 键同样拒绝 |
| `Pose.BlendFrom` | 把 base 与 add 混进接收者：replace 逐通道按权重插值、additive 按（add−setup）叠加；三个姿势必须同骨架、权重 [0,1] 有限、模式冻结，失败不动任何数 |
| `Pose.SkinVertices` | 返回激活皮肤里一个附着物的世界顶点：无权重顶点随槽骨走、有权重按影响混骨矩阵；未知槽/附着物 NotFound、空名 InvalidArg，结果每次新算不别名 |
| `Machine.AddState` | 注册一个动作：名字非空唯一、Blend 非负、CanTo 条目非空；坏动作报 InvalidArg 且不落库，CanTo 拷贝防调用方后续改库；允许前向引用（后加的目标），Request 时未知仍报 NotFound |
| `Machine.Has` / `Machine.StateCount` / `Machine.States` | 名字是否存在 / 动作数 / 全部动作名（排序新切片） |
| `Machine.Start` | 停到指定动作且无混合在途：空名 InvalidArg、未知 NotFound，出错不动旧当前 |
| `Machine.Current` | 返回机器去向（混合中=目标，稳定=当前），Start 前返回 "" |
| `Machine.From` / `Machine.To` | 混合起点（稳定时=当前）/ 混合目标（无混合时 ""） |
| `Machine.Blending` | 是否有交叉混合在途 |
| `Machine.Progress` | 混合进度 [0,1]：请求时 0、稳定 1、Start 前 0 |
| `Machine.Weights` | 返回线性交叉权重对 (fromW,toW)，和恒 1：稳定 (1,0)、混合中 (1−p,p)；toW 可直接喂骨架 BlendReplace 或透明度插值 |
| `Machine.BlendDur` / `Machine.Elapsed` | 在途混合时长 / 已推进量（稳定时 0） |
| `Machine.Request` | 请求切到目标动作（可中途改目标）：请求当前稳定态或混合目标是静默成功；请求混合起点则取消混合弹回；出站 Blend 为零则瞬时切。错误绝不移动状态机：空名/被起点禁止/Start 前请求报 InvalidArg，未知目标报 NotFound |
| `Machine.Update` | 推进在途混合并报告是否仍在混合：dt≤0 不动，到达或越过时长精确落到目标 |
| `LoopMode.String` | 返回循环模式的稳定日志名 |
| `Timeline.Duration` / `Timeline.Loop` | 时长（nil 报 0）/ 循环模式（nil 报 LoopOnce） |
| `Timeline.SetLoop` | 设循环模式：坏值 InvalidArg 保旧值，nil 时间轴也 InvalidArg |
| `Timeline.AddKey` | 往轨道放一个关键帧：轨道名非空、At 落在 [0,时长] 且该轨道未占用、Ease 必须是冻结曲线；坏帧 InvalidArg 且不落库 |
| `Timeline.AddEvent` | 放一条事件：At 落在 [0,时长]、名非空；同刻事件保持插入序；坏事件 InvalidArg 不落库 |
| `Timeline.TrackNames` / `Timeline.KeyCount` / `Timeline.EventCount` | 有关键帧的轨道名（排序）/ 某轨道帧数（未知 0）/ 事件总数 |
| `Timeline.OnEvent` | 设事件回调（nil 清除）；回调在 Update 内同步执行，禁止回调里再进同一时间轴 |
| `Timeline.Pos` / `Timeline.IsPlaying` | 播放头位置（nil 报 0）/ 是否推进中（once 播完为 false，需 Seek+Resume 才重播） |
| `Timeline.Pause` / `Timeline.Resume` | 冻结播放头 / 原位恢复；均 nil 安全；once 播完后 Resume 不回卷，要重播先 Seek |
| `Timeline.Seek` | 把播放头移到 at（钳到 [0,时长]、保持正向），不动播放态、不触发事件；nil 时间轴 InvalidArg |
| `Timeline.Sample` | 绝对时刻读某轨道采样值，不动播放头不触发事件；nil 时间轴或未知轨道 ok=false |
| `Timeline.Update` | 推进播放头并按序返回本帧越过的事件：dt≤0 或暂停返回 nil；once 播完停在末尾返回 nil；一次 Update 跨满一整圈时每个事件只报一次（按越过序）而非无限重复；pingpong 一帧内双向越过同一事件各报一次 |

## 2. `engine/asset` — 资产台账（209 条：顶层 110 + 方法 99）

> 冻结 CPU 侧资产台账（2026-09-15 冻结口子，只做加法）。一个不透明载体 + 一种块格式：`KindRaw` 承载任意字节不解析（地图/骨骼/存盘先走它，等各自能力冻结），`KindTextureKTX2` 承载 BC1 KTX2 并经 `engine/tex` 解码（上传与像素字节数诚实）。Manager 一本台账：后台 `Request` + 前台 `Poll/Wait` 保证玩法不卡；`Load/Ref/Unload` 经 core 引用计数，纹理/地图/存盘共用一本账；`Dependencies/Dependents` 记谁依赖谁；版本 + 哈希配对载荷与清单。其余种类名（basis/spine/tmx/atlas/png…）只解析到「认出种类」即报 Unsupported，绝不猜载荷。错误全走 core 错码，nil Manager/Asset 不 panic。含四个子能力：图集打包（离线工具 + 游戏装载，S65 缝隙参数化）、大资源包预算（S88）、文件热重载（12.3 单 id 换装）、按集合重载（一图编辑只换本集合）。

#### 常量（53）

| 符号 | 说明 |
|------|------|
| `MaxAssets` | 单个 Manager 缓存 id 上限（1024） |
| `MaxBytes` | 单个 Manager 保留载荷字节上限（64MB） |
| `MaxAssetBytes` | 单个载荷字节上限（8MB） |
| `MaxDeps` | 单个资产的依赖数上限（16） |
| `MaxIDLen` | id 文本字节长度上限（128） |
| `KindUnknown` | `Kind` 零值：未识别载体 |
| `KindRaw` | 不透明字节载体，内部不解析 |
| `KindTextureKTX2` | KTX2 + BC1 载体，经 `engine/tex` 解码 |
| `StateEmpty` / `StateLoading` / `StateReady` / `StateMissing` / `StateFailed` | 资产生命周期 `State`：无记录 / 后台装载在途 / 字节已缓存 / NotFound 占位 / BadData·Unsupported·OutOfMemory·VersionMismatch 占位 |
| `MaxSprites` | 单图集条目上限（1024） |
| `MaxAtlasSize` | 单图集边长上限（4096px） |
| `MaxAtlasNameLen` | 单精灵名字节上限（128） |
| `AtlasPad` | 每个精灵四周保留的透明边距（1px） |
| `MaxAtlasJSONBytes` | 单图集 JSON 文件字节上限（1MB） |
| `MinAtlasGutter` / `MaxAtlasGutter` | 透明缝隙宽度下限 2 / 上限 4 px（S65） |
| `MinAtlasExtrude` / `MaxAtlasExtrude` | 边缘外扩（重复边缘像素防渗色）下限 1 / 上限 2 px（S65） |
| `DefaultAtlasGutter` / `DefaultAtlasExtrude` | S65 默认缝隙 2px / 默认外扩 1px |
| `ToolAtlasSizeSmall` / `ToolAtlasSizeLarge` | 工具图集两档边长：2048 / 4096 |
| `MaxLargeSets` | 单份大清单可装载集合上限（64） |
| `MaxLargeSetMembers` | 单集合成员上限（512） |
| `MaxLargeNameLen` | 集合/估算行名字节上限（128） |
| `MaxLargeEstimates` | 单清单纯预算估算行上限（64） |
| `MaxLargeEstimateCount` | 单估算行图数上限（10000） |
| `MaxLargeTotalImages` | 全部估算行图数上限（10000） |
| `MaxLargeDimension` | 估算图单边像素上限（8192） |
| `MaxLargePixels` | 估算图像素上限（8192×8192） |
| `MaxLargeBroken` | 保留的断链摘要条数上限（1024） |
| `LargeScaleImages` / `LargeScaleSets` | 全牌组目标（3000 图 / 50 集）：估算行按 w/h/format 数学证明显存，真实文件只存少量集合证重载 |
| `TierPerformance` / `TierBalanced` / `TierQuality` | 画质策略档：性能=先弃空闲集合不预载；均衡=预载活动集、其余流送；画质=常驻不弃、换集宁可失败 |
| `MaxWatches` | 单 Watcher 监视 id 上限（1024） |
| `MaxWatchIDLen` / `MaxWatchPathLen` | 监视 id / 路径文本字节上限（128 / 1024） |
| `PollInterval` | 调用方轮询无文件事件时的兜底间隔（50ms）；文件事件永远优先 |
| `HotEmpty` / `HotWatching` / `HotDirty` / `HotFailed` | 热重载生命周期 `HotState`：未监视 / 已监视同步 / 磁盘已改待 Reload / Reload 失败占位（最后好快照仍可读） |
| `MaxSets` | 单 SetReloader 集合上限（256） |
| `MaxSetNameLen` | 集合名字节上限（128） |
| `MaxSetMembers` | 单集合成员上限（256） |
| `MaxReloadLogs` / `MaxBrokenReports` | 重载日志 / 断链报告保留条数上限（各 1024） |

#### 变量（2）

| 符号 | 说明 |
|------|------|
| `CurrentVersion` | Manager 清单版本（1.0）：加字段升 Minor，破坏形状升 Major |
| `CurrentAtlasVersion` | Pack 写出的图集 JSON 版本（1.0） |

#### 类型（29）

| 符号 | 说明 |
|------|------|
| `Kind` | 载体种类枚举（raw / KTX2+BC1），见常量行 |
| `State` | 单 id 生命周期枚举，见常量行 |
| `Stats` | Manager 计数器：Ready 条数 `Count`、保留字节 `TotalBytes`、装载/卸载/请求累计 |
| `Asset` | 一份不可变载荷快照：id、种类、版本、FNV-1a 哈希、依赖、状态、上传/解码字节数、数据；nil 资产的方法返回零值不 panic |
| `Manager` | 资产台账：缓存字节 + 共享 core 引用计数账本；一本账服务纹理/地图/存盘 |
| `Input` | 喂给 Pack 的一张图：id、宽高、RGBA8 像素（行主序左上）、轴心（0..1）、九宫格四边距（像素） |
| `Entry` | 一个已打包精灵：名字 + 图集内矩形；轴心与九宫格从输入原样带过 |
| `Atlas` | 一张打包完成的图集：版本、尺寸、条目表 + 合成后的 RGBA8 像素；仅解析的图集只有摆位，Pixels 为 nil |
| `PackOptions` | S65 缝隙开关：Gutter（2..4 透明 px）+ Extrude（1..2 外扩 px）；零值经 `Normalized` 落默认（2/1） |
| `EdgeBleed` | 一条检出的描边黑线：图集坐标、颜色、所属精灵（空=落在图集边框上，不属于任何外扩环） |
| `Tier` | 大资源包画质档枚举（performance/balanced/quality），只决定策略不锁字节上限 |
| `TierPolicy` | 单档策略：是否常驻、是否预载、是否弃空闲、是否流送 mip |
| `LargeScale` | 估算行代表的全牌组目标声明（图数/集数/备注） |
| `LargeMember` | 大清单里一个文件背书的图：宽高/格式/文件/版本/字节数/依赖（宽高×格式数学证明冻结的尺寸与显存，文件证重载） |
| `LargeSet` | 一个可装载集合：名字 + 文件背书成员表 |
| `LargeEstimate` | 一行纯预算估算：count 张图共用同一 w/h/format，不存真图，显存 = count × 单图数学 |
| `LargeManifest` | S88 集合表 + 尺寸数学：少量文件集合证重载，估算行证 3000 图预算 |
| `LargeLink` | `large_links.json` 里一条冻结断链（集合/图/缺失依赖/错码） |
| `LargeLinksFile` | 断链报告文件槽（links 数组） |
| `BrokenSummary` | 断链汇总：总数、按集合、按错码、明细列表 |
| `Budget` | 一份大清单 + 活台账的预算器：Manager 管字节、SetReloader 管集合表、TierPolicy 管策略；nil 不 panic |
| `ReloadSetResult` | 单集合重载记录：集合名、耗时、成功/失败数 |
| `HotState` | 热重载生命周期枚举，见常量行 |
| `HotStats` | Watcher 计数器：监视数、成功/失败 Reload 数、Poll 发现的文件事件数 |
| `Watcher` | 单 id↔单文件热重载器：Poll 只 stat（大小+mtime）标脏不装载，Reload 经 Manager.LoadFile 只换该 id（改纹理不碰地图与存盘）；失败保旧快照可读并停在 Failed；全方法加锁 |
| `SetMember` | 集合成员：把一张图连同它的装载形状（种类/版本/依赖）钉进集合 |
| `ReloadLog` | 逐图重载记录（集合/图/字节/成败/错码/耗时），用于卡住定位 |
| `BrokenLink` | 一条断链：哪个集合哪张图缺哪个依赖；`Cause` 保留 core 错误可供 CodeOf 归类 |
| `SetReloader` | 按集合组织的重载器：一个 id 只属一个集合，一图编辑只重载本集合；只读 Manager 路径不改写 |

#### 函数（26）

| 符号 | 说明 |
|------|------|
| `ParseKind` | 按冻结名查载体种类：空名 InvalidArg、未冻结名（basis/spine/tmx/atlas…）Unsupported |
| `NewManager` | 建空台账 |
| `HashBytes` | 数据的 FNV-1a 64 位哈希 |
| `Pack` | 离线把小图上架成一张图集（无 GPU 无窗口）：按高、宽、名排序，同集合永远落同矩形；每精灵四周留 AtlasPad 透明边 |
| `Parse` | 读冻结图集 JSON：多余键点名未冻结字段报 Unsupported，绝不猜摆位；解析出的图集只有摆位 |
| `Load` | 读路径并按图集 JSON 解析 |
| `DefaultPackOptions` | 返回默认缝隙参数（gutter 2 / extrude 1） |
| `PackWithOptions` | 参数化缝隙上架：外扩环重复边缘像素，环外缝隙全透明（maxDiff=0）；排序与错码与 Pack 一致 |
| `SpriteToAtlasUV` | 精灵本地 uv（0..1 左上原点）→ 图集 uv；rotated 选顺时针 90° 存储（存储矩形已宽高互换）；Entry 零值安全 |
| `AtlasToSpriteUV` | 上面的逆变换，同一 rotated 开关；浮点误差内往返一致 |
| `SeamTransparentMaxDiff` | 缝隙像素（每条目矩形外扩 extrude 之外）离透明的最大通道距离，0=全透明；nil/解析图集报 0 |
| `IsSeamTransparent` | 外扩环之外的缝隙是否全透明 |
| `ValidateToolAtlasSize` | 工具图集门禁：宽高必须各为 2048 或 4096，否则 InvalidArg 并报出实际值 |
| `PackToolAtlas` | 冻结工具预算下打包：先过门禁，再原样复用 PackWithOptions 缝隙；图集按内容收紧，门禁只钉预算不钉输出尺寸 |
| `ToolAtlasToSpriteUV` | 工具侧带校验的旋转逆映射：复用 AtlasToSpriteUV 数学；uv 须在 0..1、条目矩形须在图内 |
| `CheckStrokeBlackLine` | 扫描描边黑线：精灵外扩环内或图集边框上的不透明黑像素（容差冻结为 0）；精灵内部像素是画面本身永不误报；命中逐条带坐标可定位 |
| `ParseTier` | 冻结档名映射；空名/未知名 InvalidArg |
| `PolicyFor` | 返回档位对应的冻结策略 |
| `EstimateUpload` | 单图 GPU 上传字节估算：BC1/ASTC 用 tex 块几何（4×4 块 8/16 字节）、rgba8 为 w×h×4、raw 无上传；坏边长 InvalidArg、超面积 OutOfMemory、未知格式 Unsupported |
| `EstimatePixels` | 单图解码后 RGBA8 字节估算（w×h×4），raw 为 0；错码同上 |
| `LoadLargeManifest` | 读入并校验大清单：估算行按数学证显存，只有文件集合在重载时才要真文件 |
| `LoadLargeLinks` | 读冻结断链报告文件 |
| `SummarizeBroken` | 断链按集合与错码归组；nil 输入不 panic，明细截到 MaxLargeBroken 并排序可回放 |
| `NewBudget` | 在清单与档位上建预算器；nil 清单在使用时报 InvalidArg、查询返回零值 |
| `NewWatcher` | 在 Manager 上建热重载器；nil mgr 时 Watch/Reload 报 InvalidArg、查询返回零值 |
| `NewSetReloader` | 在 Manager 上建集合重载器；nil mgr 重载报 InvalidArg、查询返回零值 |

#### 方法（99，按接收者分组）

| 符号 | 说明 |
|------|------|
| `Kind.String` / `Kind.Valid` | 稳定日志名 / 是否可装载载体 |
| `State.String` | 生命周期稳定日志名 |
| `Asset.ID` / `Asset.Kind` / `Asset.Version` / `Asset.Hash` / `Asset.Size` / `Asset.State` | 读快照的 id / 载体 / 版本 / FNV-1a 哈希 / 载荷字节 / 生命周期 |
| `Asset.Deps` / `Asset.Bytes` | 依赖列表拷贝 / 载荷字节拷贝 |
| `Asset.UploadSize` / `Asset.PixelSize` | KTX2 的 GPU 上传字节 / 解码 RGBA8 字节（其他载体 0） |
| `Asset.Equal` | 两份快照是否逐位相同 |
| `Manager.Load` | 缓存数据并占一个引用：空 id/数据、未知种类、坏依赖、超预算一概不落库；版本不符 VersionMismatch；坏 KTX2 透传 tex 错码；「缺 id」只在 LoadFile/Poll 报 NotFound |
| `Manager.LoadFile` | 读路径缓存到 id：空路径 InvalidArg；文件缺失 NotFound 并落 Missing 占位 |
| `Manager.Request` | 后台装载字节：goroutine 启动前拷贝数据（调用方可复用）；装载期间重复请求是 no-op |
| `Manager.RequestFile` | 后台装载路径 |
| `Manager.Poll` | 非阻塞查生命周期：未知 id = StateEmpty+NotFound；Loading/Ready 报 nil；Missing/Failed 重放存储原因 |
| `Manager.Wait` | 阻塞到 id 离开 Loading 再返回快照；未知 id NotFound；等待封顶 5 秒，卡死的 worker 拖不垮玩法 |
| `Manager.Ref` | 给 Ready id 加一个引用并返回新快照；缺/坏 id 返回占位+原因；台账超预算 OutOfMemory |
| `Manager.Unload` | 掉一个引用：未知 id NotFound、无活引用 InvalidArg 不动；缓存字节留着便于重载，要释放用 Evict |
| `Manager.Evict` | 无活引用时释放缓存字节：活引用 InvalidArg、未知 NotFound |
| `Manager.Placeholder` | 返回 id 的可用空快照（缺资产用它，调用方永远不用判 nil）；空 id 返回 nil |
| `Manager.Dependencies` / `Manager.Dependents` | id 需要谁（拷贝）/ 谁 Ready 且列出 id（排序可回放） |
| `Manager.VersionOf` / `Manager.HashOf` / `Manager.StateOf` | Ready id 的版本 / 哈希 / 生命周期 |
| `Manager.LiveCount` / `Manager.Loaded` / `Manager.Ready` | id 活引用数 / 是否有活引用 / 是否有缓存字节 |
| `Manager.Stats` / `Manager.Count` / `Manager.TotalBytes` | 计数器拷贝 / Ready 条数 / 保留字节 |
| `Entry.Name` / `Entry.X` / `Entry.Y` / `Entry.W` / `Entry.H` | 精灵名 / 图集内左上角 x/y / 宽 / 高 |
| `Entry.Pivot` / `Entry.PivotX` / `Entry.PivotY` | 轴心对（0..1）/ 横轴 / 纵轴 |
| `Entry.Nine` | 九宫格 left/top/right/bottom 边距（像素） |
| `Atlas.Version` / `Atlas.Width` / `Atlas.Height` / `Atlas.Count` | 图集版本 / 宽 / 高 / 条目数 |
| `Atlas.Entry` / `Atlas.Find` | 按下标取条目拷贝 / 按名找条目 |
| `Atlas.Pixels` / `Atlas.At` | 合成 RGBA8 像素拷贝（解析图集 nil）/ 读单像素（解析图集 ok=false） |
| `Atlas.Equal` | 版本、尺寸、条目、像素全同才相等；解析图集只与解析图集相等 |
| `Atlas.Encode` | 渲染冻结 JSON（版本/尺寸/逐精灵名·矩形·轴心·九宫格）；像素随图走，永不进 JSON |
| `Atlas.Save` | Encode 写盘，供离线工具链 |
| `PackOptions.Normalized` / `PackOptions.Validate` | 零字段落默认 / 越界（gutter 出 2..4、extrude 出 1..2）拒绝 |
| `Tier.String` | 档位稳定文件名 |
| `Budget.Manifest` / `Budget.Manager` / `Budget.Reloader` / `Budget.Tier` / `Budget.Policy` | 读冻结表 / 活台账 / 集合重载器 / 档位 / 当前档策略（未设返回 nil/零值） |
| `Budget.SetTier` | 只切策略；预算即时重算，从不落存储 |
| `Budget.RegisterAll` | 把全部文件集合钉进重载器；估算行不注册，只喂显存数学 |
| `Budget.EstimateSet` | 按集合名算上传/像素/文件字节与图数（w/h/format 数学 + 自带文件大小） |
| `Budget.EstimateTotal` | 全牌组显存：只算估算行（已是 3000 图规模）；文件集合不重复累加，总数不双计 |
| `Budget.CheckBudget` | 活台账是否还装得下 MaxBytes；估算总数不在此判失败，只经 SuggestEvictions 喂策略；nil 预算 InvalidArg |
| `Budget.SuggestEvictions` | 台账要腾地方时按当前策略从大到小排出可弃集合：画质档常驻返回 nil，性能/均衡先弃最大估算；顺序确定可回放 |
| `Budget.ReloadSet` | 只重载指定集合：其余集合字节不动；缺载荷报 NotFound（带集合+图名）进断链槽，绝不冲掉最后好画面 |
| `Budget.VerifyAll` | 校验全部已注册集合并按序聚合断链；报告同时进重载器槽位 |
| `HotState.String` | 热重载状态稳定日志名 |
| `Watcher.Manager` | 被监视的台账（未设返回 nil） |
| `Watcher.Watch` | 把 id 钉到路径（装载形状即 Reload 重放的形状）并立即装载当前文件首快照：空 id/路径、未知种类、坏依赖 InvalidArg/OutOfMemory 不落监视；缺/坏文件报台账原因且不落监视，修好文件再 Watch |
| `Watcher.Unwatch` | 摘掉监视但保留台账字节便于再监视；未知 id NotFound、nil InvalidArg |
| `Watcher.Watched` / `Watcher.WatchedIDs` / `Watcher.Count` | 是否监视中 / 监视 id 列表（排序可回放）/ 监视数 |
| `Watcher.StateOf` / `Watcher.CauseOf` / `Watcher.PathOf` | id 生命周期 / 重放上次 Reload 失败（成功或未跑过为 nil）/ 监视路径（未监视 ""） |
| `Watcher.Poll` | stat 全部监视文件并标脏（不装载，删文件也标脏使 Reload 报 NotFound 而非沉默）；变更检测=大小+mtime 一次 stat 零读文件，稳态零文件 IO；写文件快过文件系统时间戳刻度的调用方须自行 bump mtime，否则变更到下一刻度才可见（同经典 make watcher） |
| `Watcher.Reload` | 经 LoadFile 重读该 id 并只换它：干净装载回 Watching；失败保最后好快照、停 Failed 且不采纳坏指纹（下次 Poll 仍报 Dirty 直到修好）；干净 id 也可显式刷新；未知 id NotFound、nil 报 InvalidArg |
| `Watcher.ReloadDirty` | Poll 一次后重载全部 Dirty id 并按 id 序返回快照；失败者停 Failed 不挡其余；nil InvalidArg |
| `Watcher.Stats` | 计数器拷贝 |
| `BrokenLink.Error` / `BrokenLink.Unwrap` | 渲染「集合+图+缺谁」（永不空）/ 取出包裹的 core 原因供 CodeOf |
| `SetReloader.Manager` | 被监视台账（未设 nil） |
| `SetReloader.SetLive` / `SetReloader.Live` | 切换在线闸门（true 时拒绝逻辑换装）/ 读闸门 |
| `SetReloader.RegisterSet` | 把成员钉到集合名：一个 id 只能属一个集合 |
| `SetReloader.Sets` / `SetReloader.Members` / `SetReloader.SetOf` | 集合名列表（排序可回放）/ 单集合成员拷贝（按 id 序）/ id 所属集合 |
| `SetReloader.Logs` / `SetReloader.Reports` | 重载日志槽拷贝 / 断链报告槽拷贝 |
| `SetReloader.ReloadOne` | 重载集合内一张图：其余集合不动；缺/坏载荷保最后好画面可读、报 BrokenLink（带集合+图），不 panic；在线模式拒绝逻辑换装并给原因 |
| `SetReloader.ReloadOneFile` | 从路径重载集合内一张图：缺文件 NotFound（带集合+图）且保最后好画面 |
| `SetReloader.VerifyLinks` | 校验集合全部成员及其依赖：断链进报告槽（集合/图/缺失项） |

## 3. `engine/audio` — 位置音与音乐链（93 条：顶层 44 + 方法 49）

> 冻结 CPU 侧 2D 位置音数学（2026-09-15 冻结，10-01 加听者单例，只做加法）。**只算数、不出声**：调用方把算出的增益/声像喂给平台音频后端或测试振荡器。距离衰减主参考 Godot `AudioStreamPlayer2D` 三旋钮（刻意差异已记录：距离用欧氏长度，让 Y 深度也参与压远声；衰减 `pow(1-t,att)`；Range=0 只声像不衰减）。音乐链主参考 Godot 音乐总线（15.2）：LIFO 曲目栈 + 线性交叉淡化、总线音量+静音、混音器保最响声部并封顶 1、Ducker 在巨响期间压音乐再线性爬回。空曲目 id 永不播放（InvalidArg 保旧栈）；淡入淡出中再次切换先把旧淡进定到目标再重来，增益永不跳 NaN。错误全走 core 错码，nil 接收器安静（player 报静音、bus 报 0、mixer 报零、ducker 报 1）。

#### 常量（15）

| 符号 | 说明 |
|------|------|
| `DefaultBusVolume` | 总线默认音量 1.0（0dB 平线） |
| `DefaultMaxVoices` | 混音器默认同时声部上限（32） |
| `MaxMixerVoices` | 声部上限的硬顶（256），约束 NewMixer/SetMaxVoices |
| `DefaultDuckDepth` | 满力打击把音乐压低的默认深度（0.5） |
| `DefaultFade` | 标准切歌时长（1 秒） |
| `MaxMusicStack` | 音乐栈层数上限（8），长跑不涨 |
| `DefaultAttenuation` | 默认线性衰减（增益随距离直落） |
| `DefaultPanStrength` | 默认声像铺开（射程边缘满偏） |
| `MaxVoiceRequests` | 单帧声部请求数上限（128），超出只排名前 128、其余静音停放 |
| `MaxVoiceChannels` | 同时占道声部上限（64） |
| `AudibleVoices` | 可发声部数（=DefaultMaxVoices），取占道者中最响的这些 |
| `MinBPM` / `MaxBPM` | 可用节拍速度下限 30 / 上限 300 |
| `MinBeatsPerBar` / `MaxBeatsPerBar` | 拍号每小节拍数下限 1 / 上限 12 |

#### 类型（12）

| 符号 | 说明 |
|------|------|
| `Listener2D` | 2D 听点（Godot AudioListener2D 同位）：持位置；`MakeCurrent` 把它发布为全局唯一当前听点（拷贝共享 id），位置混音全部读当前听点 |
| `PosSound` | 一个 2D 声源：在哪响 `Pos`、传多远 `Range`、多快压低 `Attenuation`（0 范围内不衰减）、铺多宽 `PanStrength`；自己不出声，`Mix` 算出后端要的数 |
| `Mix` | 单声源可听结果：增益 [0,1]、声像 [-1,1]（-1 全左 +1 全右）、欧氏距离、是否该出样本 `Audible` |
| `MusicPlayer` | LIFO 曲目栈 + 线性交叉淡化：栈顶即目标；`Blend` 报淡出源与淡入目标的可听对；只用 AssetID 与 Duration，不出声 |
| `Bus` | 一条混音总线：音量 + 静音开关 |
| `MixResult` | 单帧封顶声部求和：总和、是否削顶、可发声部数、保留声部数 |
| `Mixer` | 声部上限器：只保留最响的 MaxVoices 个并封顶 1 |
| `Ducker` | 音乐闪避器：hold 期间全额压低，release 期线性爬回 |
| `VoiceRequest` | 一个声部请求：声源 + 基础音量 [0,1]（距离压低前）+ 是否爆点（该 duck 音乐）；不带名字，调用方按下标映射回自己的资产 |
| `VoicePick` | 单请求裁决：响度排名、喂后端的 Gain/Pan、本帧是否出样本 |
| `VoiceStats` | 一次挑选汇总：请求数、占道数、可听数、封顶总和、是否削顶、是否有可听爆点 |
| `BeatClock` | S89 节拍尺：只数拍（整数毫秒长歌不漂），拍号到事件名/音效/时间轴寻址的映射归调用方 |

#### 函数（17）

| 符号 | 说明 |
|------|------|
| `NewListener2D` | 在 pos 建听点；非有限位置 InvalidArg |
| `CurrentListener` | 返回当前发布听点拷贝+true；无人发布返回零值+false |
| `MixCurrent` | 在当前听点按序混全部声源；无人当前返回 nil,false；坏槽位静音不拖累其余 |
| `MixAll` | 在指定听点按序混全部声源：不改输入、返回新切片；坏听点 nil,false；坏声源只占静音槽不哑全场；空输入返回空切片 ok=true |
| `Stereo` | 线性平衡把单声道样本摊到左右（l=mono·gain·min(1,1−pan)）；居中两侧全响、全右静左；构造不出/静音/非有限输入一律 0,0（fail closed，NaN 不进后端） |
| `NewPosSound` | 建声源：pos 有限、Range/Attenuation/PanStrength 有限且 ≥0；坏参 InvalidArg 不落库 |
| `NewMusicPlayer` | 建空（静音）播放器 |
| `NewBus` | 建总线：名字非空、音量有限且 [0,1]，否则 InvalidArg |
| `NewMixer` | 建声部上限器，范围 [1,MaxMixerVoices]，越界 InvalidArg |
| `NewDucker` | 建闪避器：depth 有限 [0,1]、hold/release 非负，否则 InvalidArg |
| `CombineGains` | 单声部最终增益 = 音乐×总线×闪避再封顶 1；非有限输入按 0 处理（坏时钟不炸后端）；调用方对 Blend 的 from/to 增益各过一遍同一总线/闪避 |
| `NewVoiceRequest` | 建请求：sound 须来自 NewPosSound、level 有限 [0,1]；坏输入 InvalidArg |
| `SelectVoices` | 在听点处给全部请求排名并保最响：响度=混音增益×请求音量，射程外排 0 永不上道；最响 MaxVoiceChannels 占道、其中最响 AudibleVoices 发声；不改输入、结果每次新算；坏听点 ok=false；超 MaxVoiceRequests 仍可选（只排名前 128，其余静音） |
| `SelectAtListener` | 同上但在当前发布听点处；无人当前 ok=false |
| `FollowAndSelect` | 先把听点粘到镜头中心再挑选（听点随行入口）：调用方每帧传镜头有效中心，不 import 相机类型；nil 听点或坏位置 ok=false 且不动任何数 |
| `ApplyExplosionDuck` | 可听爆点出现时压音乐：仅当 stats 报有可听爆炸才 Trigger，否则 no-op 返回 false；nil ducker InvalidArg 不动；strength 须有限 [0,1] |
| `NewBeatClock` | 建节拍尺：BPM 须在 [MinBPM,MaxBPM]、beatsPerBar 须在 [MinBeatsPerBar,MaxBeatsPerBar]，坏输入 InvalidArg |

#### 方法（49，按接收者分组）

| 符号 | 说明 |
|------|------|
| `Listener2D.Pos` / `Listener2D.SetPos` | 读听点 / 移听点（非有限 InvalidArg 不动；当前听点随移，下次混音即听新位） |
| `Listener2D.FollowCamera` | 把听点粘到镜头中心：调用方每帧传镜头有效中心，不 import 相机类型 |
| `Listener2D.MakeCurrent` | 发布为全局唯一听点，旧听点静默退位；nil no-op；零值听点首次发布领 id 使拷贝共享 |
| `Listener2D.ClearCurrent` | 是当前听点才撤发布，其他听点不动；nil no-op |
| `Listener2D.IsCurrent` | 是否为当前发布听点 |
| `Listener2D.MixSource` | 在本听点位置混一个声源 |
| `Listener2D.MixStereo` | 在本听点混一个声源并把单声道样本摊到左右；ok 表示混合有效性而非可听性（远但有效仍 ok=true、样本 0,0） |
| `MusicPlayer.Depth` / `MusicPlayer.Top` / `MusicPlayer.From` | 栈深（nil 0）/ 目标曲目（静音 ""）/ 淡出源（稳定 ""） |
| `MusicPlayer.IsFading` / `MusicPlayer.Progress` | 切换是否在途 / 进度 elapsed/total ∈[0,1]（稳定报 1） |
| `MusicPlayer.Blend` | 可听对：(from,fromGain,to,toGain)，增益线性 1−t/t；稳定报 ("",0,top,1)、静音报 ("",0,"",0)；空 id 静音，后端忽略其增益 |
| `MusicPlayer.Play` | 重置栈为单曲目不淡：空 id InvalidArg |
| `MusicPlayer.Push` | 交叉淡入新顶层（如进入战斗叠层）：超 MaxMusicStack OutOfMemory、空 id/负时长 InvalidArg |
| `MusicPlayer.Pop` | 交叉淡回下一层：空栈 NotFound、负时长 InvalidArg、零时长瞬时切 |
| `MusicPlayer.CrossfadeTo` | 原深度换栈顶曲目：空 id InvalidArg（要静音用 Stop）、同 id 重定向 no-op |
| `MusicPlayer.Stop` | 淡到静音并清栈：负时长 InvalidArg、已静音 no-op |
| `MusicPlayer.Update` | 推进淡化时钟：dt≤0、已稳定、nil 均不动 |
| `Bus.Name` / `Bus.Volume` / `Bus.SetVolume` | 总线名（nil ""）/ 音量（nil 0）/ 调音（坏值 InvalidArg 保旧） |
| `Bus.SetMute` / `Bus.Muted` / `Bus.Gain` | 切静音（nil InvalidArg）/ 读静音 / 静音时 0 否则音量 |
| `Mixer.MaxVoices` / `Mixer.SetMaxVoices` | 读上限（nil 0）/ 调上限（越界 InvalidArg） |
| `Mixer.Mix` | 求和最响保留声部并封顶 1：非有限与非正条目静音；不改输入、结果恒有限 [0,1]；nil mixer 返回零结果 |
| `Ducker.Depth` / `Ducker.Active` | 压低深度（nil 0）/ 闪避计时是否在跑 |
| `Ducker.Trigger` | 启动或延长一次闪避：strength 须有限 [0,1]；重触发取更响者 |
| `Ducker.Update` | 闪避计时递减：dt≤0、nil 不动 |
| `Ducker.Gain` | 音乐乘数 [0,1]：静止 1、hold 期 1−depth·strength、release 期线性爬回；nil/空闲报 1 |
| `PosSound.Mix` | 算 s 在听点处的增益与声像：有效输入恒 ok=true（射程外静音也 ok=true：增益 0、Audible false、声像仍给方向）；坏输入 ok=false 静音，恒不 NaN 不 panic；不改输入 |
| `BeatClock.BPM` / `BeatClock.BeatsPerBar` / `BeatClock.Pos` / `BeatClock.Beat` | 速度（nil 0）/ 拍号 / 曲内位置 / 已越拍数 |
| `BeatClock.Bar` / `BeatClock.BeatInBar` | 当前小节（拍÷拍号）/ 小节内第几拍 |
| `BeatClock.Interval` / `BeatClock.Phase` | 一拍毫秒数 / 拍内相位 [0,1) |
| `BeatClock.SetBPM` | 改速度：坏值 InvalidArg 保旧；拍数按当前位置重统计，下次 Update 不跳变 |
| `BeatClock.SetBeatsPerBar` | 改拍号：坏值 InvalidArg |
| `BeatClock.Reset` / `BeatClock.Seek` | 归零（nil no-op）/ 移动曲内位置（负目标 InvalidArg 不动；拍数随新位置） |
| `BeatClock.Update` | 推进曲钟并返回新越过的拍号：dt≤0 与 nil 返回 nil 不动；返回新切片，触发什么提示归调用方 |
| `BeatClock.TimeOfBeat` | 第 n 拍的曲内位置：负 n InvalidArg 返回 0；nil 返回 0 |

## 4. `engine/camera` — 镜头与投影（80 条：顶层 37 + 方法 43）

> 冻结镜头数学（2026-09-14 冻结口子，只做加法）。四块能力：**Camera**（纯数学 2D 镜头：位置/缩放/角度/抖动/边界/跟随平滑/屏幕锚点/视口，`View` 产出世界→屏幕矩阵喂既有绘制）、**Layer**（视差层：随镜头漂移比例 + 无缝平铺镜像 + 深度投影）、**Projector**（1.1 针孔透视：世界位置+深度→屏幕位置+缩放，`ProjectQuad` 角点级失败整块不画）、**Basis25D**（S79 Godot misc/2.5d 演示同款平行压扁：三轴基投影，六视图逐数抄演示值，与针孔互不相干）+ **真 2.5D 三件套**（影子落地 / Y 排序盖序）。全部只算数不画画面，nil/坏输入恒 ok=false 零值不 panic。

#### 常量（12）

| 符号 | 说明 |
|------|------|
| `BasisScale` | 演示的 SCALE：1 个 3D 单位 = 32 个 2D 单位 |
| `View45` / `ViewIsometric` / `ViewTopDown` / `ViewFrontSide` / `ViewObliqueY` / `ViewObliqueZ` | 六种 2.5D 视图模式（45°/等距/俯视/正视/斜 Y/斜 Z），按演示顺序冻结 |
| `MinZoom` | 缩放下限 1e-6：零/负缩放钳到此值，View 永远可逆 |
| `MaxYSort25D` | 单次排序上限 4000（照抄演示 y_sort_25d 的 4000 节点护栏） |
| `YSortZBase` / `YSortZStep` | Y 排序映射 z 槽：首槽 −4000、步长 2（奇数槽按约定留给影子） |
| `YSortZRange` | 有效 z 索引钳位半宽 4096（Godot 在 z 链每一层都 CLAMP，深链跑不飞） |

#### 类型（8）

| 符号 | 说明 |
|------|------|
| `ViewMode` | 六视图枚举，见常量行 |
| `Basis25D` | 一个视图模式的三条 2D 轴（X/Y/Z）：3D 点 (x,y,z) 落在 x·X+y·Y+z·Z，与演示 `flat_pos` 逐位同式；深度只影响绘制顺序与影子落地 |
| `Camera` | 纯数学 2D 镜头：位置、缩放、角度、抖动偏移、位置钳制框、跟随平滑、屏幕锚点、视口尺寸；不画任何东西，`View` 喂既有绘制 |
| `Layer` | 一条视差层：随镜头漂移比例（1 锁世界、0 钉屏幕、0.5 半速、>1 超前）+ 额外偏移 + 无缝平铺镜像；不画东西，`Screen`/`ScreenQuad` 结果喂 Projector 与既有的梯形/三角形绘制 |
| `Projector` | 针孔投影器：焦距 + 落点中心 + 相机点；render 类型只在边界经 `engine/renderconv` 换一次 |
| `YSortKeyFn` | 桶内排序键函数：把一个位置换成键值；nil 表示 `YSortKeyY` |
| `YSortItem` | 平铺集合里一个可排序对象：容器系坐标 `Pos`、破平局用容器序 `Order`、z 桶 `ZIndex`（平铺项视为已解析，`ZAsRelative` 无意义） |
| `YSortNode` | 树形容器里一个节点：`Parent` 是同切片下标（根为 −1），Pos 须是容器系累计变换原点（Godot 按累计变换原点排序）；ZAsRelative 时 z 沿父链逐级累加并逐级钳位 |

#### 函数（17）

| 符号 | 说明 |
|------|------|
| `NewBasis25D` | 建某视图的三轴基（演示 set_view_mode 值 × BasisScale 展开写死，六组矩阵与演示并排可读） |
| `NewCamera` | 在视口（屏幕尺寸）上建居中镜头：视口须有限、负分量钳 0、NaN/Inf 报 InvalidArg |
| `NewLayer` | 建视差层：factor 任意有限值（含负的反向漂移），mirror 分量非负；负/非有限 InvalidArg，绝不静默卷绕 |
| `NewProjector` | 建投影器：focal 须有限且正、center/camera 须有限；否则 InvalidArg，绝不猜投影器 |
| `LandShadow` | 影子落地：目标竖直落到 groundY 再经基压扁；地下目标与 NaN/Inf 报 ok=false（藏影子），恒不 NaN |
| `ShadowVisible` | 落地成功的影子是否该画：落地 ok 且目标在地面上方 |
| `YSortKeyY` | 标准桶内键：节点原点 Y（演示自带 y_sort 与 Godot ItemYSort 同口径） |
| `YSortKeySlightXZ` | 演示的 y_sort_slight_xz 变体：Y 加一丝 X+Z，让同高物体按几何排而不落回容器序；经 YSortWithKey/YSortTreeWithKey 选配 |
| `ClampZIndex` | 单个 z 索引饱和进 [−YSortZRange, YSortZRange]（Godot z 链每层的 CLAMP）；把自有优先轴映到 z 轴的模块库统一走这里，饱和规则只此一处 |
| `YSortLessWithKey` | a 是否画在 b 后面：先 z 桶（Godot 同 z_index 一个桶、桶内按 Y），再容差判等的键，再容器序；Order 不同必分胜负，结果不依赖排序算法稳定性；判等分支不可传递（Godot 同款性质，可跨的差远小于一像素） |
| `YSortLess` | 标准比较器：z 桶 → 纯 Y → 序 |
| `YSortEffectiveZ` | 从父链解析单节点 z 桶（相对 z 逐级累加并逐级钳位；绝对 z 无视链）；坏下标/非有限位置/成环报 core 错返回 0 |
| `YSortTreeWithKey` | 树形容器深度优先展平（根与子按切片序；宿主自身可作根占 Godot 0 号位）并返回绘制序（下标：远先近后）；展平是 DFS 但比较不聚簇——键是各节点自身容器系位置（同 Godot ysort_xform.origin.y），远 Y 的后代照样排后与其他分支交错，DFS 序只破平局；超 MaxYSort25D/坏父/成环/非有限位置报错返回 nil |
| `YSortTree` | YSortTreeWithKey 的标准纯 Y 版 |
| `YSortWithKey` | 平铺条目按远近排序（不改输入）：同辈集合，ZIndex 按给定值用（无父链可累加）；超上限报 OutOfMemory 返回 nil（照抄演示拒排 4000+） |
| `YSort` | YSortWithKey 的标准纯 Y 版 |
| `YSortZ` | 排序名次 → z 槽（base+rank×step；奇数槽按约定留影子，引擎尚无消费者）；这是演示的 z_index 赋值技巧（重写节点 z 让引擎自带的 z 分桶复现 Y 序），不是上面的比较器 |

#### 方法（43，按接收者分组）

| 符号 | 说明 |
|------|------|
| `Basis25D.Mode` | 本基所建视图模式 |
| `Basis25D.Project` | 一个 3D 点经三轴压扁；NaN/Inf 输入 ok=false 零点，恒不 NaN |
| `Basis25D.ProjectPoints` | 按序压扁多点：坏点整批失败（ok=false），调用方绝不画半形状（与 Projector.ProjectQuad 同契约） |
| `Camera.Pos` / `Camera.Zoom` / `Camera.Rotation` / `Camera.Shake` | 读钳制后中心（世界单位）/ 缩放（每分量 ≥MinZoom）/ 角度（弧度归一到 [−π,π]）/ 抖动偏移 |
| `Camera.Limit` / `Camera.Smoothing` / `Camera.Anchor` / `Camera.Viewport` | 读中心钳制框（空=禁用）/ 跟随平滑因子（0 立即、(0,1] 缓动）/ 屏幕锚点比例 [0,1]² / 屏幕像素尺寸 |
| `Camera.SetViewport` / `Camera.SetPos` / `Camera.SetZoom` / `Camera.SetRotation` | 换屏幕尺寸（负钳 0）/ 移中心（设了 Limit 就钳进去）/ 换缩放（零负钳 MinZoom）/ 换角度（有限值归一到 [−π,π]） |
| `Camera.SetShake` / `Camera.DecayShake` | 换抖动偏移（驱动归调用方：创伤曲线/种子随机，镜头只叠加与衰减）/ 按 [0,1] 系数缩抖动（1 清零；越界钳、NaN/Inf 报错） |
| `Camera.SetLimit` / `Camera.SetSmoothing` / `Camera.SetAnchor` | 换钳制框（空=禁用；NaN/Inf 报错、负尺寸保持空）/ 设平滑（越界钳 [0,1]）/ 设锚点（视口比例，(0.5,0.5)=居中；越界钳） |
| `Camera.Follow` | 追目标：平滑 0 立即到达，否则按因子插值；坏目标报错不动 |
| `Camera.FollowDeadzone` | 死区跟随：半径内按住不动、半径外按 Follow 追（Limit 与平滑照常生效）；负半径钳 0（恒追）、NaN/Inf InvalidArg 不动；返回镜头是否移动 |
| `Camera.DualPicture` / `Camera.DualTarget` | 画面所在=钳后中心 / 一次取「调用方目标（不钳）+镜头画面（钳后）」两个数——目标与画面天生两个查询，永不塌成一个数 |
| `Camera.EffectivePos` | 钳后中心 + 抖动偏移；Pos 在写入时已钳（热路径不再二次钳），抖动允许出界一帧是设计内 |
| `Camera.View` | 世界→屏幕矩阵：`screen = Anchor·Viewport + Rot(−rotation)·Zoom·(world − EffectivePos)`；Zoom 永不为零（见 MinZoom），有限状态下恒有限可逆 |
| `Camera.WorldToScreen` / `Camera.ScreenToWorld` | 世界点→屏幕像素 / 逆变换；坏输入 ok=false 零值 |
| `Camera.VisibleWorldRect` | 视口的轴对齐世界包围盒（瓦片/区块裁剪用）；有旋转时是保守界；空视口只看见中心点 |
| `Layer.Factor` / `Layer.Offset` / `Layer.Mirror` | 读漂移比例 / 额外偏移 / 平铺镜像 |
| `Layer.Shift` | 漂移后的世界位：`shift = world + camera·(1−factor) + offset`，mirror 分量设置时按轴卷绕进 [0,mirror)；factor=1 抵消镜头项（锁世界）、factor=0 保留镜头项待 Screen 减回（钉屏幕）；不可画输入 ok=false 零值 |
| `Layer.Screen` | 先 Shift 再 Project：每帧用同一镜头建 Projector，factor=1 层与直接 Project 落点精确一致；同时返回透视缩放供精灵定尺寸；任一步不可画 ok=false |
| `Layer.ScreenQuad` | 四角 + 逐角深度 → 四屏幕角（喂 render.DrawImageQuad）；任一角不可画整块失败，不画半个梯形 |
| `Projector.Focal` / `Projector.Center` / `Projector.Camera` | 建器时的眼到屏距 / 深度 0 落的屏幕点 / 深度 0 时 Center 下方的世界点 |
| `Projector.DepthToScale` | 深度→屏幕缩放：深度 0 返回 1；眼里侧（focal+depth≤0）或非有限深度 ok=false 缩放 0 |
| `Projector.Project` | 世界点+深度 → 屏幕点，同时返回所用缩放供定尺寸；不可画输入 ok=false 零屏幕零缩放，恒不 NaN 不 panic |
| `Projector.ProjectQuad` | 四角 + 逐角深度 → 四屏幕角（经 renderconv 各换一次后逐角喂 render.DrawImageQuad）；任一角不可画整块失败 |
| `Projector.ProjectSpriteQuad` | 世界矩形按 TL/TR/BR/BL 逐角投影成 render 透视腿要的角点 + 齐次权重（权重=focal+depth；四角同深度=等权，等价按深度缩放的仿射矩形）；消费者为 render 的 AtlasSprite 透视字段或 DrawImageQuad；任一角 behind-eye 或非有限整体 ok=false |
| `Projector.Unproject` | Project 的逆（屏幕+深度→世界）；拒绝域与 Project 相同，往返在浮点容差内闭合 |

## 5. `engine/core` — 数与错的底座（133 条：顶层 60 + 方法 73）

> 冻结全部 `engine/*` 包共用的值类型（2026-09-14 冻结口子，只做加法）。铁律：engine 下任何包**复用这些类型、不许另定义** Vec2/Color/AssetID；render 类型不碰，只在边界经 `engine/renderconv` 换一次；破坏性变更先升数据版本号再搬数据。要点：时间一律整数毫秒（百万步累积仍落在精确毫秒）；随机数 splitmix64 种子确定可回放（不用于安全）；错误统一 `Code` 分类 + `Error` 载体（什么错/哪个操作/哪个资产/包裹原因），全链 `CodeOf` 可归类。

#### 常量（11）

| 符号 | 说明 |
|------|------|
| `CodeUnknown` / `CodeNotFound` / `CodeBadData` / `CodeOutOfMemory` / `CodeUnsupported` / `CodeInvalidArg` / `CodeVersionMismatch` | 游戏侧失败分类：未知 / 缺文件缺资产 / 数据损坏或撕包 / 内存或显存预算耗尽 / 预留但未建（未冻结子集、P5 float 管线等）/ 空零非法参数 / major 版本载不进 |
| `Millisecond` / `Second` / `Minute` / `Hour` | `Duration` 单位常量：1 / 1000 / 60000 / 3600000 毫秒 |

#### 变量（10）

| 符号 | 说明 |
|------|------|
| `Black` / `White` / `Red` / `Green` / `Blue` / `Yellow` / `Cyan` / `Magenta` | 常用命名色（不透明） |
| `Transparent` | 全透明色 |
| `SchemaVersion` | core 契约版本（1.0）：后续一切数据格式（图集 JSON、场景 JSON、存盘 JSON、Spine/TMX 子集）都对着它校验；破坏性升 Major、追加升 Minor |

#### 类型（14）

| 符号 | 说明 |
|------|------|
| `AssetID` | 可装载资产名（纹理/地图/骨骼/存盘…）：小写路径风格，如 `tex/hero`、`map/level1`、`bone/slime` |
| `Handle` | 资产的引用计数句柄：`Ref`/`Release` 必须成对，越过零报错不静默；Handle.refs 记本句柄次数，Manager.counts 汇总同 id 全部句柄，两侧同步更新 |
| `Manager` | id→计数的共享账本：纹理/地图/存盘共用一本，不各自发明记账 |
| `Version` | major.minor 数据版本（存盘/地图/骨骼/图集都带）；Major 升级断装载、Minor 升级兼容 |
| `Color` | 直色 alpha 颜色，float64 分量 [0,1]；字段与 render RGBA 一一对应但独立成类——游戏代码（染色/灯/LUT）永不依赖 render 内部，边界处一次性转换 |
| `Rand` | 种子确定性生成器（splitmix64）：同种子回放同序列，粒子与玩法测试不靠运气；不用于安全 |
| `Code` | 失败分类枚举，见常量行；`String()` 给稳定日志名 |
| `Error` | 统一错误载体：种类 `Code`、操作 `Op`、资产 `ID`、包裹原因 `Err` |
| `Duration` | 游戏时间：整数毫秒存储（累积不漂），`Seconds()` 给 render/物理边界要的浮点秒 |
| `Step` | 一个固定逻辑步：序号 `Index` + 步宽 `Dt`；固定循环本体在 engine/step，core 只保「第几步」的无漂计数 |
| `Vec2` | 2D 向量（游戏单位）：字段镜像 render 点类型但独立成类，边界一次性转换 |
| `Rect` | 轴对齐框：原点 (X,Y) + 尺寸 (W,H) |
| `Mat2D` | 2D 仿射矩阵（行主序 2×3：x'=Ax+By+C，y'=Dx+Ey+F）；与 render 矩阵同布局独立成类；透视不进这里 |
| `Vec3` | 3D 点（真 2.5D 数学专用，S79）；风格同 Vec2，边界一次性转换 |

#### 函数（25）

| 符号 | 说明 |
|------|------|
| `NewManager` | 建空账本 |
| `ParseVersion` | 解析 "major.minor"（minor 可省）；其余一律 BadData，绝不猜版本 |
| `RGB` / `RGBA` | 建不透明色 / 建直 alpha 色 |
| `ColorFromBytes` | 从 8 位直 alpha 通道建色 |
| `ParseHex` | 解析 `RGB`/`RGBA`/`RRGGBB`/`RRGGBBAA`（`#` 可省）；其余 BadData，绝不静默给色 |
| `NewRand` | 从种子建生成器 |
| `New` | 无原因的 Code 错误 |
| `NotFound` / `BadData` / `OutOfMemory` / `Unsupported` / `InvalidArg` / `VersionMismatch` | 六类便捷构造：缺文件资产 / 数据坏 / 预算耗尽 / 预留未建 / 非法参数 / 版本载不进（均可带 cause） |
| `CodeOf` | 给 err 归类：`*Error` 的 Code（包裹多层也算），nil 与外来错误报 CodeUnknown |
| `Milliseconds` / `SecondsFloat` | 按毫秒数建 Duration / 按浮点秒建 Duration（四舍五入到毫秒） |
| `FirstStep` | 用给定步宽开跑第一步 |
| `V2` / `V3` | 建 Vec2 / 建 Vec3 |
| `NewRect` | 建 Rect |
| `Identity2D` / `Translate2D` / `Scale2D` / `Rotate2D` | 单位矩阵 / 平移矩阵 / 缩放矩阵 / 旋转矩阵（弧度逆时针） |

#### 方法（73，按接收者分组）

| 符号 | 说明 |
|------|------|
| `AssetID.Empty` / `AssetID.String` / `AssetID.Validate` | 是否空名 / 原始 id 文本 / 空名报 InvalidArg |
| `Handle.ID` / `Handle.Refs` | 句柄声明的资产 / 该资产活引用数 |
| `Handle.Ref` / `Handle.Release` | 加一个引用（死/nil 句柄返回 false）/ 掉一个引用（nil、死、零引用句柄报错不转负；与 Ref 成对） |
| `Manager.Acquire` | 认领 id（计数 1）并返回活句柄；空 id InvalidArg |
| `Manager.Loaded` / `Manager.LiveCount` | id 是否有活引用 / 活引用数 |
| `Version.String` / `Version.CompatibleWith` / `Version.CheckCompatibility` | 渲染 "major.minor" / 数据 v 能否在引擎 cur 下装载（同 major 且数据 minor 不新于引擎）/ 不能装载报 VersionMismatch |
| `Color.Clamped` / `Color.ToBytes` | 各分量钳到 [0,1] / 8 位直 alpha 通道（钳位四舍五入；render 路径是截断，边缘值差 1 属设计内，精确值走 renderconv 浮点路） |
| `Color.Lerp` / `Color.Premultiplied` / `Color.Unpremultiplied` | 插值（t=0 得 c、t=1 得 o）/ RGB 乘 A（预乘）/ 逆预乘（零 alpha 保持零不出 NaN） |
| `Color.ApproxEqual` | 四通道差均小于 eps |
| `Rand.Seed` / `Rand.Uint64` / `Rand.Int63` / `Rand.Int63n` | 重置序列 / 下一个 64 位值 / 非负 63 位值 / [0,n)（n≤0 返 0 不 panic；取模偏有意接受——序列冻结为回放，非安全非统计用） |
| `Rand.Float64` / `Rand.RangeFloat` / `Rand.RangeInt` | [0,1) / [min,max]（写反自动换）/ [min,max)（max≤min 返 min） |
| `Rand.Shuffle` | Fisher-Yates 洗牌 0..n−1；n≤0 或 nil swap no-op |
| `Code.String` | 稳定日志名（not-found、bad-data…） |
| `Error.Error` / `Error.Unwrap` | 渲染 `op "id": code: cause`（空段跳过）/ 取包裹原因 |
| `Duration.Milliseconds` / `Duration.Seconds` | 原始毫秒数 / 浮点秒（render/物理边界用） |
| `Duration.Add` / `Duration.Sub` / `Duration.Scale` / `Duration.Clamp` | 加 / 减（可为负，钳制归调用方）/ 乘系数（时间缩放慢动作，四舍五入到毫秒）/ 钳到 [min,max]（max<min 视作相等） |
| `Step.Next` / `Step.Total` | 序号推进（Dt 一经定下中途不变，长跑不像浮点累加那样漂）/ Index×Dt=本步前的累计逻辑时间 |
| `Vec2.Add` / `Vec2.Sub` / `Vec2.Mul` / `Vec2.Div` / `Vec2.Neg` | 加 / 减 / 数乘 / 数除（零 s 按 IEEE-754 不 panic）/ 取负 |
| `Vec2.Dot` / `Vec2.Cross` / `Vec2.Length` / `Vec2.LengthSq` | 点积 / 2D 叉积（标量）/ 模长 / 模长平方（比较更省） |
| `Vec2.Normalize` / `Vec2.Lerp` / `Vec2.Rotate` / `Vec2.Perp` | 单位化（零向量保持零不出 NaN）/ 插值（t=0 得 v、t=1 得 w）/ 逆时针旋转 rad 弧度 / 逆时针转 90° |
| `Vec2.Atan2` / `Vec2.Angle` | v 的方向角 / v 到 w 的带符号角 |
| `Vec2.IsZero` / `Vec2.ApproxEqual` | 是否精确零向量 / 逐分量差小于 eps |
| `Rect.IsEmpty` / `Rect.Area` / `Rect.Center` | 是否无面积 / W×H（空框 0）/ 中心点 |
| `Rect.Contains` / `Rect.ContainsRect` / `Rect.Intersects` | 点是否在内（min 含 max 不含，空框不含任何点）/ o 是否全在 r 内 / 是否正面积重叠 |
| `Rect.Intersection` / `Rect.Union` / `Rect.Offset` / `Rect.Inset` | 重叠框（无重叠 false）/ 容下两者的最小框（空侧忽略）/ 平移 / 收缩（正）或放大（负） |
| `Mat2D.Mul` / `Mat2D.TransformPoint` | 矩阵乘（n 先施加）/ 作用于点 |
| `Mat2D.Invert` | 逆矩阵（奇异矩阵返回 false）；奇异保持报错——render 矩阵的 Invert 回退单位阵，这里精确 det==0 保证跨机回放稳定 |
| `Mat2D.IsIdentity` / `Mat2D.ApproxEqual` | 是否精确单位阵 / 六分量差均小于 eps |
| `Vec3.Add` / `Vec3.Sub` / `Vec3.IsZero` | 加 / 减 / 是否精确零向量 |

## 6. `engine/debug` — 性能与崩溃数字（60 条：顶层 25 + 方法 35）

> 冻结游戏侧性能与崩溃数字（2026-09-16 冻结口子，只做加法）。**只带数字**：不画画面、不碰 render 主路径。调用方每个呈现帧记一帧 `Frame`（帧耗时/绘制调用/过绘制/显存采样/分段明细），`RecordShader` 累加着色器编译开销，`NoteMemory` 在帧间采显存；`Report` 冻结长跑报告与崩溃日志共享的快照（帧率/绘制/内存/明细/过绘制/着色器耗时），`Text` 渲染人读报告（形状冻结，golden 逐字节钉住）。文件为自述 JSON（version "1.0" + Report 字段），未知字段忽略使新 minor 仍可解析；外来 major/更新 minor 报 VersionMismatch，绝不猜装载。帧耗时钳在 (0, MaxFrameDt]：超长帧（后台间隙）钳到上限并计一次 Gap，恢复时 FPS 不塌；超过 SlowFrameDt 计一次慢帧；显存超预算计一次 OOM 且保旧值（fail closed 不崩）。nil/零值 Stats 读全零、写报 InvalidArg。

#### 常量（8）

| 符号 | 说明 |
|------|------|
| `MaxFrames` | 保留逐帧样本上限（3600，60fps 下 60 秒；最老淘汰） |
| `MaxSpansPerFrame` | 单帧明细分段上限（32） |
| `MaxShaders` | 单 Stats 内不同着色器名上限（256） |
| `MaxNameLen` | 分段名/着色器名/崩溃原因文本上限（64 字节） |
| `MaxBytes` | 单份报告或崩溃文件字节上限（1MB） |
| `MaxMemoryBytes` | 单次显存采样上限（512MB） |
| `MaxFrameDt` | 单帧耗时上限（250ms，随 step.MaxFrame）：更长的帧钳到上限并计一次 Gap |
| `SlowFrameDt` | 慢帧线（20ms）：超过计一次慢帧（验收口径：超 20ms 每分钟至多 3 次） |

#### 变量（1）

| 符号 | 说明 |
|------|------|
| `CurrentVersion` | 报告文件版本（1.0）：加字段升 Minor，破坏形状升 Major |

#### 类型（7）

| 符号 | 说明 |
|------|------|
| `Span` | 帧内一段明细：谁吃了多久；字段私有，写入全过校验 |
| `Frame` | 一个呈现帧：耗时、绘制调用、过绘制比（1.0=无过绘制）、显存采样、明细分段 |
| `SpanStat` | 聚合后的一行明细：总/均/最大耗时与占全部段时比例；导出字段直接序列化 |
| `ShaderStat` | 聚合后的一行着色器编译：次数/总耗时/最大耗时 |
| `Stats` | 采集器：保留帧 + 着色器与显存台账；零值可用，读不失败（nil 答零）、写报 InvalidArg |
| `Report` | 报告员与崩溃日志共享的冻结快照（版本/帧数/帧率/均/p95/慢帧/间隙/OOM/绘制/显存/过绘制/明细/着色器），导出字段直接对上冻结文件形状 |
| `CrashLog` | 一份崩溃/在跑快照：为何停跑 `Reason` + core 错码名 `Code` + 当时的冻结 `Report` |

#### 函数（9）

| 符号 | 说明 |
|------|------|
| `NewSpan` | 建一段明细：空名/超长名/负时长 InvalidArg 不落库 |
| `NewFrame` | 建一帧：非正 Dt、负绘制/显存、非有限或负过绘制、坏分段、超长分段表一律 InvalidArg 不落库；显存预算在 NoteMemory 时判，不在建帧时判 |
| `NewStats` | 建空采集器 |
| `ParseReport` | 校验报告文件字节：空 InvalidArg、超预算 OutOfMemory、坏 JSON/坏形状 BadData、外来或更新版本 VersionMismatch |
| `StoreReport` / `LoadReport` | 报告写盘（自动建父目录；空路径 InvalidArg、OS 失败 NotFound 带原因）/ 读盘（缺文件 NotFound，其余同 Parse） |
| `ParseCrash` / `StoreCrash` / `LoadCrash` | 崩溃文件的解析/写盘/读盘：错码档位与报告文件一致 |

#### 方法（35，按接收者分组）

| 符号 | 说明 |
|------|------|
| `Span.Name` / `Span.Took` | 分段键 / 分段耗时 |
| `Frame.Dt` / `Frame.Draws` / `Frame.Overdraw` / `Frame.MemBytes` / `Frame.Spans` | 读帧耗时 / 绘制调用 / 过绘制比 / 显存采样 / 明细分段拷贝（改结果动不了帧） |
| `Stats.Reset` | 清空全部台账，nil 不动 |
| `Stats.Count` / `Stats.TotalFrames` | 保留帧数 / 历史总帧数（含淘汰） |
| `Stats.Gaps` / `Stats.OOMs` | 钳制的后台间隙帧数 / 拒收的超预算显存采样数 |
| `Stats.RecordFrame` | 存一帧：坏帧 InvalidArg 不动；超 MaxFrameDt 的 Dt 钳制并计一次 Gap（台账存钳后值，后台间隙不塌 FPS） |
| `Stats.RecordShader` | 记一次着色器编译：空/超长名与负时长 InvalidArg、新名超 MaxShaders OutOfMemory；失败不动 |
| `Stats.NoteMemory` | 帧外采显存（加载屏/清库）：负值 InvalidArg；超 MaxMemoryBytes 报 OutOfMemory、计一次 OOM、保旧值 |
| `Stats.AvgFrame` / `Stats.FPS` / `Stats.P95Frame` | 保留帧均耗时 / 1000÷均毫秒 / 95 分位帧耗时（上取整排名），空台账 0 |
| `Stats.SlowFrames` / `Stats.TotalDraws` / `Stats.AvgDraws` | 超 SlowFrameDt 的保留帧数 / 保留帧绘制调用总和 / 均绘制调用 |
| `Stats.CurMemory` / `Stats.PeakMemory` / `Stats.AvgOverdraw` | 最新显存采样 / 历史峰值 / 均过绘制比 |
| `Stats.Breakdown` / `Stats.TopSpans` | 明细聚合（按总耗时降序、名破平局；share 除以全部段时；无段时空表不出 NaN）/ 前 n 行（n<0 或超给全部） |
| `Stats.Shaders` / `Stats.ShaderTotal` | 按名升序聚合编译行 / 全部编译总耗时 |
| `Stats.Report` | 快照各台账；nil 采集器报当前版本的零快照，不崩 |
| `Stats.Snapshot` | 带原因与 core 错码冻结台账（长跑与崩溃报告用）；nil 冻结零报告 |
| `Report.Equal` | 快照是否相同：浮点精确比较（两侧都源自整数台账，同输入同位） |
| `Report.Encode` | 渲染规范文件字节；超预算 OutOfMemory；不改输入 |
| `Report.Text` | 渲染人读报告（F 场景读）：头行（帧数/帧率/计数）+ 绘制内存行 + 按占比排序明细行 + 按名着色器行；形状冻结，golden 逐字节钉住 |

## 7. `engine/dirty` — 逐帧重绘框（18 条：顶层 6 + 方法 12）

> 动态脏区跟踪：一个重绘单元，全程只碰 `core.Rect` 数字（2026-09-15 冻结口子，只做加法）。动态跟踪器记「本帧哪些框要重画」；精灵跟踪器（twin-layer 双层规则）独立于静态世界更新——只有动过的精灵标脏，不动的地面保持零框。非并发安全；nil 跟踪器不 panic。

#### 常量（1）

| 符号 | 说明 |
|------|------|
| `MaxDirtyRects` | 触发整帧重绘前的脏框上限（16）；必须与 ui/scene 的 MaxDirtyRects 相等（超长回落整帧） |

#### 类型（2）

| 符号 | 说明 |
|------|------|
| `DirtyStats` | 单跟踪器帧账目：Clear 次数 `Frames`、非整帧保留框累计 `TotalRects`、回落整帧次数 `FullFallbacks`、单帧最大保留框数 `MaxRects` |
| `DirtyTracker` | 一个动态重绘单元：界内脏框列表 + 是否整帧标记；精灵型跟踪器独立更新，不并发安全，nil 不 panic |

#### 函数（3）

| 符号 | 说明 |
|------|------|
| `NewDirtyTracker` | 在 bounds 上建动态跟踪器：非有限坐标或 W/H≤0 报 invalid-arg |
| `NewSpriteDirtyTracker` | 建独立的精灵更新跟踪器；校验同上 |
| `DirtyForMove` | 纯追逐车并集：盖住新旧两个精灵框（拖尾与到达都重画）；一侧空=出生/回收（取非空侧）；同框=没动（false）；非有限/全空输入 false；界内裁剪交给 MarkDirty/MarkMoved |

#### 方法（12，均在 `DirtyTracker` 上）

| 符号 | 说明 |
|------|------|
| `DirtyTracker.Bounds` / `DirtyTracker.IsSprite` | 跟踪界 / 是否精灵型独立跟踪器 |
| `DirtyTracker.NeedsFull` / `DirtyTracker.MarkFull` | 本帧是否必须全量重绘 / 强制整帧（丢框列表、NeedsFull 置真） |
| `DirtyTracker.DirtyCount` / `DirtyTracker.DirtyRects` / `DirtyTracker.DirtyArea` | 保留框数（整帧或 nil 为 0）/ 框拷贝（整帧或空为 nil；调用方可改，跟踪器不留别名）/ 框面积和（整帧 0） |
| `DirtyTracker.Coverage` | 脏面积占界面积比例（仅观测，不主动触发回落） |
| `DirtyTracker.Stats` | 帧账目拷贝 |
| `DirtyTracker.Clear` | 收帧：先记账再清框与整帧标记；每次 Clear 记一帧（零框的静止帧也算） |
| `DirtyTracker.MarkDirty` | 把 r 裁进跟踪界并保留：空/非有限/全在界外的框静默忽略；返回是否保留一框；nil 或已整帧的跟踪器不保留 |
| `DirtyTracker.MarkMoved` | 精灵旧框+新框的并集标脏（见 DirtyForMove），裁进跟踪界；返回是否保留一框 |

## 8. `engine/fx` — 全屏特效数学（84 条：顶层 50 + 方法 34）

> 冻结 CPU 侧全屏特效数学（2026-09-15 冻结口子，只做加法）。热浪/水波把画面在噪声偏移位置采样而非原像素：**Warp 只算偏移**，调用方把结果应用于新的中间拷贝、绝不动活帧。数学冻结（CPU/GPU 双腿同式）：整数格上的 2D value noise（splitmix64 哈希 + smoothstep），`Offset(p,t)=strength·noise(p·freq+drift·t)` 每轴独立通道盐；纯函数，同输入跨机逐位回放，CPU/GPU 天然一致。热路径 `OffsetAt` 是全函数：strength 0（含零值 Warp）精确恒等、任何非有限输入得零偏移，绝不 panic 不漏 NaN。另含：艺术家挂钩 Custom（identity/outline/dissolve 三种冻结材质 + Registry 按句柄挂调，降级位明确标注 CPU 参考路径）、调色链 Grade（泛光→晕影→LUT→色调映射，S66 默认 Reinhard）、LUT、每通道 1D 表重映射、独立中间图 Image（永不别名）。唯一 render 侧 import 是 bloom.go 的 `render/hdr`：P3.7 批 5 起泛光核在 render 层 float 域、fx 只喂参数（engine→render 是允许方向）。

#### 常量（22）

| 符号 | 说明 |
|------|------|
| `MaxImagePixels` | 单张调色中间图上限（1600 万像素，RGBA8 下 64MB）；更大报 OutOfMemory 不分配 |
| `MaxBloomSigma` | 泛光 σ 上限（=render.MaxBloomSigma，σ=16 已是每向 49 tap；引擎委托领域不自造上限） |
| `MaxBloomIntensity` | 泛光叠加强度上限 4（比 render 层上限更紧的游戏侧红线） |
| `MaxCustomNameLen` / `MaxCustomParams` / `MaxCustomOutlineWidth` / `MaxCustomSeed` / `MaxCustomSlots` | 材质名 64 字节 / 参数表 8 项 / 描边宽度 8px / 溶解种子 uint32 上限 / 活跃挂钩句柄 256 |
| `CustomIdentity` / `CustomOutline` / `CustomDissolve` | 三种冻结材质名；未知名一律 Unsupported，绝不猜 |
| `MinLUTSize` / `MaxLUTSize` | 1D 每通道表长下限 2（更短弯不了色）/ 上限 256（更长属于 3D cube，不归本文件） |
| `MaxExposure` | 色调映射曝光上限 16（已是昼转夜范围） |
| `TonemapNone` / `TonemapReinhard` / `TonemapACES` / `TonemapFilmic` | 高光滚降曲线：关 / x/(1+x)（Godot Reinhard 同念）/ Narkowicz 电影近似（Skia 无内置曲线，游戏自带公式）/ Hejl-Burgess-Dawson 肩曲线（S66 第三选项，可选非默认） |
| `DefaultTonemap` | S66 冻结曲线选择：Reinhard（代码可选，窗与 golden 钉死此值） |
| `MaxVignetteRadius` | 晕影 Inner/Outer 上限 2（UV 单位，足够从框内任意中心盖住任意角） |
| `MaxWarpDimension` / `MaxWarpPixels` | 扭曲图单边 8192px / 面积 8192×8192 上限 |

#### 类型（10）

| 符号 | 说明 |
|------|------|
| `Image` | 一张隔离的调色中间图：W×H 直 alpha `core.Color` 行主序；构造拷贝、Apply 出新图、Clone 再拷贝，永不别名 |
| `Bloom` | 泛光：超阈像素软膝遮罩 → render 层可分离高斯模糊（共享核，与 GPU 腿同式）→ 按 Intensity 加回；σ=0 只亮罩不糊；Intensity=0 恒等（可冻结一套 Grade 用 Intensity 调光晕） |
| `Custom` | 一个艺术家材质挂钩：名字 + float 参数；只存游戏层数字，Apply 结果喂既有绘制 |
| `Registry` | 按句柄管理的挂钩表：Attach 拷入、Detach 删除、Apply 按句柄调用；零值不可用，须走 NewRegistry |
| `LUT` | 每通道 1D 表线性插值重映射：R/G/B 各存 Size 个 0..1 输入样本（0=黑、Size−1=白）；入时一律拷贝、永不与调用方共享 |
| `Tonemap` | 色调映射曲线枚举，见常量行 |
| `Grade` | 整条后期链（滤镜链序：泛光→晕影→LUT→色调映射）；nil LUT 跳 LUT 级、TonemapNone 跳 tonemap 级；每级自出一张新图，四级链同时至多两张中间图，永不碰调用方像素 |
| `Vignette` | 晕影：Inner 内全亮、Outer 外乘 (1−Strength)、中间 Hermite 平滑过渡；Center 归一化 UV，距离为普通 UV 欧氏长度（宽帧横向衰减更快属设计内，将来加宽高比校正模式只增不改） |
| `Noise` | Warp 采样的偏移场：哪个格 `Seed`、波纹多密 `Freq`（周期/像素）、场随时间漂多快 `Drift`（格/秒） |
| `Warp` | 热浪/水波采样器：像素从 p+OffsetAt(p,t) 处读，偏移按 Strength 缩放；零值合法且精确恒等 |

#### 函数（18）

| 符号 | 说明 |
|------|------|
| `NewImage` | 建 W×H 全透明黑图：尺寸非正 InvalidArg、超 MaxImagePixels OutOfMemory |
| `NewImageFromColors` | 从像素拷贝建图（行主序，len=W×H）：坏尺寸 InvalidArg、长度不符 BadData、超预算 OutOfMemory、NaN/Inf 分量 BadData 绝不静默收像素 |
| `NewBloom` | 建泛光：Threshold [0,1]、Sigma [0,MaxBloomSigma]、Intensity [0,MaxBloomIntensity] 且全部有限，否则 InvalidArg 不建 |
| `NewCustom` | 建挂钩（拷贝参数）：空/超长名 InvalidArg、未知名 Unsupported、坏参数 InvalidArg |
| `NewIdentity` | 建「什么都不做」挂钩（无参数、永不降级） |
| `NewOutline` | 建描边挂钩：低于阈值像素的方形邻域（宽 width 内）有不透明邻居时，该像素变 (r,g,b,1) |
| `NewDissolve` | 建溶解挂钩：逐像素哈希 > amount+edge 保原样、边缘带显示 (r,g,b,1)、低于 amount 全透明 |
| `NewRegistry` | 建空挂钩表 |
| `NewLUT` | 建表（拷贝 r/g/b）：三表同长且在 [MinLUTSize,MaxLUTSize]、每项有限 [0,1]，否则 InvalidArg |
| `NewIdentityLUT` | 建恒等表：table[i]=i/(size−1)；size 越界 InvalidArg |
| `ParseTonemap` | "none"/"reinhard"/"aces"/"filmic" → Tonemap；其余 InvalidArg 返回 TonemapNone |
| `ApplyTonemap` | 整图色调映射出新图；None 返回拷贝（仍隔离不别名）；nil 图 InvalidArg、坏图 BadData、坏曲线/曝光 InvalidArg |
| `NewGrade` | 建调色链并逐级校验（nil lut 跳 LUT）；坏 tonemap InvalidArg；曝光只在 tonemap 非 None 时检查 |
| `NewVignette` | 建晕影：center 有限、Inner [0,Max]、Outer (Inner,Max]、Strength [0,1]，否则 InvalidArg 不建 |
| `DefaultNoise` | 冻结的水波场：seed 20260915、每 32px 一波、缓慢向右下漂；测试钉住 |
| `NewWarp` | 建 Warp：strength 有限非负、Freq 有限正、Drift 有限；坏参 InvalidArg 不建 |
| `SampleClamped` | 带钳边读一个 RGBA8 像素：越界坐标钉到最近边缘像素，与 GPU clamp-to-edge 采样器一致，两侧边界一致；坏参 InvalidArg 不读 |
| `WarpRGBA` | 把 w 施加到 src（w×h RGBA8）时刻 t，返回新的中间拷贝：dst 按 WarpPoint 四舍五入取最近像素、钳边；src 永不写、调用方保活帧换入返回拷贝；strength 0 返回精确拷贝；长度不符 BadData、超预算 OutOfMemory、零负边长 InvalidArg |

#### 方法（34，按接收者分组）

| 符号 | 说明 |
|------|------|
| `Image.At` / `Image.Set` | 读 (x,y) 像素（越界 false）/ 写像素（坏坐标或坏图 false；非有限 c 拒收不落） |
| `Image.Clone` / `Image.ApproxEqual` | 新拷贝（nil/坏图返回 nil 不 panic）/ 同尺寸且每分量差小于 eps（eps≤0 即精确相等） |
| `Bloom.Validate` / `Bloom.IsIdentity` | 字段非有限或越界报 InvalidArg / 是否几乎不加东西（零 intensity 恒是；零 σ 只是不糊、亮罩仍会点亮，不算恒等） |
| `Bloom.Apply` | src + 模糊亮部 = 新图：像素数学走 render 层 float 域泛光核（双腿一份实现），超亮辉光过级不丢光；本级仍交回 LDR 颜色（[0,1] 钳）；颜色过 render float32 存储（1e-7 相对）、alpha 不进核原样返回；src 永不改；nil/坏/超宽 src 各自报错且不留分配 |
| `Custom.Validate` / `Custom.IsIdentity` / `Custom.Degraded` | 名与参数校验（未知名 Unsupported）/ 是否空挂钩 / 冻结的 CPU 回退标记（identity=false 精确；outline/dissolve=true 是 GPU 过滤着色器替换前的 CPU 参考路径；调用方按此位控 parity） |
| `Custom.Param` / `Custom.SetParam` | 读单参数（缺键 false）/ 换单参数（未知键或越界值 InvalidArg 且 c 不变） |
| `Custom.Apply` | 跑挂钩出（新图, 降级位, 错误）：src 永不改；nil src InvalidArg、坏 src BadData、超预算 OutOfMemory、坏挂钩 InvalidArg、未知挂钩 Unsupported；合法 src 配坏挂钩仍返回占位拷贝，主路径永不断 |
| `Registry.Len` / `Registry.Attach` / `Registry.Detach` / `Registry.Get` | 活句柄数 / 校验并登记返回新句柄（坏挂钩不挂、满表 OutOfMemory）/ 摘除（非正 id InvalidArg、缺失 NotFound）/ 取挂钩拷贝（缺失 false） |
| `Registry.Apply` | 按句柄对 src 调挂钩：坏 src 报图错误、缺句柄 NotFound（src 合法时附占位拷贝）、存量坏挂钩报自身错误；合法 src 永远拿得到可画占位 |
| `LUT.Validate` / `LUT.Sample` / `LUT.Apply` | 零值或构造器会拒的表报 InvalidArg / 单通道值过表（探针用；坏通道返回 0 不 panic）/ 整图过表出新图（alpha 精确保留、src 不改；nil/坏图/坏表各自报错） |
| `Tonemap.String` / `Tonemap.Map` | 稳定日志名 / 单通道值按曲线与曝光映射（探针用） |
| `Grade.Stages` / `Grade.Degraded` / `Grade.ExposureValue` | 冻结的滤镜链级序 / 冻结 CPU 回退标记（tonemap 级开着即 true——GPU 着色器未建、CPU 参考在跑；有位无日志=假绿 bug）/ 冻结曝光 + 是否开 tonemap（None 报 (0,false)） |
| `Grade.Apply` | 跑全链（泛光→晕影→LUT→tonemap，可跳级）出最终新图；src 永不改；nil src InvalidArg、坏 src BadData |
| `Vignette.Validate` / `Vignette.FactorAt` / `Vignette.Apply` | 字段校验 / 归一化 uv 处的亮度乘数（Inner 内 1、Outer 外 1−Strength、中间平滑；探针与测试用）/ 整图按晕影系数变暗出新图（RGB 缩放、alpha 精确保留、src 不改） |
| `Warp.SetStrength` / `Warp.SetNoise` | 换波纹幅度 / 换采样场；坏输入 InvalidArg 且 w 不变 |
| `Warp.OffsetAt` / `Warp.WarpPoint` | 画面点 p 时刻 t 的采样偏移（strength·noise，每轴一通道）/ 采样来源点 p+OffsetAt；两者皆全函数：strength 0 精确恒等、任何非有限输入得零偏移，不 panic 不漏 NaN |

## 9. `engine/input` — 动作映射（71 条：顶层 25 + 方法 46）

> 冻结 CPU 侧动作映射（2026-09-15 冻结口子，只做加法）。**不画不响，只把硬件数字变成动作数字**：调用方在边界处把平台码一次性映射成这里存的 int id（如 ui/input.Key→int），摇杆值/双指缩放增量/震动时长从外面喂进来。一个动作名拥有一条死区 + 绑定列表，任何绑定输入都能触发动作，上报强度=过死区后所有绑定的最大值；空绑定列表合法且保持静默。来源四类：按键（数码 code>0）、手柄键（pad≥0, button≥0）、手柄轴一侧（sign ±1）、双指缩放（张开 +1、捏合 −1）；Device −1（DeviceAny）匹配任意手柄。另含两件套：**Buffer**（连招缓存窗，早按一点点不丢）与 **TouchTracker**（活动触点跟踪，上限 10 指）。

#### 常量（3）

| 符号 | 说明 |
|------|------|
| `DeviceAny` | 手柄通配（−1）；按键与缩放绑定必须用它 |
| `MaxBuffered` / `MaxTouches` / `DefaultBufferWindow` | Buffer 存键上限 64 / TouchTracker 同时触点上限 10 / 连招缓存窗默认 150ms |

#### 类型（4）

| 符号 | 说明 |
|------|------|
| `Source` | 绑定来源枚举：`SourceKey`（数码键）、`SourcePadButton`（手柄键）、`SourcePadAxis`（摇杆轴一侧）、`SourcePinch`（缩放方向） |
| `Binding` | 一条硬件输入→动作绑定：来源 + 设备 + 码 + 方向符号（数码来源 0，轴/缩放 ±1） |
| `Map` | 动作映射总表：动作名+绑定+活硬件状态（键/手柄键/轴）+缩放累计+震动请求；不画不响 |
| `BufferedPress` / `Buffer` / `Touch` / `TouchTracker` | 缓存的一条按键（动作名+压下时刻）/ 连招缓存（窗内按键可稍迟消费，Consume 取最老活匹配）/ 一根被跟踪手指（id+末位+时刻）/ 触点→位置跟踪表（id 由调用方分配 ≥0） |

#### 函数（7）

| 符号 | 说明 |
|------|------|
| `Valid` | s 是否冻结来源 |
| `ParseSource` / `AllSources` | 按冻结名查来源（空/未知名 InvalidArg 绝不猜）/ 按声明序返回全部来源 |
| `NewKeyBinding` / `NewPadButtonBinding` / `NewPadAxisBinding` / `NewPinchBinding` | 建数码键绑定（code>0）/ 手柄键绑定 / 摇杆轴一侧绑定（sign 须 +1/−1）/ 缩放方向绑定（+1 张开 −1 捏合） |
| `NewMap` | 建空动作映射表 |
| `NewBuffer` / `NewTouchTracker` | 建缓存（窗须 >0，坏窗 InvalidArg 返回 nil）/ 建空触点跟踪器 |

#### 方法（46，按接收者分组）

| 符号 | 说明 |
|------|------|
| `Source.String` | 冻结名（坏来源 "unknown"） |
| `Map.AddAction` | 注册动作与死区 [0,1)：空名/重名/越界死区 InvalidArg 不落库 |
| `Map.Has` / `Map.Actions` / `Map.ActionCount` | 是否注册 / 全部动作名（排序）/ 动作数 |
| `Map.RemoveAction` | 删动作：空名 InvalidArg、缺失 NotFound；其余动作不动 |
| `Map.SetDeadzone` / `Map.Deadzone` | 调摇杆漂移门限（坏值 InvalidArg 保旧）/ 读死区（缺失 ok=false） |
| `Map.Bind` / `Map.Unbind` | 加一条绑定（重映射只加不换；完全重复 InvalidArg 不落）/ 摘一条绑定（不存在 NotFound 不动） |
| `Map.Rebind` / `Map.ClearBindings` / `Map.Bindings` | 整表替换绑定（空表清空保持静默；坏项与表内重复 InvalidArg 保旧表）/ 摘光绑定（动作保留且静默）/ 绑定列表拷贝 |
| `Map.SetKey` / `Map.SetPadButton` / `Map.SetPadAxis` | 喂数码键态（code>0）/ 喂手柄键态 / 喂摇杆轴值 [−1,1]（越界有限值钳端点、非有限 InvalidArg 保旧） |
| `Map.AddPinchDelta` / `Map.Pinch` / `Map.ClearPinch` | 喂一步双指缩放（距上步的乘法因子：1.2 张开 0.8 捏合；累计 (factor−1) 钳 [−1,1]，张开驱动 +1 绑定、捏合驱动 −1 绑定；非有限与非正 InvalidArg）/ 读累计缩放幅 / 清零 |
| `Map.ResetInputs` | 清全部活硬件态（键/手柄键/轴/缩放）；绑定、死区、震动请求保留 |
| `Map.Strength` | 过死区后的动作强度 [0,1]（全部绑定取最大；未绑定动作 0 无错；恒有限不出 NaN） |
| `Map.Pressed` | 死区后强度是否大于 0 |
| `Map.Vector` | 四个动作合成一根摇杆向量：x 随 posX 增 negX 减、y 同理；长度超 1 归一到 1（斜向不超速）；缺名 NotFound 返回零向量，绝不猜方向 |
| `Map.StartRumble` / `Map.StopRumble` / `Map.UpdateRumbles` / `Map.RumbleLevels` / `Map.RumbleActive` | 请求手柄震动（弱/强强度 [0,1] + 时长；只记请求，硬件归调用方；同手柄新请求覆盖旧的）/ 取消（未知手柄静默）/ 推进震动时钟并到期回收 / 读请求强度（不活跃 0,0,false）/ 是否有活请求 |
| `Buffer.Window` / `Buffer.SetWindow` | 读缓存窗 / 调缓存窗（坏窗 InvalidArg 保旧） |
| `Buffer.Push` | 缓存一条动作按压：空动作/负时刻 InvalidArg 不落；满了丢最老腾位、新条照存 |
| `Buffer.Prune` / `Buffer.Clear` | 丢弃过期按压 / 清空全部 |
| `Buffer.Consume` / `Buffer.Peek` | 取走（并移除）该动作最老的活匹配（命中 true；未中 false 无错；空动作/负时刻 InvalidArg 返 false 不动）/ 只探不取 |
| `Buffer.Len` / `Buffer.LiveCount` / `Buffer.Presses` | 存量数（含未 Prune 的过期条）/ 当前时刻可消费数 / 存量拷贝 |
| `TouchTracker.Begin` / `TouchTracker.Move` / `TouchTracker.End` | 开始跟踪触点（重复 id InvalidArg、满 MaxTouches OutOfMemory，活集不动）/ 更新活触点位置（未知 id NotFound 不动）/ 结束跟踪（抬指或打断；未知 id NotFound 不动不 panic） |
| `TouchTracker.Position` / `TouchTracker.ActiveCount` / `TouchTracker.ActiveIDs` / `TouchTracker.Touches` / `TouchTracker.Clear` | 末位（未知 ok=false）/ 活触点数 / 活 id 列表（排序新切片）/ 活触点表（按 id 排序）/ 清空全部 |

## 10. `engine/light` — 2D 灯光数学（150 条：顶层 84 + 方法 66）

> 冻结 CPU 侧 2D 灯光数学（2026-09-15/16 冻结口子，只做加法），fx 调色之后的夜景光照。**点光**按距离衰减（Hermite smoothstep of t=1−dist/range）、可选 Cookie 手电遮片（双线性采样，缺片回退裸灯泡）、只照层掩码内的绘制带；**方向光**照命中层均匀一致。全局夜色恒定生效：零灯也显示夜色画面、绝不黑屏（夜色通道下限 `MinNightChannel`）。逐像素公式 `base·night + base·Σ(factor·intensity·lightRGB)` 钳 [0,1]，alpha 原样通过；纯函数，CPU/GPU 双腿共享同一条路径（`Lit/LitGPU`、`Blocked/BlockedGPU` 等 GPU 镜像逐位一致）。**6.3 阴影**：每个遮挡体是闭合多边形环，墙棱横穿光线时像素背光（端点相接不挡，墙面故意留亮）；背光系数 1−Occlusion，全影像素保 base·night 不变黑；零 Length 不投影（合法 no-op）。**法线贴图**（6.2）：凹凸表 1:1 对齐受光图，灯吊在平面上方 Height 处，朝灯的余弦 NdotL 调制——朝灯面亮于平面、平面亮于背面；缺表读作平面。**粗糙度**：Blinn-Phong 高光（镜面 shininess 64 滑到哑光 8），Diffuse+Specular 权重和偏离 1 报 BadData 绝不静默归一。**软阴影**：PCF5/PCF13 多 tap 边缘柔化 + 三级降级链（灯数超预算/OOM 时逐级下降并强制留痕）。热路径全函数：非有限输入一律 0 贡献，不 panic 不漏 NaN。

#### 常量（24）

| 符号 | 说明 |
|------|------|
| `MaxLightRange` / `MaxIntensity` / `MaxLights` / `MaxCookieDimension` / `MaxImagePixels` | 预算线：射程 8192 / 强度 8 / 灯数 256 / Cookie 边 512px / 受光图 1600 万像素；超预算图报 OutOfMemory、超数场景与越界标量报 InvalidArg，先校验后分配 |
| `LayerWorld` / `LayerFX` / `LayerUI` | 灯触及的绘制带（数值镜像 engine/sprite 分带，调用方传同一组数） |
| `KindPoint` / `KindDirectional` | 灯形：点光 / 方向光 |
| `RoughWeightTol` | Diffuse+Specular 权重和的允许漂移（0.001），更大 BadData 绝不静默归一 |
| `MaxOccluderVerts` / `MaxOccluders` / `MaxShadowLength` | 阴影预算：单遮挡体顶点 64 / 遮挡体 256 个 / 影长 8192 |
| `MaxShadowLights` / `MaxPCF13Lights` / `LargeLightPixelsWarn` | 60fps 阴影灯数上限 10 / PCF13 重 tap 灯数上限 2 / 单灯覆盖超 2048 像素告警线 |
| `MinNightChannel` / `MinLitPixels` / `MinLitGain` | 夜色通道可读下限 0.03（绝不黑屏）/ 增益门的受光像素数下限 1000 / 受光像素均增益须达到 0.6 |
| `DefaultSoftRadius` | 软阴影 tap 步长默认 0.5（调用方传 0 时启用） |
| `SoftNone` / `SoftPCF5` / `SoftPCF13` / `DefaultSoftMode` | 阴影边缘档：单硬 tap / 中心+4 轴向 tap / 中心+12 环 tap（默认 PCF5 中档） |

#### 类型（17）

| 符号 | 说明 |
|------|------|
| `Layer` | 灯触及的绘制带枚举 |
| `Kind` | 灯形枚举（点/方向） |
| `Cookie` | 手电遮片：W×H 个 [0,1] 采样行主序；nil 即裸灯泡；入时一律拷贝 |
| `Light` | 一盏冻结的 2D 灯：点光用 Pos/Range/Cookie，方向光用 Dir（Pos/Range/Cookie 无效）；Layers 是带位掩码；Cookie nil=裸灯泡 |
| `Image` | 一张隔离的受光中间图：W×H 基色 + 逐像素绘制带，行主序；Origin 是像素 (0,0) 中心的世界位、Step 是每像素世界单位；构造拷贝、Apply 出新图 |
| `Scene` | 一个冻结的夜：全局夜色 + 灯表（NewScene 拷贝切片）；Apply 在 fx 调色后给 src 上光 |
| `Normal` | 光照空间表面方向：X 右 Y 下（图像像素）Z 出屏；单位长为常规，坏值经 Normalize 归为平面 |
| `NormalMap` | 一张隔离凹凸表：W×H 法线行主序，与受光图 1:1 对齐；构造拷贝、At 只读 |
| `NormalScene` | 一个冻结的法线夜景：夜色 + 灯表 + 灯离平面高度 Height；用 1:1 法线表上光 |
| `RoughMap` | 一张隔离粗糙度表：W×H 个 [0,1] 采样（0 镜面、1 哑光），与受光图 1:1 |
| `RoughWeights` | 一份灯贡献拆分：漫射 + 高光，和须在 RoughWeightTol 内 |
| `RoughScene` | 一个冻结的粗糙夜景：夜色 + 灯表 + 灯高 + 拆分；nil 表回退（平面法线+FlatRough）不崩、坏表 BadData |
| `Occluder` | 一个冻结的 2D 阴影遮挡体：≥3 个有限顶点的闭合多边形环，绕向不限；入时一律拷贝 |
| `Shadow` | 一套冻结遮挡体：每个遮挡体对每盏灯同等遮挡（Godot LightOccluder2D 同念）；Occlusion [0,1] 定背光暗度（0 保光、1 只剩夜色不变黑）；Length 定影长（0 不投影） |
| `SoftMode` | 阴影边缘柔化档枚举 |
| `SoftShadow` | 硬阴影 + 边缘档：Mode 定 tap 数、Radius 定每 tap 世界步长（0=默认 0.5） |
| `FrameBill` | 一帧灯光账单：逐灯覆盖像素数、总足迹、tap 数、成本（足迹×tap） |

#### 函数（43）

| 符号 | 说明 |
|------|------|
| `LayersFor` / `HitsLayer` | 把绘制带打包成位掩码（空=照不亮任何东西）/ 掩码是否覆盖某带（越界带不命中） |
| `ParseKind` | "point"/"directional" → Kind；其余 InvalidArg 返回 KindPoint |
| `NewCookie` | 建遮片（拷贝 mask）：坏尺寸 InvalidArg、长度不符 BadData、非有限或越界样本 BadData、超尺寸 OutOfMemory |
| `NewPointLight` / `NewDirectionalLight` | 建点光（Pos 有限、Range (0,MaxLightRange]、颜色分量 [0,1]、强度 [0,MaxIntensity]、cookie nil 或有效）/ 建方向光（Dir 有限非零；方向专为 6.3 阴影携带，6.1 系数在命中层均匀） |
| `LitCPU` / `LitGPU` | 单像素上光（CPU 路径）/ GPU 镜像逐位一致；坏夜色读白（不黑屏）、坏灯贡献 0、alpha 原样通过 |
| `NewImage` / `NewImageFromColors` | 建 W×H 透明图（LayerWorld；尺寸非正/step 非正非有限/origin 非有限 InvalidArg、超 MaxImagePixels OutOfMemory）/ 从像素与层拷贝建图（长度不符 BadData、非有限像素 BadData） |
| `NewScene` | 建夜景（拷贝灯表）：坏夜色/坏灯 InvalidArg、超 MaxLights InvalidArg 不建 |
| `N` / `FlatNormal` | 从分量建法线 / 缺省表面法线（正对观察者；缺失或退化数据回退到它，绝不黑） |
| `NdotL` | 表面法线与入射光的漫射余弦：内部各自归一、负值钳 0、结果 [0,1]；退化输入读作 FlatNormal 不出 NaN |
| `LightDir` | 表面点 p 指向灯的单位向量（灯吊在平面上方 height 处）：点光瞄准灯泡、方向光沿 Dir 瞄向无穷（Dir 指向灯，Z 来自 height；6.3 阴影沿 −Dir 延伸）；几何坏 false |
| `NewNormalMap` / `NewNormalMapFromColors` | 建凹凸表（拷贝法线；尺寸非正 InvalidArg、超预算 OutOfMemory、长度不符或非有限 BadData）/ 解码「中灰朝上」RGB 表（flat=(0.5,0.5,1)，X=2R−1 等，alpha 忽略） |
| `NormalFactor` / `NormalFactorGPU` | 法线受光系数（CPU 路径）/ GPU 镜像一致；恒不 NaN 不 panic |
| `LitNormalCPU` / `LitNormalGPU` | 单法线像素上光（坏夜色读白、坏灯 0、退化法线读作平面）/ GPU 镜像逐位一致 |
| `NewNormalScene` | 建法线夜景（拷贝灯表）：坏夜色/灯/高度 InvalidArg、超灯数 InvalidArg 不建 |
| `FlatRough` / `NewRoughMap` | 缺表回退粗糙度（中间哑光，不黑不崩）/ 建粗糙度表（尺寸/长度/值域校验，超预算 OutOfMemory） |
| `NewRoughWeights` / `DefaultRoughWeights` | 建权重（非有限/负/和漂超 tol 均 BadData）/ golden 图用的冻结拆分（大半漫射 + 可见高光抬升） |
| `RoughSpec` | 单灯 Blinn-Phong 高光：半向量点积、镜面 shininess 64 滑到哑光 8、按 (1−rough) 缩放（哑光杀高光）；结果 [0,1]，退化/非有限读 0 |
| `RoughFactor` / `RoughFactorGPU` | 粗糙受光系数（CPU）/ GPU 镜像一致 |
| `LitRoughCPU` / `LitRoughGPU` | 单粗糙像素上光（坏夜色白、坏灯 0、退化法线平面、坏粗糙度 FlatRough、坏权重 0 贡献）/ GPU 镜像逐位一致 |
| `NewRoughScene` | 建粗糙夜景：坏夜色/灯/高度 InvalidArg、超灯数 InvalidArg、坏权重 BadData，出错不建 |
| `NewOccluder` | 建遮挡体（拷贝 pts）：<3 点或非有限顶点 InvalidArg、超 MaxOccluderVerts OutOfMemory |
| `NewShadow` | 建阴影集（拷贝遮挡体）：Occlusion 出 [0,1]、Length 非有限/负/超限、超 MaxOccluders 均 InvalidArg；坏遮挡体带自身错码使建失败 |
| `ParseSoftMode` / `NewSoftShadow` | "None"/"PCF5"/"PCF13" → SoftMode / 建软阴影（坏档或坏半径 InvalidArg、坏阴影带自身错码；半径 0 读作默认） |
| `ClampNightFloor` / `CheckNightFloor` / `NightOnlyPixel` | 把暗夜抬到可读下限（坏夜色读白，否则 RGB 钉在 MinNightChannel）/ 校验夜色（无效或通道低于下限 InvalidArg）/ 火把落空参考像素：基色×抬底夜色 |
| `CoverPixels` / `OverShadowLightCount` / `LitStats` | 数一盏灯的实际贡献像素数（FactorAt>0；nil/坏图读 0 不 panic）/ 灯数是否超 60fps 上限 / 数受光像素与其均增益（增益=受光亮度−纯夜色基线；受光=增益≥MinLitGain；坏输入 0,0） |

#### 方法（66，按接收者分组）

| 符号 | 说明 |
|------|------|
| `Kind.String` | 稳定日志名 |
| `Cookie.Validate` / `Cookie.At` / `Cookie.Sample` | 零值 InvalidArg、坏样本 BadData、超尺寸 OutOfMemory / 读单个纹素（越界或坏表 false）/ uv [0,1] 双线性钳边采样（无效 Cookie 或非有限 uv 读 0；FactorAt 对缺片代 1，直接 Sample 保持 fail-closed） |
| `Light.Validate` / `Light.HitsLayer` | 坏灯报 InvalidArg（点查 Pos/Range、方向查 Dir、共查颜色/强度、cookie 有则须过校验）/ 灯是否触及某带 |
| `Light.FactorAt` / `Light.FactorGPU` | p 处该带的几何系数（CPU）：衰减×cookie，命中方向光 1、未命中或坏输入 0；恒不 NaN 不 panic / GPU 镜像同式逐位一致 |
| `Image.PosAt` / `Image.At` / `Image.Set` / `Image.Clone` / `Image.ApproxEqual` | 像素中心世界位 / 读像素与层 / 写像素与层（坏坐标/坏图/非有限 false 不落）/ 新拷贝（nil/坏图 nil）/ 几何与逐像素逐层一致（eps≤0 精确） |
| `Scene.Validate` / `Scene.Lit` / `Scene.LitGPU` / `Scene.Apply` / `Scene.ApplyCPU` / `Scene.ApplyGPU` | 校验 / 单像素过景上光（CPU）/ GPU 镜像 / 整图上光（src 不改；nil InvalidArg、坏图 BadData、坏景 InvalidArg）/ 显式 CPU 名 / GPU 镜像逐位一致 |
| `Normal.Normalize` | 单位化：零/非有限/溢出输入返回 FlatNormal（缺倾斜读作平面，不出 NaN） |
| `NormalMap.Validate` / `NormalMap.At` | 校验（零值 InvalidArg、长度不符或非有限 BadData、超预算 OutOfMemory）/ 读法线（退化零法线是合法存储，着色读作平面） |
| `NormalScene.Validate` / `NormalScene.Lit` / `NormalScene.LitGPU` / `NormalScene.Apply` / `NormalScene.ApplyCPU` / `NormalScene.ApplyGPU` | 校验（夜色/灯/高度/灯数）/ 单法线像素过景 / GPU 镜像 / 整图配法线表上光（两输入均不改；nil 表 InvalidArg、坏或错尺寸表 BadData）/ 显式 CPU 名 / GPU 镜像 |
| `RoughMap.Validate` / `RoughMap.At` | 校验 / 读粗糙度（越界或坏表读 FlatRough 且 false，不 NaN 不 panic） |
| `RoughWeights.Validate` | 非有限/负/和漂超 tol 报 BadData |
| `RoughScene.Validate` / `RoughScene.Lit` / `RoughScene.LitGPU` / `RoughScene.Apply` / `RoughScene.ApplyCPU` / `RoughScene.ApplyGPU` | 校验 / 单粗糙像素过景 / GPU 镜像 / 整图配双表上光（nil 表回退不崩、坏表 BadData）/ 显式 CPU 名 / GPU 镜像 |
| `Occluder.Validate` | 顶点 <3 或非有限 InvalidArg、超数 OutOfMemory |
| `Shadow.Validate` / `Shadow.Blocked` / `Shadow.BlockedGPU` / `Shadow.FactorFor` / `Shadow.FactorForGPU` | 校验（遮挡参数/数量/遮挡体）/ p 是否被遮挡体从灯 l 处影住（CPU：墙棱横穿光线的几何判定）/ GPU 镜像逐位一致 / p 处的阴影乘数 / GPU 镜像 |
| `Shadow.Lit` / `Shadow.LitGPU` / `Shadow.Apply` / `Shadow.ApplyCPU` / `Shadow.ApplyGPU` | 单像素过「灯+夜色+本阴影」（CPU）/ GPU 镜像 / 整图上光（src 不改；nil InvalidArg、坏图 BadData、坏影 InvalidArg）/ 显式 CPU 名 / GPU 镜像 |
| `SoftMode.String` / `SoftMode.Validate` / `SoftMode.Taps` | 稳定日志名 / 未知档 InvalidArg / tap 数 1/5/13（未知读 1 绝不 0） |
| `SoftShadow.Validate` / `SoftShadow.Taps` / `SoftShadow.SoftFactor` / `SoftShadow.SoftFactorGPU` | 校验（档/半径/影）/ 当前档 tap 数 / p 处软阴影乘数（CPU 多 tap 均值）/ GPU 镜像一致 |
| `SoftShadow.SoftLit` / `SoftShadow.SoftLitGPU` | 单像素过软 tap（CPU）/ GPU 镜像 |
| `SoftShadow.Bill` / `FrameBill.WarnLarge` | 计一帧账（逐灯足迹+总成本）/ 报第一个覆盖超告警线的灯 |
| `SoftShadow.OverPCF13Count` / `SoftShadow.DowngradeOOM` / `SoftShadow.AutoDowngradeForLights` / `SoftShadow.DowngradeOnAllocFail` | PCF13 灯数是否超小上限 / 降一级并记日志（nil 软影或 nil 日志拒绝——静默降级即是失败；已 None 只记请求不动）/ 灯数超预算自动降级（PCF13 超小上限或任意档超灯数上限；发生必有日志）/ err 为 OutOfMemory 时降级（其他错码与 nil 错误永不降） |
| `SoftShadow.Apply` / `SoftShadow.ApplyCPU` / `SoftShadow.ApplyGPU` | 软 tap 整图上光并返回帧账单（src 不改；坏软影或灯数 InvalidArg、nil 图 InvalidArg、坏图 BadData）/ 显式 CPU 名 / GPU 镜像（同 tap 同像素同账单） |

## 11. `engine/loop` — 固定步跑道（17 条：顶层 4 + 方法 13）

> 冻结所有 2.5D 玩法跑的通用跑道（2026-09-30 E05 冻结口子，只做加法）。一帧永远同一顺序：**排空队列的网络包 → 按注册序逐固定步跑全部系统 → 把混合因子交给 submit 钩子**。跑道不写任何玩法（无怪/无场景/无胜负）：玩法注册系统，跑道只跑表。时间一律 core.Duration 整数毫秒走在固定节拍上，慢机快机逐拍一致；跨边界的只有 core 数字，render 类型由 submit 钩子的主人去转。零值不可用（New 是唯一入口）；非并发安全；nil `*Loop` 不 panic。

#### 常量（1）

| 符号 | 说明 |
|------|------|
| `MaxPendingPackets` | 排队网络包上限（1024）：超限注入报 OutOfMemory 且不落库，死网络撑不涨跑道 |

#### 类型（2）

| 符号 | 说明 |
|------|------|
| `System` | 一个注册的步进工人：名字 + 每 tick 更新函数（收到固定步宽）；系统不回报错误、自己管自己的错；nil 更新不注册 |
| `Loop` | 通用跑道：固定节拍 + 有序系统表 + 网络包口子 + 提交钩子 |

#### 函数（1）

| 符号 | 说明 |
|------|------|
| `New` | 在固定步宽 dt 上建跑道：dt 非正报 InvalidArg 返回 nil，绝不猜节拍 |

#### 方法（13，均在 `Loop` 上）

| 符号 | 说明 |
|------|------|
| `Loop.AddSystem` | 表尾注册一个系统：空名/重名/nil 更新 InvalidArg 不落；nil 跑道 InvalidArg |
| `System.Name` | 返回注册名 |
| `Loop.Systems` / `Loop.SystemCount` | 注册名列表（按运行序，新切片）/ 注册数 |
| `Loop.SetPacketHook` / `Loop.InjectPacket` / `Loop.PendingPackets` | 插网络排空钩子（每个有排队包的 tick 前整队移交并清空；nil 钩子只存不排；传 nil 清钩子；钩子不得持有切片）/ 为下一 tick 排队一个包（空包 InvalidArg、满队 OutOfMemory；字节克隆不别名）/ 未排空包数 |
| `Loop.SetSubmit` | 插渲染交接钩子：每帧结束后带混合因子调用一次（零 tick 帧也调）；传 nil 清钩子 |
| `Loop.Frame` | 跑一个真实帧：按固定 tick 切分 → 每 tick 先排包再按序跑全部系统 → 提交混合因子；返回 (tick 数, alpha)；坏帧宽按固定节拍处理（停 0、超长钳 step.MaxFrame）；nil 跑道返回 0,0 且不调任何钩子 |
| `Loop.Dt` / `Loop.Steps` / `Loop.Elapsed` / `Loop.Alpha` | 固定步宽 / 已跑 tick 数 / 已跑模拟时间 / 当前渲染混合因子（nil 全报 0） |

## 12. `engine/nav` — 网格寻路（59 条：顶层 32 + 方法 27）

> 冻结 CPU 侧网格寻路（2026-10-05 S87 冻结口子，只做加法）。**只算数、不画不步进**：调用方烘焙一张 Grid、经 Queue 排队寻路请求、再沿返回路点移动 Agent。Godot 对齐（只借思路不搬码）：AStarGrid2D 八向默认手感（正交代价 1、对角 √2、octile 启发，对角步永不切墙角——两个正交邻格都可行才走）；NavigationAgent2D 的 target/next_path/desired_distance/max_speed 对应 Agent 的路点表/游标/到达容差/速度；NavigationPolygon 的 bake 对应 Bake（可通行网格由调用方的 solid 判定函数生成）。另含 JSON 可调的每帧寻路配额（Tuning）。挂未导出堆类型 `openHeap` 的 5 个导出方法名不属公开 API（见 §0 注）。

#### 常量（12）

| 符号 | 说明 |
|------|------|
| `DefaultSpeed` / `ArriveTol` | S87 真值：行走速度 200 世界单位/秒（导航演示手感）、到达容差 2 单位（desired_distance） |
| `StatusInvalid` / `StatusFound` / `StatusNoPath` / `StatusStartBlocked` / `StatusGoalBlocked` | 单次寻路的终态：查询本身坏（坏网格或越界格）/ 找到可行链 / 两端合法但不连通 / 起点实心 / 终点实心 |
| `MaxGridW` / `MaxGridH` / `MaxGridCells` | 烘焙上限：1024×1024 最大单烘（与 tilemap large 档同量级），超出报 OutOfMemory 绝不产半张网格 |
| `DefaultPerFrame` / `MaxPerFrame` | 每次 Update 计算的路数默认 8 / 上限 64（调参不能炸帧） |

#### 类型（8）

| 符号 | 说明 |
|------|------|
| `Cell` | 一个网格地址 (X,Y) |
| `Status` | 寻路终态枚举，见常量行 |
| `Agent` | 按固定速度走路点表的行走者：`Pos` 实时位置、`Next` 是 next_path 游标、`Done` 到达终态（距终点 ArriveTol 内）；网格真相归 FindPath，Agent 只积分运动 |
| `Grid` | 一张烘焙好的可通行网格：宽高 + 行主序实心标记（true 挡路）；零值无效，须经 NewGrid/Bake 构建 |
| `Request` | 一个排队的寻路任务：稠密 FIFO，ID 按入队序递增 |
| `Result` | 一个算完的终态：仅 StatusFound 时 Path 非 nil |
| `Queue` | 待算任务 + 已算终态的容器：网格真相与排空归调用方；Queue 不画不动 |
| `Tuning` | JSON 可调的队列文件形状：`{"per_frame": 8}` |

#### 函数（12）

| 符号 | 说明 |
|------|------|
| `ParseStatus` | 把 testdata 的 want 字符串映射成 Status |
| `FindPath` | 在 g 上跑 A*（返回链含两端）：八向禁切墙角；坏网格/越界端点 InvalidArg+StatusInvalid 空 path；实心端点 InvalidArg+StatusStartBlocked/GoalBlocked 空 path（终态不重试）；合法但不连通 StatusNoPath nil 错（终态不是崩溃） |
| `CellToWorld` / `WorldToCell` / `CellsToWorld` | 格→世界中心（origin 是 (0,0) 格左上、cellSize 是格边长）/ 世界点→所属格（出界 ok=false）/ 格链→世界路点（格中心） |
| `NewAgent` | 在 pos 建速度 speed 的行走者：pos 须有限、speed 有限且 >0，否则 InvalidArg |
| `NewGrid` | 建全可通行空网格：W/H 须在 [1,1024] 且 W×H≤MaxGridCells，越界 InvalidArg（过大 OutOfMemory） |
| `Bake` | 逐格采样 solidAt 建网格：nil 判定=全可通行；同尺寸+同判定答案恒产同网格同 Encode 字节 |
| `Decode` | 从 Encode 字节重建网格：长度须等于 W×H、每字节 0/1；尺寸错 InvalidArg（过大 OutOfMemory）、长度/值错 BadData；坏写入不建 |
| `NewQueue` | 建空队列（每帧排空 DefaultPerFrame 条） |
| `DrainFrames` | 预测排完 total 个任务需几次 Update：ceil(total/perFrame)；坏输入报 0 |
| `LoadTuning` | 解析队列调参：per_frame 须在 1..MaxPerFrame；语法错 BadData、范围错 InvalidArg |

#### 方法（27，按接收者分组）

| 符号 | 说明 |
|------|------|
| `Status.String` | 稳定终态名（JSON 与 testdata 用） |
| `Grid.Valid` / `Grid.W` / `Grid.H` | 是否可用网格 / 宽 / 高（坏网格 0） |
| `Grid.InBounds` / `Grid.SolidAt` / `Grid.SetSolid` / `Grid.SolidCount` | (x,y) 是否界内 / 是否挡路（越界与坏网格按挡路处理 fail-closed 且 ok=false）/ 翻转单格（越界或坏网格 InvalidArg 不动）/ 实心格数 |
| `Grid.Encode` / `Grid.Equal` | 渲染确定性的烘焙工件（行主序 0/1，y·W+x 下标；同输入恒同字节；坏网格 nil）/ 尺寸与每个实心标记全同才相等（坏网格互不相等） |
| `Agent.Pos` / `Agent.Speed` / `Agent.Done` / `Agent.Remaining` | 实时位置（nil 零）/ 速度 / 是否停在终点（空路点也算 done；nil 报 done）/ 未走路点数 |
| `Agent.Waypoints` / `Agent.SetWaypoints` / `Agent.SetCells` | 路点新拷贝 / 装新路线（从当前位置走向 pts[0]；空表停 done；非有限点 InvalidArg 保旧线）/ 装格链为世界路点（空链停 done；坏 cellSize InvalidArg） |
| `Agent.Step` | 向当前路点走 speed·dt：进入 ArriveTol 内消耗路点、吃掉最后一个停 done 并报 arrived；dt 须有限且 ≥0 否则 InvalidArg 原地不动；nil 报 InvalidArg |
| `Queue.PerFrame` / `Queue.SetPerFrame` | 排空数 / 调排空数（1..MaxPerFrame 之外 InvalidArg 保旧） |
| `Queue.Enqueue` | 追加一个任务返回 ID：起终点合法性在 Update 时对活动网格检查（点击入队快）；nil 队列报 −1 |
| `Queue.Pending` / `Queue.Done` / `Queue.Updates` / `Queue.Results` | 排队未算数 / 已算终态数 / Update 调用数 / 全部终态新拷贝（按计算序） |
| `Queue.Clear` | 清待算与已算；调参与 ID 账本保留（同会话 ID 永不重复） |
| `Queue.Update` | 对 g 算至多 perFrame 个待算任务并逐个追加 Result（found 与 no-path 都算终态）：坏网格 InvalidArg 一个不算、pending 保留；单任务坏格只标记该任务不挡本帧其余 |

## 13. `engine/particle` — 粒子与拖尾（90 条：顶层 42 + 方法 48）

> 冻结 CPU 侧粒子发射数学（2026-09-15 冻结口子，只做加法）。发射主参考 Godot CPUParticles2D：一个发射器拥有固定预算（Max）、每秒出生率、速度/寿命区间、重力、起止色 ramp、一种形状、一种湍流、一级子发射器。Spawn 按形状采样位置方向、按区间采样速度寿命、用种子生成器取回放种子；Update 先积分存量再按出生率补新（新生儿同 tick 从 Age 0 起步）；死亡时在死点生成 Sub.Count 个孩子（孩子不再生）。形状四种：点/锥（Dir±Angle 均匀展开）/盒（均匀采样+Dir）/环（环带均匀+径向向外）；湍流是 (Seed,Age) 的确定性旋涡（同种子回放同漂移，不按 tick 消耗生成器）；颜色 = Start.Lerp(End, AgeFrac)。超预算静默丢弃不报错（爆发不炸帧）。**Trail**（主参考 Godot Line2D）：轨迹环从老到新，宽度 TailWidth→HeadWidth 插值（刀光宽到窄）、颜色 Start（头）→End（尾）；Miter 尖角到 MiterLimit 后切 bevel；Verts 承载精确接头几何、Segments 承载可画中线（每段一个旋转图集精灵走既有 DrawAtlasEx，不开新提交路）。**GPUPool**（主参考 Godot GPUParticles2D）：位置走冻结的 5.1 Emitter 数学，每个活粒子按出生 Seed 挂一条 Trail（同种子同轨迹，53 位空间种子碰撞共用为设计内）。只算数不画，AppendToBatch 喂既有 sprite.Batch 后用 DrawAtlas 画。

#### 常量（15）

| 符号 | 说明 |
|------|------|
| `MaxParticlesCap` / `MaxSubPerDeath` / `MaxRate` / `MaxSpeed` | 发射器预算上限 8192 / 每死孩子上限 8（一级不递归）/ 每秒出生率上限 20000 / 采样速度上限 5000 单位每秒 |
| `ShapePoint` / `ShapeCone` / `ShapeBox` / `ShapeRing` | 出生位置规则：在 Origin / Origin±锥形展开 / 盒内均匀 / 环带均匀 |
| `JointMiter` / `JointBevel` / `JointRound` / `MiterLimit` / `RoundArcSteps` | 拖尾接头规则（Godot Line2D 同族）：尖角（超 MiterLimit 4 个半宽后切 bevel，尖刺不爆）/ 平切一对顶点 / 圆弧扇 RoundArcSteps=4 对内部顶点 |
| `MaxTrailPointsCap` | 单条拖尾环缓冲上限 512 |
| `MaxPoolTrailsCap` | 单池上限（=MaxParticlesCap：至多一粒子一尾，池子不许许诺超过 CPU 数学能生的量） |

#### 类型（13）

| 符号 | 说明 |
|------|------|
| `Particle` | 一个活的 CPU 粒子：位置/速度（世界单位）、年龄/寿命（整数毫秒）、回放种子 [0,1)、子代标记 |
| `ShapeKind` | 出生位置规则枚举，见常量行 |
| `Shape` | 一条出生规则：Kind 选位置采样、Dir 选速度方向（锥加展开、环改径向）、Angle 锥半角（弧度）、Extents 盒半尺寸、Inner/Outer 环半径 |
| `Turbulence` | 确定性漂移：Strength（世界单位/秒²）+ Scale（弧度/秒）；零强度即关闭 |
| `Sub` | 一级孩子规则：非孩子的死亡按 Count 生成孩子（速度/寿命区间），孩子继承死点+父速度四分之一、不再生 |
| `EmitterConfig` | 冻结的出生配方：Origin 出生中心、Rate 每秒、Max 活跃预算、速度/寿命区间、重力、起止色、形状、湍流、孩子规则 |
| `Emitter` | 活跃集 + 种子生成器的持有者：同配方同种子回放同出生；非并发安全 |
| `GPUPool` | 5.2 批量粒子池：位置走冻结 5.1 Emitter（Spawn/Update 语义不变、同一对象），每个活粒子按出生 Seed 挂一条 Trail；画走既有 sprite Batch/DrawAtlas；非并发安全 |
| `JointKind` | 拖尾接头规则枚举，见常量行 |
| `TrailConfig` | 冻结拖尾配方：MaxPoints 环上限、Head/TailWidth 刀锋收窄、Start/End 头尾色 ramp、Joint 接头规则 |
| `Trail` | 一条轨迹带：点从老（尾）到新（头）；超 MaxPoints 丢最老；非并发安全 |
| `TrailVert` | 一个带状顶点对：Center + 沿接头法线半宽处的 Left/Right 边；圆角扇出同 Center 的额外对 |
| `TrailSegment` | 一段可画中线：两端点/角度/两端宽/两端色——窗把每段转成一个旋转图集精灵 + 一个接头方块，不开新提交路 |

#### 函数（14）

| 符号 | 说明 |
|------|------|
| `ParseShape` | "point"/"cone"/"box"/"ring" → kind；否则 ShapePoint 且 ok=false（绝不猜） |
| `PointShape` / `ConeShape` / `BoxShape` / `RingShape` | 建点形（Dir 有限，零允许=零速度）/ 锥形（Dir 有限非零、半角 [0,π]）/ 盒形（Extents 有限 ≥0）/ 环形（半径有限 ≥0 且 Inner≤Outer；方向径向向外）；坏输入 InvalidArg |
| `NoTurbulence` / `NewTurbulence` | 关闭漂移 / 建漂移（Strength 有限 ≥0、Scale 有限；坏输入 InvalidArg） |
| `NoSub` / `NewSub` | 关闭孩子规则 / 建孩子规则（Count [0,MaxSubPerDeath]；Count 0 忽略其余；正 Count 要求速度有限 Min≤Max 且 ≤MaxSpeed、寿命正且 Min≤Max） |
| `NewEmitter` | 按配方+回放种子建发射器：坏配方 InvalidArg（Max 超帽 OutOfMemory）返回 nil |
| `ParseJoint` | "miter"/"bevel"/"round" → kind；否则 JointMiter 且 ok=false |
| `NewTrailConfig` | 建拖尾配方：MaxPoints [1,MaxTrailPointsCap]、宽度有限 ≥0、颜色有限、接头已知；否则 InvalidArg |
| `NewTrail` | 按配方建空拖尾：坏配方 InvalidArg 返回 nil |
| `NewGPUPool` | 按 5.1 配方+种子+拖尾配方建池：发射器配方错报其自身错、拖尾配方错 InvalidArg；均返回 nil |

#### 方法（48，按接收者分组）

| 符号 | 说明 |
|------|------|
| `Particle.AgeFrac` / `Particle.Alive` / `Particle.ColorAt` | Age/Life 钳 [0,1]（非正 Life 停 1）/ Age 严格小于 Life 且 Life 正 / 按 AgeFrac 插值起止色（AgeFrac 已钳，NaN 不漏） |
| `ShapeKind.String` | 稳定日志名 |
| `Shape.Validate` | 是否构造器产的合法形状 |
| `Turbulence.Vec` | (seed,ageSec) 处的加速度：Strength 缩放的 sin/cos——同种子回放同漂移、不按 tick 消耗随机数 |
| `EmitterConfig.Validate` | 全配方校验：Max 超帽 OutOfMemory、其余坏 InvalidArg |
| `Emitter.Config` / `Emitter.Alive` / `Emitter.Spawned` / `Emitter.Died` / `Emitter.ChildSpawned` | 配方拷贝 / 活粒子数 / 总出生（速率+孩子）/ 总死亡 / 孩子出生数 |
| `Emitter.AppendParticles` / `Emitter.Particles` / `Emitter.Clear` | 活集追加进 buf 返回扩后切片（传复用底 buf[:0] 免逐帧分配；结果不得保留过下一次 Spawn/Update/Clear）/ 活集新拷贝 / 清空活集并重置累计与计数（配方与生成器序列保留） |
| `Emitter.Spawn` | 立刻生 n 个父代（受剩余预算钳，超额静默丢）；负 n InvalidArg、零 no-op |
| `Emitter.Update` | 推进 dt：积分存量 → 清死者 → 非孩子死亡生孩子 → 按出生率补量；新生儿同 tick 从 0 起步；dt≤0 no-op 返回 0,0；活集永不超 Max |
| `Emitter.ColorOf` / `Emitter.AppendToBatch` | 按粒子年龄占比插值发射器色 ramp / 逐活粒子追加一个居中精灵（Src 调用方图集块、Opacity 当前 alpha；alpha≤0 跳过省一次画、非有限位置跳过防爆包毒染；坏参 InvalidArg 不追加） |
| `GPUPool.Config` / `GPUPool.TrailConfig` / `GPUPool.Alive` / `GPUPool.Spawned` / `GPUPool.Died` / `GPUPool.ChildSpawned` / `GPUPool.Trails` / `GPUPool.TotalPoints` | 发射器配方拷贝 / 拖尾配方拷贝 / 活粒子数 / 总出生 / 总死亡 / 孩子出生数 / 活拖尾数（一粒子一尾，Seed 碰撞共用）/ 全部轨迹存点数 |
| `GPUPool.AppendParticles` / `GPUPool.Particles` / `GPUPool.ColorOf` / `GPUPool.TrailOf` | 同 Emitter 语义追加活集 / 活集新拷贝 / 按年龄占比插值色 / 按 seed 取轨迹新拷贝（无活尾 nil） |
| `GPUPool.Clear` / `GPUPool.Spawn` / `GPUPool.Update` | 清全部粒子与轨迹（配方保留）/ 立即生 n 个父代（语义同 Emitter）/ 按 5.1 语义推进后再给每个活粒子记一个轨迹点并清死轨（dt≤0 静默 0,0） |
| `GPUPool.AppendToBatch` | 按存点逐个追加方块精灵（WidthAt 尺寸、ColorAt alpha，跳过规则同 Emitter；坏参 InvalidArg 不追加） |
| `JointKind.String` | 稳定日志名 |
| `TrailConfig.Validate` | 是否构造器产的合法配方 |
| `Trail.Config` / `Trail.Len` / `Trail.Points` / `Trail.AppendPoints` / `Trail.Clear` | 配方拷贝 / 存点数 / 点序新拷贝（老到新）/ 存点追加进 buf（老到新；复用底免分配；结果不得保留过下一次 Push/Clear）/ 清空点（配方保留） |
| `Trail.Push` | 追加 p 为最新点（超 MaxPoints 丢最老）；非有限 p InvalidArg 不落；nil InvalidArg |
| `Trail.WidthAt` / `Trail.ColorAt` | 原始下标 i（最老 0）处的带宽 / ramp 色（头 Start 尾 End）；空尾与坏下标 InvalidArg |
| `Trail.Verts` | 带状展开成顶点对（老到新）：空尾 nil nil（可跳过）；Miter 到限切 bevel、bevel 一对、round 扇内部对；孤点朝 +X 停放；重复邻居复用邻居方向——退化轨迹不出 NaN |
| `Trail.Segments` | 中线段列表（老到新）；空与单点尾 nil nil（无可画） |
| `Trail.AppendToBatch` | 逐点追加方块精灵（WidthAt 尺寸、ColorAt alpha；零 alpha 与非有限跳过、坏参 InvalidArg 不追加） |

## 14. `engine/physics` — 碰撞数学（88 条：顶层 46 + 方法 42）

> 冻结 CPU 侧碰撞数学（2026-09-15 冻结口子，10-05 S86 加宽相索引，只做加法）。**只算数、不画不步进**：调用方移动物体（赋 Pos 或重建）、调 Query、再解实心碰撞并触发传感器。形状两种：盒=中心+半尺寸、圆=中心+半径；非有限/负尺寸 InvalidArg 不落库。层掩码对齐 Godot collision_layer/mask：Layer 是自己在哪组、Mask 扫哪些组，`CanCollide=双方互扫`；掩码不匹配的重叠对在 Query 里永不相遇。Trigger=true 是传感器（金币/门区/跳探针）：Query 照报该对但调用方不得把实心推出传感器。重叠规则包含边界（边接触也算重叠，站平台边不掉），无可回放不稳定的 epsilon。**射线**（14.1b 跳跃探针与子弹线）：单侧掩码（body.Layer&ray.Mask，与 Query 的双向不同）、盒 slab 检验/圆二次检验、边接触与切线算命中、起点在内部报 Dist 0 零法线、同距保小下标。**平台载客**（14.1b）：移动平台步进 delta 只在移动前的站立姿态下带给骑手。**斜坡**（14.2）：细线段 A-B（+Y 向下同 tilemap）、单向/实心两态、GroundYAt 插值、坡角、下滑方向、脚底换算、Grounded/Step 落地与顶头；tilemap 以格顶边喂入，本包不 import tilemap。**吸附与地板**（Godot CharacterBody2D 同念）：GROUNDED/FLOATING 两档、地板法线、SnapGrounded/IsOnFloor/StepWithSnap（刻意差异已记录：吸附无「曾在地板上」前置）。**宽相**（S86 大场索引）：均匀网格 + 静眠（displacement<eps 连续 N 帧）+ 候选上限 + 单区域查询；Query 与全量 Query 同对同序，稳帧缓存对表只付窄相。挂未导出排序类型的 6 个导出方法名不属公开 API（见 §0 注）。

#### 常量（12）

| 符号 | 说明 |
|------|------|
| `ShapeBox` / `ShapeCircle` | 物体几何：中心+半尺寸的盒 / 中心+半径的圆 |
| `DefaultBroadphaseCell` / `MaxBroadphaseBodies` / `DefaultMaxCandidates` | 宽相网格间距 128 世界单位（64px 盒落 1-4 格）/ 单索引容量 8192（大场 1200；超出 Track 报 OutOfMemory 绝不半收录）/ 每次 Query 配对评估上限 40（0=不限，128px 堆叠有余量） |
| `DefaultSleepEps` / `DefaultSleepFrames` | 静眠阈值：单次 Update 位移低于 0.5 算静止 / 连续静止 60 帧后入睡 |
| `MotionModeGrounded` / `MotionModeFloating` | 运动档位（Godot motion_mode 同念）：贴地（平台跳，吸附窗内粘可行走地板）/ 漂浮（俯视漂移，永不报地板不吸附） |
| `DefaultSnapLength` / `DefaultFloorMaxAngle` / `DefaultSafeMargin` | 吸附窗默认 0.1（跨过地板上方小缝）/ 可站地板最大坡角 45°（更陡不是地板）/ 允许下沉量 0.08 |

#### 类型（10）

| 符号 | 说明 |
|------|------|
| `Shape` | 物体几何枚举，见常量行 |
| `Body` | 一个碰撞体：形状 + 中心 + Layer/Mask 位掩码 + Trigger 标记；Name 只是测试与日志的调试键（可空，不是资产 id）；Half 归盒、Radius 归圆，未用侧保持零 |
| `Contact` | Query 的一对相触：输入下标 + 调试名 + Trigger 标记（任一侧是传感器即 true；下标区分重名，名字永不解析） |
| `Ray` | 一条跳跃探针/子弹线：起点 + 方向（不必单位化，长度不影响命中距离）+ 射程 + 单侧掩码 |
| `Hit` | 射线命中的最近物体：输入下标、名、沿归一化方向距 Origin 的距离（世界单位）、命中点、外向面法线、Trigger 标记 |
| `Spring` | 一个跳板：Impulse 额外上抛（+Y 向下，所以起跳是负值）、Damp 衰减（保留下落速度比例，0 死垫 1 完美弹） |
| `Broadphase` | 大场索引：跟踪体的均匀网格 + 静眠 + 候选上限 + 区域查询；Track/Move 是摆放真相，脏时每次 Query 从存量体积重建、永不漂移 |
| `Slope` | 一条细行走面：线段 A-B + 单向标记；+Y 向下（tilemap CellToWorld 单位）；单向坡只从上方落、实心坡从下方穿过还报顶头；上下面同一 Y，Side 由 Step 按运动方向判 |
| `MotionMode` | 运动档位枚举，见常量行 |
| `SnapConfig` | 调用方自持的地板调参：档位 + 吸附长 + 地板角 + 安全下沉；零值无效，从 DefaultSnapConfig 改起 |

#### 函数（24）

| 符号 | 说明 |
|------|------|
| `NewBox` / `NewCircle` | 建 Box 体（half 负或非有限、pos 非有限 InvalidArg；Layer/Mask 任意位，0 什么都不碰）/ 建 Circle 体（半径同理） |
| `Bounds` | 物体的轴对齐包围盒；坏体 ok=false；零尺寸体报空矩形但 ok=true（空≠无效） |
| `CanCollide` | 双方互扫的纯位运算；与有效性无关；Query 用它与 Overlaps 联判 |
| `Overlaps` | 是否相触或穿插（含边接触）；坏体永不重叠（false 不 panic） |
| `Query` | 返回掩码匹配的全部相触对（CanCollide 且 Overlaps 都真）：按输入序 i<j、不改输入、结果新切片（无触 nil）；任一坏体 InvalidArg nil 结果 |
| `NewRay` | 建射线：Origin 有限、Dir 有限非零、MaxDist 有限 ≥0；Mask 任意位（0 不命中） |
| `CastRay` | 返回射线命中的最近体：掩码单侧过滤、同距保小下标、空输入不命中；坏射线或任一坏体 InvalidArg |
| `IsStandingOn` / `CarryRider` | rider 是否站在 platform 顶上（双有效、rider 中心不低于平台中心、rider 底距平台顶 1e-9 内、横向区间重叠含边；圆用包围盒足迹）/ 站立时把平台步进 delta 施加到 rider.Pos（只在移动前的站立姿态；不站 false nil 且不动 rider；nil/坏体/非有限 delta/推越有限域 InvalidArg 且 rider 不动） |
| `NewSpring` | 建跳板：Impulse 有限 ≥0、Damp [0,1]，否则 InvalidArg |
| `NewBroadphase` | 建大场索引（128px 格；自动静眠默认开 0.5 单位/60 帧；AutoWake 默认开——被碰的睡眠者立即归队；候选帽 40） |
| `NewSlope` / `NewOneWay` | 建实心坡 / 建单向坡（任意角度皆收，常规是水平；落点同实心坡，上行穿过永不报顶头）；端点须有限且不同，否则 InvalidArg |
| `FeetOf` / `WithFeet` | 物体中心→脚底点（盒中心+半高、圆中心+半径）/ 脚底点→物体拷贝（Pos.X 随脚底列、Pos.Y 回退半高或半径）；只读不改输入 |
| `Grounded` | 脚底列处表面距脚 1e-9 内的最高坡（坏坡软跳过、同高保先输入；单向与实心在此同等——无方向触碰，方向由 Step 判） |
| `Step` | 脚按 vel·dt 移动并解一个细线段接触：下落或静止取新旧列间最高被穿表面吸附并报 grounded；上行时沿同坡仍 grounded、从下方穿实心坡报 hitHead 且脚钳到天花板、单向坡永不报头；dt==0 只采样 Grounded；坏坡 InvalidArg（与 Query 同 fail-closed）、坏脚/速/dt InvalidArg 脚不动；沿坡行走需沿坡速度（或下一步重力），纯横向撞坡侧不自动吸附 |
| `UpDirection` / `DefaultSnapConfig` | 世界向上单位 (0,−1)（Godot up_direction 默认同值）/ Godot D18 默认参数的贴地档 SnapConfig |
| `FloorNormal` | 坡的朝上单位法线（Y≤0）：平坡 (0,−1)、斜坡向低侧倾（SlideDir 反直立）；竖直段无上法线 ok=false |
| `SnapGrounded` / `IsOnFloor` | 脚处「够得着」的最高可行走坡（上方至多 SnapLength 补缝、下方至多 SafeMargin 容沉；陡面不算——坡能站墙不能；漂浮档无地板）/ 是否站在 cfg 下的可行走地板上（只取地板位） |
| `StepWithSnap` | Step 移动后套 Godot apply_floor_snap 窗：可行走面在下落/静止体的吸附窗内则吸附到面并报 onFloor；上行体永不吸附（跳过单向、实心仍经 Step 顶头）；Step 落在不可行走面上只触碰不算地板；漂浮档运动委托 Step 且 onFloor 恒 false；dt==0 只采样 SnapGrounded；刻意差异：吸附窗内有可行走面即吸附，无「曾在地板」前置 |

#### 方法（42，按接收者分组）

| 符号 | 说明 |
|------|------|
| `Body.Valid` | 数字有限、形状已知、尺寸合法；直写结构体绕过构造器，所以每个查询先查这个并 fail-closed |
| `Ray.Valid` | 射线可发射判定；Mask 不影响有效性 |
| `Spring.Valid` / `Spring.Bounce` | 是否可用跳板 / 触发：下落体（vy>0）以 −(vy·Damp+Impulse) 离开；上升或静止原样通过（只顶面起效，单向坡同精神）；坏垫不触发 vy 原样返回 |
| `Broadphase.Cell` / `Broadphase.MaxCandidates` / `Broadphase.SetMaxCandidates` | 网格间距 / 评估帽（0=不限）/ 调帽（超 n 的 Query 保 track 序前 n 个、其余计 Trimmed；负数 InvalidArg 保旧） |
| `Broadphase.SetAutoWake` / `Broadphase.AutoWake` / `Broadphase.SetAutoSleep` / `Broadphase.SetSleep` | 切接触唤醒（开=Query 返回前唤醒被碰睡眠者；关=只出现在 Woke 由调用方唤醒）/ 读开关 / 切静眠（关=只有显式 Sleep 能停）/ 调静眠（eps 有限 ≥0、frames ≥1，否则 InvalidArg 保旧） |
| `Broadphase.Track` / `Broadphase.Move` / `Broadphase.Remove` | 登记物体返回稳定句柄（坏体 InvalidArg、超帽 OutOfMemory；static 永不自眠；句柄跨 Remove 稳定——槽位走空闲表回收不交换）/ 改写 h 的体积并唤醒（重定位的体必归队；未知句柄 NotFound、坏体不动）/ 遗忘 h（槽位回收） |
| `Broadphase.SetStatic` / `Broadphase.Sleep` / `Broadphase.Wake` / `Broadphase.IsActive` / `Broadphase.Count` / `Broadphase.SleepingCount` | 翻 static 标记 / 停一个动态（static 拒绝 InvalidArg——静态不眠只是不动）/ 唤醒 h（顺带重置静眠基线，刚醒的体第一帧 Update 不停）/ 是否在跟踪且醒着 / 活体数 / 入睡动态数 |
| `Broadphase.Update` | 不查询地推进静眠：上次 Update 后位移低于 eps 的动态加一帧静止、到调定帧数停住；动过的重计；static 永不停 |
| `Broadphase.Query` | track 序返回全部掩码匹配的相触对（网格初筛；无睡眠无帽时与全量 Query 同对同序）：对表干净期缓存（Godot update() 语义）——Track/Move/Remove/SetStatic 标脏重建一次、稳帧复用对表只付窄相；评估数计每个测试的网格对、帽只裁 contacts 不裁评估（Trimmed+contacts 与对数恒对账）；睡眠体保持可撞并在被醒着的非静态碰到时进 Woke（AutoWake 开则顺带唤醒；静态静止与睡眠者互靠不唤醒） |
| `Broadphase.Woke` / `Broadphase.Evaluations` / `Broadphase.Trimmed` / `Broadphase.Queries` / `Broadphase.LastQueryUs` | 上次 Query 被醒体碰到的睡眠句柄（首触序、新切片）/ 上次 Query 的配对测试数 / 被帽裁掉的对数 / Query 累计调用 / 上次 Query 微秒成本（仅参考；门禁由测试与窗量） |
| `Broadphase.QueryRegion` | 返回包围盒触到 rect 的句柄（升序）：Godot cull_aabb 单查询，只走被触格——一次视口查询微秒级而非扫全场；空/非有限矩形 InvalidArg、干净未中 nil |
| `Broadphase.Ray` / `Broadphase.Reset` | 过网格射线：包围盒错过线段盒的体跳过、其余跑与 CastRay 相同的窄相与最近+最小下标规则；睡眠体照样挡射线 / 清空全部跟踪与计数（格距/帽/静眠调参/唤醒开关保留，换场同帧重登记） |
| `Slope.Name` / `Slope.A` / `Slope.B` / `Slope.OneWay` / `Slope.Valid` | 调试名（永不解析）/ 端点 A / 端点 B / 是否单向 / 端点有限且不同 |
| `Slope.GroundYAt` / `Slope.Angle` / `Slope.Walkable` / `Slope.SlideDir` | 列 x 处的表面 Y（竖直段不是 x 的函数 ok=false、端点含在界内）/ 距水平的坡角 [0,π/2] / 是否缓到可站（angle≤maxAngle 精确比较；maxAngle 归调用方）/ 下坡单位向量（平坡无下坡 ok=false、竖直段 (0,1)） |
| `MotionMode.Valid` / `MotionMode.String` | 是否两档之一 / 调试键 |
| `SnapConfig.Valid` | 档位已知且调参有限非负（零吸附合法=只认精确接触；负数不行） |

## 15. `engine/pool` — 数字缓冲池（18 条：顶层 8 + 方法 10）

> 跨帧复用 core 数字缓冲（2026-09-15 冻结口子；2026-09-30 E03 从 engine/step 搬入，step 只留固定步与时间）：Vec2 给路径点与顶点位置、Color 给顶点颜色、float64 给原始坐标流。**Put 交出所有权**——调用方 Put 之后不得再用该切片；Get 返回长度 n 的未定义内容——绘制前必须覆写前 n 个元素。nil Pool 不 panic：Get 报 InvalidArg、Put/ResetStats no-op、Stats 零值。

#### 常量（5）

| 符号 | 说明 |
|------|------|
| `MaxLen` | 单次 Get 请求元素上限（1M，按类各计） |
| `MaxRetained` | 每类保留切片数上限（vec/color/float 各 32） |
| `VecBytes` / `ColorBytes` / `FloatBytes` | 每元素保留记账宽度：Vec2 16 字节 / Color 32 字节 / float64 8 字节 |

#### 类型（2）

| 符号 | 说明 |
|------|------|
| `Stats` | 池计数器：Get/Put 调用数（nil 与零容量 Put 是 no-op 两边都不计）、命中/未命中数、当前各类保留数与保留字节 |
| `Pool` | 三类缓冲的复用池（互斥锁保护）；nil 安全 |

#### 函数（1）

| 符号 | 说明 |
|------|------|
| `NewPool` | 建空池 |

#### 方法（10，均在 `Pool` 上）

| 符号 | 说明 |
|------|------|
| `Pool.GetVec` / `Pool.PutVec` | 取 n 个 Vec2（路径/顶点位置；n<0 InvalidArg、n>MaxLen OutOfMemory、n=0 空切片不碰保留集）/ 归还（nil 与零容量 no-op；超尺寸或超预算的归还计数但丢弃缓冲） |
| `Pool.GetColor` / `Pool.PutColor` | 取 n 个 Color（顶点颜色）/ 归还；规则同上 |
| `Pool.GetFloat` / `Pool.PutFloat` | 取 n 个 float64（原始坐标流）/ 归还；规则同上 |
| `Pool.Stats` / `Pool.RetainedBytes` / `Pool.HitRate` / `Pool.ResetStats` | 计数器拷贝 / 保留底层字节（cap×冻结宽度）/ 命中率 hits/gets（无 Get 返 0）/ 清计数不丢缓冲 |

## 16. `engine/preview` — 预览帧记录与拆账（10 条：顶层 10 + 方法 0）

> 所见即所得预览帧记录 + 逐节点预算拆账（关卡/粒子预览工具用）。刻意自包含（只用 stdlib）：调用方交来一帧的像素 + 节点表，`Record` 落盘 JSON（节点表）+ PNG（像素）到目标目录；`Breakdown` 按节点拆成本/像素占比，报告能点名到节点，超预算节点带 OverBudget（红标）。

#### 常量（2）

| 符号 | 说明 |
|------|------|
| `MaxNameLen` | 帧名与节点名保留长度上限（64） |
| `MaxPixels` | 单帧像素上限（3200 万 RGBA），坏尺寸快速失败 |

#### 类型（3）

| 符号 | 说明 |
|------|------|
| `Node` | 帧内一个成本/像素贡献者：名字、耗时 ms、像素数 |
| `Frame` | 一张预览帧：RGBA 像素 + 节点表；像素不进 JSON（PNG 文件承载），节点表可序列化 |
| `Row` | 一节点在帧中的份额：耗时与占比、像素占比、超预算（红）标记；导出字段供报告直接序列化 |

#### 函数（5）

| 符号 | 说明 |
|------|------|
| `NewNode` | 建节点：空名/超长名/负耗时/负像素报错不落 |
| `NewFrame` | 建帧：坏尺寸、像素长度不符、坏节点报错不落 |
| `Record` | 落一帧为 `<dir>/<name>.json` + `<dir>/<name>.png`（按需建目录），返回两个路径；空 dir/帧名报错 |
| `Breakdown` | 按节点拆帧（按耗时降序、名破平局——下标 0 即吃帧大户）：CostShare 除以节点总耗时、PixelShare 除以节点总像素（空为 0）；超 budgetMs 的节点带 OverBudget（budgetMs≤0 关闭） |
| `Text` | 渲染人工预览文本：每节点一行（耗时、占比、预算判定）；超预算行红包裹并以 OVER 结尾，肉眼（与测试）一眼落到节点名 |

## 17. `engine/quality` — 画质分档（21 条：顶层 14 + 方法 7）

> 冻结画质分档数学（2026-09-16 冻结口子；2026-09-30 E02 从 engine/save 搬入，save 只留槽位/版本迁移）。三档冻结：**high**=1000 粒子+8 灯+1.00 渲染比；**medium**=500+4+0.75；**low**=200+2+0.50（高保真全量 / 中档粒子灯减半、四分之三分辨率 / 低档五分之一粒子、四分之一灯、半分辨率）。数字住在 `engine/quality/testdata/quality_cases.json`，代码只回放——调档先改那个文件。调用方经 New/Parse 建 Quality、读冻结 Spec、经 Switch 换档：换档只换 level，场景继续跑、不重载不闪屏。按设计才解析：只认 version "1.0"；构造器路（New/ParseLevel）坏档报 InvalidArg、文件 Parse 同样坏档报 BadData；nil 接收器报 InvalidArg 不动。文件未知字段忽略（新 minor 文件仍解出档位）、缺 quality 字段 BadData 绝不猜档。

#### 变量（1）

| 符号 | 说明 |
|------|------|
| `QualityVersion` | 画质文件版本（1.0）：加字段升 Minor、破坏形状升 Major |

#### 常量（4）

| 符号 | 说明 |
|------|------|
| `MaxQualityBytes` | 单画质文件字节上限（4096）：形好但超预算报 OutOfMemory，绝不猜装载 |
| `LevelHigh` / `LevelMedium` / `LevelLow` | 三档冻结名（high/medium/low）；解析大小写敏感（"HIGH" 拒收——手滑不许静默落到别的档） |

#### 类型（3）

| 符号 | 说明 |
|------|------|
| `Level` | 档位（string 类型），见常量行 |
| `Spec` | 冻结的每档预算：活跃粒子上限 `Particles`、参与计算的灯上限 `Lights`、渲染分辨率比 `Scale`；字段全是值，拷贝不别名 |
| `Quality` | 当前档句柄：一个 level + 其冻结 spec；零值无档，New/Parse 是唯一入口；有效句柄读永不失败、nil 写报 InvalidArg 不动 |

#### 函数（6）

| 符号 | 说明 |
|------|------|
| `ParseLevel` | 档名 → Level；空名与未知名 InvalidArg |
| `SpecFor` | 档位的冻结 Spec；未知档 InvalidArg |
| `NewQuality` | 建档位句柄；未知档 InvalidArg |
| `ParseQuality` | 校验文件字节成 Quality：空输入 InvalidArg、超尺寸 OutOfMemory、坏 JSON 与坏档 BadData、外来或更新版本 VersionMismatch |
| `StoreQuality` | 写盘（自动建父目录）：空路径 InvalidArg、OS 失败 NotFound 带原因、档错同 Encode |
| `LoadQuality` | 读盘：空路径 InvalidArg、缺文件 NotFound；其余同 ParseQuality |

#### 方法（7，按接收者分组）

| 符号 | 说明 |
|------|------|
| `Level.String` / `Level.Valid` | 档名 / 是否三档之一 |
| `Quality.Level` / `Quality.Spec` / `Quality.Equal` | 当前档（零句柄 ""）/ 冻结 Spec（零句柄零 Spec；有效句柄不失败）/ 是否同档 |
| `Quality.Switch` / `Quality.Encode` | 换档（其他一切不动：场景按新 Spec 继续跑；换到当前档 no-op；未知档 InvalidArg 不动）/ 渲染规范文件字节（零句柄 InvalidArg、超预算 OutOfMemory；不改输入） |

## 18. `engine/renderconv` — core↔render 转换口（10 条：顶层 10 + 方法 0）

> 唯一的 core↔render 类型转换口（2026-09-30 E01 冻结，从 engine/core 的 vec.go/color.go 原文搬出，core 保持零外部 import）。规则：**逐字段拷贝、不舍入、不钳位、不给默认**；游戏代码在碰 render 类型前在此转换一次；数字冻结、边界往返无损。tex.Filter 的转换永不归一化、永不发明默认：未设轴与带不动的开关以命名错误返回，Mip 原样通过（零值保持历史单级路 render.MipmapNone）；画面证据：④ p37_mip_exit F 臂与 ⑥ p37_scene E 面板经此转换驱动真实绘制。

#### 变量（3）

| 符号 | 说明 |
|------|------|
| `ErrFilterMixedAxes` | Near≠Far 的 tex.Filter：render 的放大/缩小共用一个 filter，该组合没有无损形态 |
| `ErrFilterUnknownKind` | tex.Filter 有未设（Unknown）轴：「没选」映射不到任何东西；先经 tex.Filter.Normalized 归一再转换 |
| `ErrFilterAnisoUnsupported` | MaxAniso>1：render 精灵/图像路没有各向异性字段，开关会被静默丢掉 |

#### 函数（7）

| 符号 | 说明 |
|------|------|
| `Vec2ToRenderPoint` / `Vec2FromRenderPoint` | 边界处 Vec2→render.Point / 反向 |
| `ColorToRender` / `ColorFromRender` | 边界处 Color→render.RGBA / 反向 |
| `Mat2DToRenderMatrix` / `Mat2DFromRenderMatrix` | 边界处 Mat2D→render.Matrix / 反向 |
| `FilterToRender` | tex.Filter → render 采样对（InterpolationMode + MipmapMode）：见包注规则；命名错误见变量行 |

## 19. `engine/save` — 进度存取（35 条：顶层 17 + 方法 18）

> 冻结文件侧进度数学（2026-09-15 冻结口子，只做加法）。**不画不玩，只在两次运行之间带数字**：调用方用 New + SetProgress/AddItem/SetStars 建 Save、Store 落盘、Load 读回、Migrate 升老档。文件为自述 JSON（冻结形状第一版）：version "1.0" + slot + progress{level,checkpoint,play_ms} + inventory[{id,count}] + stars{level: 0..3}。未知字段忽略（新 minor 仍可解析）；缺 progress/inventory/stars 默认空（无星的老档照样载，Migrate 补齐）。按设计才解析：可装载的旧版（legacy 0.x 与 CompatibleWith minor）能解析以便 Migrate；外来 major 与更新 minor 报 VersionMismatch 绝不猜装载。槽位 0..MaxSlots−1（3 槽）；progress 的 level/checkpoint 是纯名字（空=新局）、play_ms 整数毫秒；背包按插入序、重复 id 不落库；星 0 表示删除条目。构造器坏值报 InvalidArg、Parse 同样坏形状报 BadData。

#### 变量（1）

| 符号 | 说明 |
|------|------|
| `CurrentVersion` | 引擎存档版本（1.0）：每次 Store 写入；加字段升 Minor、破坏形状升 Major |

#### 常量（7）

| 符号 | 说明 |
|------|------|
| `MaxSlots` | 槽位账上限（3，槽号 0..2） |
| `MaxInventory` | 单存档背包条目上限（256） |
| `MaxItemCount` | 单堆叠上限（计数 1..9999） |
| `MaxStars` | 单存档星级关卡数上限（64） |
| `MaxStarsPerLevel` | 单关星数上限（0..3，0 删除） |
| `MaxNameLen` | 关卡/检查点/物品/星键名长度上限（64 字节） |
| `MaxBytes` | 单存档文件字节上限（1MB） |

#### 类型（3）

| 符号 | 说明 |
|------|------|
| `Item` | 一条背包：id + 数量 1..MaxItemCount；数量永不为负 |
| `Progress` | 一份进度快照：关卡名、检查点名、游玩时长（整数毫秒，慢机快机一致）；空名=新局 |
| `Save` | 一份槽位档：槽号 + 版本 + 进度 + 背包 + 关卡星；字段私有、写入全过校验、读取全是拷贝 |

#### 函数（6）

| 符号 | 说明 |
|------|------|
| `NewItem` | 建背包条目：空/超长 id、计数出 1..MaxItemCount 报 InvalidArg 不落 |
| `NewProgress` | 建进度：超长名、负时长 InvalidArg 不落；空名合法（=新局） |
| `New` | 在 slot 上建带 CurrentVersion 的空档；槽号出界 InvalidArg |
| `Parse` | 校验文件字节成 Save：空输入 InvalidArg、超尺寸 OutOfMemory、坏 JSON/坏形状 BadData、更新或外来版本 VersionMismatch；legacy 0.x 照常解析以便 Migrate |
| `Load` | 读盘：空路径 InvalidArg、缺文件 NotFound；其余同 Parse |
| `Migrate` | 升到 CurrentVersion（保留槽号/进度/背包/星）：当前版本返回拷贝、可装载旧版升上来；外来 major/更新 minor VersionMismatch、坏形状 InvalidArg、超预算 OutOfMemory；不改输入 |

#### 方法（18，按接收者分组）

| 符号 | 说明 |
|------|------|
| `Item.ID` / `Item.Count` | 物品键 / 堆叠数 |
| `Progress.Level` / `Progress.Checkpoint` / `Progress.Play` / `Progress.PlayMs` | 关卡名（空=新局）/ 检查点（空=无）/ 游玩时长 / 游玩毫秒数 |
| `Save.Slot` / `Save.Version` / `Save.Progress` | 槽号 / 版本 / 进度快照 |
| `Save.Inventory` / `Save.Stars` / `Save.StarsOf` | 背包拷贝（按存储序；改结果动不了档）/ 星表拷贝（nil/空报空非 nil map）/ 单关星数（缺关 ok=false） |
| `Save.SetProgress` / `Save.AddItem` / `Save.SetStars` | 整体替换进度（坏名/负时长 InvalidArg 不动）/ 加物品（已有 id 合并；新 id 超 MaxInventory OutOfMemory；坏 id/坏计数/合并超 MaxItemCount InvalidArg；失败不动）/ 记星（0 删条目；坏关名与星出 0..3 InvalidArg；新关超 MaxStars OutOfMemory） |
| `Save.Equal` | 槽号、版本、进度、背包序、星全同才相等（nil 与空背包/星表视为相等） |
| `Save.Encode` / `Save.Store` | 渲染规范文件字节（档必须已是 CurrentVersion——legacy 先 Migrate，否则 VersionMismatch；坏形状 InvalidArg、超预算 OutOfMemory；不改输入）/ 写盘（自动建父目录；空路径 InvalidArg、OS 失败 NotFound 带原因；形状错同 Encode） |

## 20. `engine/sprite` — 精灵排序与批渲染（63 条：顶层 42 + 方法 21）

> 冻结 CPU 侧精灵排序（2026-09-15 冻结口子，只做加法）。**盖序规则**：先层升序（world 0 人物树木地面 < fx 1 火烟拖尾 < ui 2 按钮提示；大层恒盖小层——特效永远飘不到界面上，自定义层是普通整数规则不变），再脚底 Y 升序（同层同 Y 保输入序稳定，同高不闪）；2026-11-04 P3.7 批 2 步 2b 起**判定委托给 camera**：带位映射到 camera 的 z 桶（`ZIndexForLayer` 占偶数槽、奇数槽留给影子），`camera.YSortLess` 裁决（容差判等下平局改按输入位置定，不再看浮点末位），testdata/p37_zsort_parity.json 实测与旧序逐位一致。**深度排序**（DeepItem）：Depth 是本仓扩展轴（Godot 无 2D 距离键）——大=远=先画（远到近画家算法），同深度内层/脚 Y 沿 2.3 规则。**逐帧动画**（Flipbook）：动作库（idle/run/jump 各一 Clip）+ 播放头 + 事件（越帧/回绕/播完）。**批渲染**（Batch）：同图集一次提交——Flush 按图首次出现序分组、每图 emit 一次，削减 draw call。**AtlasSprite**：图集子块 + 旋转/染色/翻转/轴心 + 逐精灵采样（Filter 并入 render.InterpolationMode、Mipmap 并入 render.MipmapMode，零值走老路逐位不变）+ P3.7 透视腿（Perspective/Corners/Weights——投影引擎算好、render 只收顶点级数据，同 Godot Sprite3D 分工）。只算数不画，结果逐字段喂既有 render DrawAtlas/DrawAtlasEx。

#### 常量（11）

| 符号 | 说明 |
|------|------|
| `LayerWorld` / `LayerFX` / `LayerUI` | 绘制带 0/1/2：人物树木地面（先画在后）/ 火烟拖尾（盖世界）/ 按钮提示（最后画在最顶，永不被盖） |
| `FilterUnset` | 采样质量零值：render 归一成 bilinear 老路 |
| `LoopOnce` / `LoopLoop` / `LoopPingPong` | 逐帧动画循环模式：播完定格停止 / 回第一帧继续 / 两端往返（0 1 2 3 2 1 0…） |
| `EventNone` / `EventFrame` / `EventLoop` / `EventFinished` | Update 越过的事件类型：可见帧未变 / 帧前进（未回绕未播完）/ 循环回绕或往返折返 / once 片到尾停住 |

#### 类型（11）

| 符号 | 说明 |
|------|------|
| `Sprite` | 大图的一个子块：图集 id、图像像素内的源块、世界单位的目标框、透明度；Tag 只是测试日志调试键 |
| `AtlasSprite` | 带旋转/染色/翻转/轴心/逐精灵采样的图集子块：Rot 弧度绕轴心、Flip 先镜像后旋转、Pivot 是 Dst 原点起的偏移（零=左上）、Tint 零结构=不染否则直乘含 alpha、Tag 仅调试；**Perspective**（P3.7 批 1c）切到投影腿：Corners 装投影后屏幕角（TL/TR/BR/BL，目标画布用户空间——相机口填好）、Weights 装逐角齐次 w（=focal+depth），此时 Dst/Rot/Flip/Pivot 全忽略、不许半仿射半透视混搭；零值保持仿射路 |
| `Batch` | 精灵攒批器：按图分组冲刷，同图集一次提交削减 draw call；非并发安全 |
| `DeepItem` | 深度可排序精灵：Depth 大=远=先画（R5/engine/camera 约定）；Layer/FeetY 在同深度内按 camera 规则破平局 |
| `LoopMode` | 动画循环模式枚举 |
| `Clip` | 一个命名动作：帧图 id 播放序表 + 每帧时长 + 循环模式 |
| `EventKind` | 事件类型枚举 |
| `Event` | 一次 Update 发生的事：哪个片、哪个图 id、片内位置、哪种越过 |
| `Flipbook` | 逐帧动画播放器：Play 挑动作、Update 按真实时间推进、OnEvent 报越过；只回图 id，画归调用方 |
| `Layer` | 绘制带（int）：大盖小；自定义带是普通整数，排序规则永不改变 |
| `Item` | 一个可排序精灵：带位 + 世界单位脚底；Name 仅调试键（可空） |

#### 函数（20）

| 符号 | 说明 |
|------|------|
| `FilterName` / `ParseFilter` | 采样质量的稳定日志/数据名 / `""`/`default`/`nearest`/`bilinear`/`bicubic` → 采样质量（其余 InvalidArg 绝不猜；图集 case 数据词汇在并入时原样保留） |
| `MipmapName` / `ParseMipmap` | 层级路的稳定名 / `""`/`none`/`nearest`/`linear` → 层级路；其余报错——未设字段含义是「留在单级路」而不是「挑个更好的」，这正是既有精灵逐位不变的原因 |
| `NewAtlasSprite` | 建 AtlasSprite：图非空、坐标/透明度/旋转/轴心/染色分量有限、Filter 可设；零尺寸矩形合法（后面被绘制跳过，同 R4 skip）；Tag 留空调用方自设 |
| `FeetPivot` | 把旋转中心钉在脚底的轴心偏移（Dst 底边中心，Dst 单位）：当 Pivot 用，转身脚不离地 |
| `AtlasToRender` | 边界处整表转 render 绘制（保序保数无损）：不改输入、结果新切片；空输入 nil nil；第一个坏精灵以其 InvalidArg 中止返回 nil——绝不画一半 |
| `Skippable` / `EffectiveOpacity` / `NewSprite` | Flush 是否跳过不画（同 render DrawAtlas skip：空源或零目标尺寸）/ 透明度归一到绘制实际用的值（≤0 默认 1、>1 钳 1）/ 建 Sprite（图非空、坐标与透明度有限；零尺寸合法后面跳过） |
| `NewBatch` | 建空批 |
| `NewDeepItem` / `SetDepth` / `DepthSort` / `IsDepthSorted` | 建 DeepItem（Depth/FeetY 须有限，NaN/Inf InvalidArg 绝不猜）/ 改深度（NaN/Inf InvalidArg 原值不动）/ 深度绘制序（depth 降序远先画，同深度走 camera 规则；不改输入、结果新切片；任一非有限 depth/feet InvalidArg nil 结果输入不动）/ 是否已是绘制序（空与单项算已排；非有限 fail-closed 报 false） |
| `NewFlipbook` | 建空动作库；片经 AddClip 注册、当前动作经 Play 指定 |
| `ZIndexForLayer` | 绘制带 → camera z 桶轴（Godot 外层键）：带占 camera.YSortZ 分出的同款槽（偶数槽，奇数槽留给该带的影子）；映射单调——「大带盖小带」永不改变；超出 ±camera.YSortZRange 的带饱和进末桶（同 Godot 钳越界 z_index） |
| `NewItem` / `Sort` / `IsSorted` | 建 Item（FeetY 须有限；Layer 收任意整数便于自定义带）/ 绘制序（带升序 → 脚 Y 升序（camera 容差判等）→ 输入位置破平局；带+脚公式只在 camera.YSortLess 一处——这是委托不是拷贝；不改输入、结果新切片按序画；非有限 feet InvalidArg nil 结果）/ 是否已是绘制序（空与单项已排；非有限 fail-closed false） |

#### 方法（21，按接收者分组）

| 符号 | 说明 |
|------|------|
| `AtlasSprite.Validate` / `AtlasSprite.Skippable` / `AtlasSprite.HasTint` | 是否可画（数字有限、filter 已知、图非空；零尺寸矩形在此合法、由 Skippable 与绘制跳过，绝不是错）/ 是否被绘制跳过（同 R4 skip：空源或零目标尺寸；负目标尺寸保留——渲染边缘的镜像画）/ 是否带显式染色（零色=不染即渲染边缘的白不透明；其他任何值含显式白都乘纹素） |
| `AtlasSprite.PivotPoint` / `AtlasSprite.ToRender` | 世界单位绝对轴心（Dst 原点+偏移）：绕脚旋转时此点固定——转身身体脚上粘着 / 边界处逐字段转 render 绘制（坏精灵 InvalidArg 绝不猜；可跳过的照常转换、后面被 DrawAtlasEx 跳过） |
| `Batch.Add` / `Batch.Len` / `Batch.Skipped` / `Batch.Clear` | 存一个精灵（空图/非有限 InvalidArg 不落；零尺寸精灵静默跳过——added=false nil 错永不入库，与绘制跳过完全一致）/ 存量数（跳过的不存）/ 已跳过的零尺寸数 / 清空并重置跳过计数（底层容量保留——复用 Batch 不逐帧重分配；Len 照样 0） |
| `Batch.FlushCount` / `Batch.Flush` | 排空并只返回 draw call 数（不同图数），不分配不调 emit（与 Flush 同口径：空或 nil 0、单图快路 1、否则不同图数）；供 draw-call 门禁用（内容由契约测试证），画仍走 Flush / 按图首次出现序分组、组内保 Add 序、每图 emit 一次、排空批、返回 draw call 数；空批不调返回 0；nil emit 静默 no-op 返回 0 且保批；emit 禁止回调同一 Batch |
| `LoopMode.String` / `EventKind.String` | 稳定日志名 |
| `Flipbook.AddClip` / `Flipbook.Has` / `Flipbook.Count` | 存一个动作（名非空唯一、Frames 至少一个非负图 id、FrameDur 正、Loop 三态之一；坏片 InvalidArg 不落；帧表拷贝防调用方后续改库）/ 动作是否存在 / 动作数 |
| `Flipbook.OnEvent` / `Flipbook.Play` / `Flipbook.Current` / `Flipbook.IsPlaying` / `Flipbook.Update` | 设越过回调（nil 清除；回调在 Update 内同步执行、禁回调同一 Flipbook）/ 挑动作并回卷到首帧（未知 NotFound、空名 InvalidArg；出错不动当前动作与位置）/ 当前动作、位置、图 id（Play 从未成功 ok=false）/ 时间是否推进当前动作（once 播完 false 直到 Play 回卷）/ 推进播放头并返回可见图 id：dt≤0 不动报 EventNone；无当前动作 ok=false 不发事件；大 dt 合并为一个事件（finished > loop > frame） |

## 21. `engine/step` — 固定步与分层时间（27 条：顶层 10 + 方法 17）

> 冻结 CPU 侧固定步 + 分层时间（2026-09-15 冻结口子；池 2026-09-30 E03 搬去 engine/pool、脏区搬去 engine/dirty）。**固定步**（8.1 逻辑节拍 + 11.1 渲染混合一体）：调用方把每个真实帧喂给 `Advance`、跑返回数量的固定逻辑 tick、再用 `Alpha` 混合最后两个逻辑状态出画面。数学是 Fix Your Timestep 经典（Gaffer）：帧先钳 [0,MaxFrame]（负停 0、超长钳 250ms——卡顿绝不螺旋成几百个追帧 tick）；`accum += frame; while accum>=dt: accum-=dt, steps++`；alpha=accum/dt ∈[0,1)；picture=prev+(curr−prev)·alpha。账本是整数毫秒（core.Duration），百万 tick 落在精确毫秒上、慢机快机逐拍一致；Dt 一经定下中途不变。**分层时间**（11.2，Godot time-scale 同念）：一个真实帧拆成缩放后的世界增量与不缩放的 UI 增量；暂停只停世界、UI 照走；坏缩放钳 [0,MaxScale] 绝不报错（NaN/零/负停 0）；scale>1 时世界增量可超 MaxFrame，喂给 Fixed.Advance 会再钳。

#### 常量（2）

| 符号 | 说明 |
|------|------|
| `MaxFrame` | 单次 Advance 输入上限（250ms）：超长帧钳到上限，循环绝不螺旋追帧（随 Gaffer 经典 frameTime=min(0.25)） |
| `MaxScale` | 时间缩放上限 8.0：更坏的滑杆停在帽上（250ms×8 仍有界；Fixed 会再钳喂进来的世界增量） |

#### 类型（2）

| 符号 | 说明 |
|------|------|
| `Fixed` | 一条固定逻辑节拍 + 渲染混合因子：喂帧→跑 tick→用 Alpha 出画面；Dt 中途不变、整数毫秒账本 |
| `Clock` | 分层游戏时间：一帧拆成缩放世界增量 + 不缩放 UI 增量；暂停只停世界 |

#### 函数（6）

| 符号 | 说明 |
|------|------|
| `NewFixed` | 建步宽 dt 的循环：dt 必须为正（零负 InvalidArg 不落） |
| `Interp` / `InterpFloat` / `InterpVec` | prev 到 curr 按 alpha 混合（alpha 先钳 [0,1]、NaN 停在 prev；两端原样透传；prev/curr 按 IEEE-754 无守卫，同 anim.Lerp）/ 显式 float64 版（与 Interp 同，两个名字都冻结让资产文件与代码读法一致）/ 位置版 |
| `NewClock` | 建不暂停、1x、零累计的时钟 |
| `Split` | 无状态地把一帧拆成 (world, ui)：帧先钳 [0,MaxFrame]（与 Fixed 共用）；UI 拿钳后帧、世界拿缩放帧（暂停时 0）；scale>1 时 world 可超 MaxFrame，喂 Fixed.Advance 会再钳 |

#### 方法（17，按接收者分组）

| 符号 | 说明 |
|------|------|
| `Fixed.Dt` / `Fixed.Steps` / `Fixed.Elapsed` / `Fixed.Accum` / `Fixed.Alpha` | 冻结步宽（nil 0）/ Advance 累计产出的 tick 数 / steps×dt=已耗逻辑时间 / 等下一个 tick 的剩余帧时间 / accum/dt ∈[0,1) 的画面混合因子（Advance 保证 accum<dt、Reset 清零，除法天然在域内；nil 与坏循环停 0） |
| `Fixed.Advance` | 折一帧进循环并返回该跑几个固定 tick：负帧停 0、超 MaxFrame 钳到 MaxFrame；nil 循环报 0 不动 |
| `Fixed.Reset` | 清余量与总量（保留 Dt）；nil 不动 |
| `Clock.SetTimeScale` / `Clock.TimeScale` / `Clock.SetPaused` / `Clock.Paused` / `Clock.Pause` / `Clock.Resume` | 存缩放（钳 [0,MaxScale]；nil 不动）/ 读缩放（nil 0）/ 存暂停标记 / 世界是否停 / 暂停世界（UI 照走）/ 恢复世界 |
| `Clock.Advance` / `Clock.WorldElapsed` / `Clock.UIElapsed` / `Clock.Reset` | 拆一帧并把两个增量加进累计（nil 报 0,0 不动）/ 缩放后的世界累计 / 不缩放的 UI 累计 / 清两个累计（保留缩放与暂停） |

## 22. `engine/tex` — 压缩纹理（123 条：顶层 77 + 方法 46）

> 冻结压缩纹理（2026-09-15 冻结口子，10-02 S64 加第二种类，只做加法）。一个容器 + 冻结的块格式：KTX2（Khronos v2 小端头）承载 VK_FORMAT_BC1_RGBA_UNORM_BLOCK（133），supercompression NONE、单 2D 级、faceCount 1、layerCount 0、宽高 4 的倍数；8 字节块原样上传 GPU，CPU 把同样的块解成 RGBA8 供离屏比对。**其余全部预留**：其他 vkFormat、非零 supercompression（BasisLZ/Zstd/Zlib）、多级 mipmap（归 3.2）、数组/立方/3D、未压缩 KTX2——头解析到「认出种类」即报 Unsupported，绝不猜图。ASTC 4x4（S64 第二种，16 字节/块）Go 只携带与校验块、**不随引擎带 ASTC 软解**：CPU 侧像素是平铺品红占位（画面诚实可测、CPU 上显粉色），真颜色来自带驱动钩子的 Transcode。**流送**（3.3）：Stream 把 KTX2+BC1 解码挪到后台（不挡玩法路径），预算镜像 engine/asset 但账本独立（asset 读 tex，tex 不能反向 import）；缺图服务共享的 4×4 品红占位（永不 nil）。**采样开关**（3.2）：一张图一个 Filter（Near 放大 / Far 缩小 / Mip 层间选择 / MaxAniso 1..16），SetFilter/SetMipmap 大声校验（InvalidArg）出错不动接收器；`SamplerParams` 是唯一的 render 边界映射——全 nearest 图像屏蔽 mip/aniso（与 GPU nearest 采样器同款），两张 nearest 图永远可合并；MipMode 别名 render.MipmapMode（P3.6 批 2 D11：真相在 render，tex 不留第二份）。层级数学镜像 CPU box 链。老 PNG/JPG/WebP 8-bit 同步路径原样不动。

#### 常量（31）

| 符号 | 说明 |
|------|------|
| `MaxDimension` / `MaxPixels` | 单边上限 8192 / 面积上限 8192×8192；形好但超预算报 OutOfMemory 绝不猜 |
| `LargeTranscodePixels` | 大图线（256×256 恰在线上）：≥ 此像素数必须走 TranscodeAsync，转码绝不跑在玩法线程 |
| `SchemeNone` / `SchemeBasisLZ` / `SchemeZstd` / `SchemeZlib` | KTX2 supercompressionScheme 值：无压缩 / Basis Universal / Zstd / Zlib（后三者在冻结子集里全部 Unsupported） |
| `FormatUnknown` / `FormatBC1RGBAUnorm` / `FormatASTCRGBA4x4` | 块格式枚举：零值 / KTX2+BC1 RGBA（4×4 块 8 字节）/ KTX2+ASTC 4×4 RGBA（4×4 块 16 字节；ETC2 保持预留） |
| `UsageColor` / `UsageNormal` | 采样用途：基色/漫射（任何冻结格式可采）/ 切线空间法线图（拒收 BC1——最低档块法线必糊） |
| `KindUnknown` / `KindNearest` / `KindLinear` | 单轴滤波：零值（Normalized 映射为 Linear 历史默认）/ 最近邻（近景清晰）/ 线性混合（远景稳定） |
| `MipUnknown` / `MipNearest` / `MipLinear` | 层间选择：零值（=render.MipmapNone——tex 里「未设」≠「无 mip」，Normalized 归一为 MipNearest）/ 只取最近层 / 相邻层混合（三线性） |
| `AnisoMin` / `AnisoMax` / `DefaultAniso` | 各向异性下限 1（关闭）/ 设备帽 16 / 历史采样器行为（关） |
| `RealisticAniso` | S65 写实档默认各向异性 4（45° 地面） |
| `MaxStreams` / `MaxStreamBytes` / `MaxStreamAssetBytes` / `MaxStreamIDLen` | 流送预算：Ready+在途 id 1024 / 保留解码 RGBA8 64MB / 单 KTX2 载荷 8MB / id 文本 128 字节 |
| `StateEmpty` / `StateLoading` / `StateReady` / `StateMissing` / `StateFailed` | 流送生命周期：无记录 / 后台解码在途 / 解码图已缓存 / NotFound 占位 / BadData·Unsupported·OutOfMemory 占位 |

#### 变量（4）

| 符号 | 说明 |
|------|------|
| `CapsDesktopBC1` / `CapsMobileASTC` / `CapsUniversal` / `CapsNone` | 四张显卡能力表：只解 BC1（ASTC 须转码或吃粉）/ 只解 ASTC / 双解 / 都不解（全部压缩图吃粉）；Go 按表路由，像素归驱动 |

#### 类型（13）

| 符号 | 说明 |
|------|------|
| `Format` | 冻结块格式枚举；BC1 与 ASTC 4x4 已建，其余经 ParseKTX2 报 Unsupported |
| `Image` | 一张解码好的 KTX2 图：原样上传 GPU 的块字节 + CPU 解码的 RGBA8 像素（离屏比对用）；BC1 像素从块解出，ASTC 像素是平铺品红代理（真色来自带驱动钩子的 Transcode） |
| `Usage` | 采样用途枚举；法线图要精度，最低档块只许上色 |
| `GPUCaps` | 显卡原生解码能力表（名字 + BC1/ASTC 布尔） |
| `TranscodeTarget` | 单图在单卡上的路由裁决：`TargetNative` 原样上传 / `TargetBC1` 重打包成 BC1（驱动钩子，Go 只校验）/ `TargetPink` 品红占位（绝不猜像素） |
| `Transcoder` | 驱动钩子函数类型：ASTC 块 → BC1 块（同几何 8 字节/块）；nil=此处无驱动，调用方拿到粉图+Unsupported——引擎不带 ASTC 软解，测试或驱动须自带 |
| `Kind` | 单轴滤波枚举 |
| `MipMode` | 层间选择枚举（别名 render.MipmapMode——render 持有真相） |
| `Filter` | 一张图的采样开关：Near/Far/Mip/MaxAniso（1..16）；零值合法（Normalized 映射到 DefaultFilter） |
| `Table` | 逐图滤波开关表：一个 AssetID 一个 Filter（读写锁保护） |
| `State` | 流送生命周期枚举 |
| `Stats` | Stream 计数器：Ready 条数、解码 RGBA8 字节、GPU 块字节、装载/卸载/请求累计 |
| `Stream` | 后台纹理账本：id 经 tex.ParseKTX2 在玩法路径外解码、Poll 永不阻塞、缺图服务品红占位；nil 不 panic；老 PNG/JPG/WebP 同步路原样不动 |

#### 函数（25）

| 符号 | 说明 |
|------|------|
| `ParseKTX2` / `LoadKTX2` | 校验 KTX2 文件图并携带块（BC1 块同时解 RGBA8；ASTC 块原样带粉代理；冻结子集之外 Unsupported/BadData）/ 读路径并解析 |
| `ParseBasis` / `LoadBasis` | 预留 Basis 入口：冻结为 unsupported 让调用方可探测不猜（空输入仍 InvalidArg）/ 读路径解析到「认出缺失」：缺文件 NotFound、存在的文件一律 Unsupported（Basis 解码器后置） |
| `IsPink` | 图是否全品红（转码回退或 ASTC CPU 代理）；nil 永不粉 |
| `CheckUsage` | 拒收会糊的格式/用途组合：BC1 是最低档（4 色 4×4 块）只许上色；ASTC 4x4 是 ASTC 顶档可载法线；未知名大声拒收 |
| `TranscodeTargetFor` | 按能力表路由格式：卡能解则 native、只有 BC1 时 ASTC 回退 BC1、其余回退粉；纯查表不碰像素 |
| `Transcode` | 按能力表路由 im：native 原样返回；BC1 路跑钩子 fn 并用冻结 BC1 解码器解结果——fn 缺失/失败/过短返回同尺寸粉图+原因，绝不 panic 绝不 nil；nil im 返回共享占位 |
| `NeedsAsync` / `TranscodeAsync` | im 是否大图（转码必须走异步而非玩法线程）/ 在调用线程外跑 Transcode，结果经带缓冲 channel 送达一个 TranscodeResult；调用本身永不阻塞，失败的转码照样送粉图 |
| `ParseKind` / `ParseMipMode` | "nearest"/"linear" → Kind（其余 Unknown）/ 同款 → MipMode（其余 Unknown） |
| `DefaultFilter` / `NearestFilter` | 历史 GPU linear 采样器同款：两轴平滑 + nearest 层 + aniso 关 / 历史 GPU nearest 采样器同款：两轴锐利（Mip 与 MaxAniso 随行但被 SamplerParams 屏蔽，与 GPU nearest 路同款） |
| `NumLevels` / `LevelForScale` / `LevelSize` | w×h 图的 mipmap 层数（1+floor(log2(max))；非正边 0 层小图护栏）/ 缩放比 → 层下标（≥1.0 是 0 级、每半一深、钳到末级；NaN/∞/零/负回退 0，镜像 CPU 链钳制使两侧选同层）/ 层尺寸（0 级原图、逐级减半、最小 1；越界 ok=false） |
| `NewTable` | 建空逐图开关表 |
| `RealisticFilter` / `PixelArtFilter` | S65 写实一键默认：mipmap + 三线性 + 4x aniso（render 采样开关仍走老路）/ 像素风预设：Nearest + 整数坐标（位置与 UV 配 SnapPixel/IsPixelAligned） |
| `SnapPixel` / `SnapPixelInt` / `IsPixelAligned` | 取整到最近整数像素中心 / 最近整数像素下标 / 是否落在整数像素上 |
| `PlaceholderImage` / `NewStream` | 共享的 4×4 品红占位（永不 nil；调用方不得改动——访问器都拷贝）/ 建空后台账本 |

#### 方法（46，按接收者分组）

| 符号 | 说明 |
|------|------|
| `Format.String` / `Format.BlockBytes` / `Format.BlockExtent` / `Format.VkFormat` | 稳定日志名 / 每块 GPU 上传字节（BC1 8、ASTC 16、未知 0）/ 块足迹（两种都 4×4）/ KTX2 头携带的 Vulkan 枚举（BC1=133、ASTC=157） |
| `Image.Format` / `Image.Width` / `Image.Height` / `Image.BlockCount` / `Image.UploadSize` / `Image.PixelSize` / `Image.Ratio` | 块格式 / 宽 / 高 / 4×4 块数（按格式：BC1 ÷8、ASTC ÷16）/ GPU 上传字节（块原样）/ 解码 RGBA8 字节（w×h×4）/ 解码字节每上传字节比（BC1 为 8） |
| `Image.Blocks` / `Image.Pixels` / `Image.At` / `Image.Equal` | GPU 块字节拷贝（永不别名图；改拷贝毁不掉回放）/ 解码 RGBA8 拷贝（行主序左上）/ (x,y) 像素 core.Color（越界或 nil ok=false 不 panic）/ 是否逐位同图 |
| `Usage.String` | 稳定日志名 |
| `TranscodeTarget.String` | 稳定日志名 |
| `Kind.String` | 稳定日志名 |
| `Filter.Normalized` / `Filter.SetFilter` / `Filter.SetMipmap` | 全部 Unknown 轴映射到历史默认（Linear 轴、Nearest 层）并把 aniso 钳 [1,16]；永不失败（要大声校验用 Set/SetMipmap）/ 换两轴滤波与 aniso 帽（3.2 冻结接口：近/远逐图、aniso 可调；Unknown Mip 顺带落 MipNearest 历史选择——零值 Filter 一次 SetFilter 即有效；已设 Mip 不动保老调用方的层选；坏输入 InvalidArg 不动）/ 换层间选择（坏输入 InvalidArg 不动） |
| `Filter.SamplerParams` | f → render 边界的有效 GPU 采样器选择（mag/min/mip linear + aniso 帽）：全 nearest 图屏蔽 mip/aniso——两张 nearest 图即使携带的 Mip/MaxAniso 不同也永远可合并 |
| `Filter.AnisoWarning` / `Filter.IsAnisoValid` / `Filter.LogAnisoWarning` | 点名 S65 无效配置（aniso>1 却无 mipmap-linear；或全 nearest 图携带 aniso——会被采样器屏蔽）：空=合法 / aniso 开关是否可用 / 经 std log 发出警告并返回文本（空=合法不记日志） |
| `Table.Set` / `Table.Get` / `Table.Remove` / `Table.Len` / `Table.Clear` | 记录 f（空 id 或坏 filter InvalidArg 不落）/ 取（未知 false）/ 删（报是否曾存）/ 存量数 / 清空 |
| `State.String` | 稳定日志名 |
| `Stream.Request` / `Stream.RequestFile` | 后台解码字节（worker 启动前拷贝数据，调用方可复用；Loading 中重复请求 no-op）/ 后台装载路径（缺文件经 Poll/Wait 以 Missing 呈现，绝不作为 Request 错误） |
| `Stream.Poll` / `Stream.Wait` / `Stream.Get` | 非阻塞查生命周期（未知 id=StateEmpty+NotFound；Loading/Ready nil；Missing/Failed 重放原因）/ 阻塞到 id 离开 Loading 再返回图（图不可变：返回指针恒有效；等待封顶 5 秒，卡死的 worker 拖不垮玩法）/ 非阻塞取 Ready 图（Ready (图,true)；其余 (占位,false)；永不 nil 永不 panic） |
| `Stream.Placeholder` / `Stream.Ready` / `Stream.StateOf` | id 的可用替身（缺纹理永不判 nil；Ready 回真图，否则共享品红）/ 是否已缓存解码图 / 生命周期 |
| `Stream.Ref` / `Stream.Unload` / `Stream.Evict` / `Stream.LiveCount` | 加引用并返回图（Loading 报 NotFound；Missing/Failed 重放原因）/ 掉一引用（未知 NotFound、无活引用 InvalidArg 不动；缓存字节留着便于重载）/ 无活引用时释放缓存图 / 活引用数 |
| `Stream.Stats` / `Stream.Count` / `Stream.TotalBytes` | 计数器拷贝 / Ready 条数 / 保留解码 RGBA8 字节 |

## 23. `engine/tilemap` — 瓦片地图（126 条：顶层 38 + 方法 88）

> 冻结 CPU 侧瓦片地图数学（2026-09-15 冻结口子；10-04 S84 加大地图流送；LOD 规则与自动拼接规则文件后冻——不读文件，阈值只经 NewLOD/调用方注入）。正交网格：格 (col,row) 左上 = (col·TileW, row·TileH)；斜 45° 等距：包围盒左上 = ((col−row)·TileW/2, (col+row)·TileH/2)，菱形中心加半格；WorldToCell 向下取整到所属格、出图 ok=false。层五种：ground/decoration 载美术 GID、collision 载实心格、navigation 载移动代价、occlusion 载阴影格；GID 0 是空格——永不实心、永无代价、永不遮影；层宽高必须等于地图宽高。物件摆怪摆箱摆出生点：像素矩形（与 CellToWorld 同单位）。自动拼接（路沿自接，仅内部规则）：AutoMaskAt 读 4 邻存在位（N=1,E=2,S=4,W=8）、AutoVariant4 把 0..15 映到变体（当前恒等）。TMX 子集先冻：orthogonal/isometric、地图尺寸、tileset 头、csv 层、矩形物件、kind 属性；六边形/staggered/无限图/非 csv/椭圆多边形模板物件一律 Unsupported，坏 XML BadData。本包不画不 import camera：视图以 core.Rect 喂入。**分块流送**（Chunk）：把图切成 chunkW×chunkH 块并跟踪已载集合（读写锁保护 loaded——窗 tick 线程写、光栅线程读）；**大图**（S84 Large）：32×32 冻结分区 + 后台请求队列 + 每帧前台合并预算（S84：流送一帧至多 2ms）；**LOD**：迟滞带内保持原档，抖动镜头不闪。

#### 常量（14）

| 符号 | 说明 |
|------|------|
| `MaxChunks` | 单网格分块上限（1M）：坏尺寸快速失败而不是卡死 Needed 扫描；超出 NewChunker 报 OutOfMemory |
| `LargeChunkW` / `LargeChunkH` / `MaxLargeTiles` | S84 冻结 V3 分区：32×32 格一块 / 单张大图上限 1024×1024=一百万格（超出 NewLarge 报 OutOfMemory） |
| `DefaultMergePerFrame` | 前景合并每帧上限 4（走路不超每帧预算） |
| `OrientOrthogonal` / `OrientIsometric` | 网格数学选择：方格 / 斜 45° 菱形 |
| `KindGround` / `KindDecoration` / `KindCollision` / `KindNavigation` / `KindOcclusion` | 层含义五态：地面美术 GID / 地面之上的装饰 GID / 实心格（非零 GID 挡路）/ 移动代价（非零 GID 即代价）/ 阴影格（非零 GID 变暗） |
| `LevelNear` / `LevelFar` | 分块 LOD 档：完整细节（距离≤近线）/ 简化绘制（距离≥远线） |

#### 类型（12）

| 符号 | 说明 |
|------|------|
| `ChunkID` | 一个分块地址（CX/CY 数的是块不是格；块 (0,0) 盖 col[0,chunkW) row[0,chunkH)） |
| `Chunk` | 分块器：把图切成块并跟踪已载集合；视图是喂进来的 core.Rect（永不 import camera 包）；loaded 有读写锁（窗 tick 写 / 光栅读，真指针事件的手动跑不崩） |
| `Iso` | 斜 45° 换算器：格→菱形包围盒/中心/判定；TileToWorld 喂相机与投影数学 |
| `Large` | 大图流送索引（架在 Chunk 网格上；网格按指针持有、构造后绝不拷贝——Chunk 带锁）：loaded/pending 集合 + 每帧合并预算 |
| `Level` | 分块 LOD 档枚举 |
| `LOD` | 按距离的分块 LOD 开关（带迟滞带：≤Near 保持精细、≥Far 保持简化、(Near,Far) 之间保持原档——抖动镜头不闪）；相机焦点以 core.Vec2 喂入 |
| `Orientation` / `LayerKind` | 网格数学枚举 / 层含义枚举，见常量行 |
| `Tileset` | 一张美术图集：GID 区间基（firstGID）+ 名 + 数量 + 列数 |
| `Layer` | 一层瓦片平面：W×H 个 GID 行主序（0=空） |
| `Object` | 一个摆件：id/名/类型 + CellToWorld 单位的像素矩形 |
| `Tilemap` | 整张图：网格参数 + 层表 + 物件表 + 图集表 |

#### 函数（12）

| 符号 | 说明 |
|------|------|
| `NewChunker` | 在 mapW×mapH 图上建分区（tile 尺寸与 chunk 尺寸按格）：大于图的 chunk 塌缩成一块、尾行尾列收余格；坏尺寸 InvalidArg、超 MaxChunks OutOfMemory |
| `NewIso` | 建斜 45° 换算器：tile 尺寸须有限且 >0 |
| `NewLarge` / `NewLargeSized` | 建可载 1024 图的流送索引（冻结 32×32 分区 + 32×32 世界格；其他尺寸用 Sized 版）/ 显式几何建索引（tile/chunk 须有限 >0；格总数须进 MaxLargeTiles、块网格须进 MaxChunks） |
| `ChunkDist` | 焦点到块包围盒距离：在内或贴边为 0，否则到最近边的欧氏距；坏/空输入 ok=false |
| `NewLOD` | 建 LOD 开关：距离须有限且 0<near<far；坏尺寸 InvalidArg 绝不猜带 |
| `NewTilemap` / `NewTileset` / `NewLayer` / `NewObject` | 建空图（尺寸 >0、tile 有限 >0）/ 建图集条目（firstGID>0、名非空）/ 建层（GID 拷贝、长须 W×H、每个 GID≥0、名非空、kind 五态之一）/ 建物件（id>0、bounds 有限 W/H≥0；零尺寸=点出生点，仍可被 ObjectsIn 匹配） |
| `AutoVariant4` | 4 位边缘掩码 0..15 → 美术变体下标（当前恒等；规则文件后冻不读文件）；越界掩码 ok=false |
| `ParseTMX` | 解析冻结 TMX 子集（orthogonal/isometric、csv 层、矩形物件、kind 属性）：坏内容 BadData、预留未建形状 Unsupported、空输入 InvalidArg |

#### 方法（88，按接收者分组）

| 符号 | 说明 |
|------|------|
| `Chunk.Orient` / `Chunk.W` / `Chunk.H` / `Chunk.TileW` / `Chunk.TileH` / `Chunk.ChunkW` / `Chunk.ChunkH` / `Chunk.ChunkCols` / `Chunk.ChunkRows` / `Chunk.LoadedCount` | 读网格数学选择 / 图宽（格）/ 图高 / 格宽 / 格高（世界单位）/ 块宽（格）/ 块高 / 横向块数 / 纵向块数 / 当前已载块数 |
| `Chunk.ChunkOf` / `Chunk.ChunkBounds` | 格→所属块（出界 ok=false）/ 块的世界包围盒（正交=格块；等距=成员菱形盒并集——保守界，邻居按设计重叠；未知 id ok=false 零值） |
| `Chunk.Needed` | 与 view 相触的块列表（按 (CY,CX) 排序回放逐位一致；贴边不算——只算正面积重叠；坏/空视图什么都不需要不 panic） |
| `Chunk.Load` / `Chunk.Unload` / `Chunk.Update` | 标记 Needed(view) 已载并返回新载块（排序、无新 nil；坏视图不动）/ 卸掉 view 外已载块并返回被卸的（空但有限的视图=全卸；坏视图不动）/ 一步完成载+卸（跳跃镜头不留陈块不涨：之后 Loaded==Needed(view)） |
| `Chunk.Visible` / `Chunk.Loaded` / `Chunk.IsLoaded` / `Chunk.Reset` | 已载且与 view 相触的块=绘制清单（镜头外不画；不改已载集）/ 已载块列表（排序新切片）/ 是否已载（未知 false、nil 网格永不载）/ 全卸（nil 静默） |
| `Iso.TileW` / `Iso.TileH` / `Iso.TileToWorld` / `Iso.TileCenter` / `Iso.TileBounds` / `Iso.DiamondContains` / `Iso.WorldToTile` | 格宽 / 格高 / (col,row) 菱形包围盒左上（任意整数格可映射；NaN 只来自溢出——ok=false 零值）/ 菱形中心 / 菱形包围盒（W=tileW,H=tileH）/ 世界点是否在菱形内（边算在内；坏输入恒 false 不 panic）/ 世界点向下取整到所属菱形（包围盒重叠，逆公式只播 3×3 种子搜索、菱形判定定主；共享边属两菱形、顶点属三菱形——候选优先序给出确定唯一主；中心恒回自家格；坏输入 ok=false 零值） |
| `Large.W` / `Large.H` / `Large.ChunkW` / `Large.ChunkH` / `Large.Cols` / `Large.Rows` / `Large.ChunkCount` / `Large.LoadedCount` / `Large.PendingCount` / `Large.Requests` / `Large.Merges` | 图宽 / 图高 / 块宽 / 块高 / 横向块数 / 纵向块数 / 总块数（1024² 图即 1024）/ 已载块数 / 待前台合并块数 / 历史排队总数 / 历史合并到 loaded 总数 |
| `Large.ChunkOf` / `Large.ChunkBounds` / `Large.Needed` / `Large.IsLoaded` / `Large.IsMissing` / `Large.Loaded` / `Large.Pending` / `Large.Visible` | 格→块（出界 false）/ 块世界盒（未知 ok=false 零值）/ 与 view 相触块列表（(CY,CX) 序）/ 是否已载（未知 false）/ 是否合法块但仍显示品红占位（网格知道但未载）/ 已载列表（排序新切片）/ 排队列表（队序新切片——窗画琥珀色让预算可见）/ 已载且触 view 的绘制清单 |
| `Large.Request` / `Large.MergeBudget` / `Large.Update` / `Large.Reset` | 把 view 需要且既未载也未排队的块排进后台解析队列（返回新排队块，排序；空/坏视图不排）/ 把至多 maxPerFrame 个排队块并入 loaded（每帧预算内的前台合并，返回队序；maxPerFrame≤0 不并、空队 nil）/ 流送一帧：排队 view 所需 → 至多 maxPerFrame 合并 → 卸掉 view 外的（merged 队序、dropped 排序；跳跃镜头不留陈块不涨：之后 Loaded ⊆ Needed(view)；空视图保留一切——resize 安全，瞬态零尺寸视图绝不卸图，与 Chunk.Update 不同）/ 全卸并清队（nil 静默） |
| `Level.String` | "near"/"far"/"unknown"（日志与 JSON 用） |
| `LOD.Near` / `LOD.Far` / `LOD.Band` / `LOD.Select` / `LOD.ChunkLevel` | 近线（精细距离）/ 远线（简化距离）/ 迟滞带宽（Far−Near，恒 >0）/ 按 dist 选档（带内保持 cur；坏距离保 cur；带内非法 cur 在中点劈开；带外由距离独裁——新块照样落）/ 从相机焦点给单块选档（坏输入保 cur 且 ok=false 不 panic） |
| `Tilemap.Orient` / `Tilemap.W` / `Tilemap.H` / `Tilemap.TileW` / `Tilemap.TileH` / `Tilemap.Layers` / `Tilemap.Objects` / `Tilemap.Tilesets` | 网格数学 / 图宽 / 图高 / 格宽 / 格高 / 层表拷贝 / 物件表拷贝 / 图集表拷贝 |
| `Tileset.FirstGID` / `Tileset.Name` / `Tileset.TileCount` / `Tileset.Columns` | 图集基 GID / 名 / 瓦片数 / 图集列数 |
| `Layer.Name` / `Layer.Kind` / `Layer.GIDs` | 层名 / 层含义 / 行主序 GID 拷贝（0=空） |
| `Object.ID` / `Object.Name` / `Object.Type` / `Object.Bounds` | 物件 id / 名 / 类型（spawn/chest…）/ CellToWorld 单位像素矩形 |
| `Tilemap.AddTileset` / `Tilemap.AddLayer` / `Tilemap.AddObject` | 追加图集条目 / 追加层（尺寸必须等于图尺寸、名唯一）/ 追加物件（id 必须唯一） |
| `Tilemap.LayerIndex` / `Tilemap.At` | 按名找层下标 / 第 layerIdx 层 (col,row) 的 GID（出界 ok=false 且 gid 0——缺块占位不崩：调用方把空当可走美术洞） |
| `Tilemap.CellToWorld` / `Tilemap.WorldToCell` / `Tilemap.CellBounds` | 格原点世界位（正交=左上；等距=包围盒左上；出界 ok=false 零值）/ 世界点向下取整到所属格（等距取包含点的菱形；坏输入或图外 ok=false）/ 格包围盒（W=tileW,H=tileH） |
| `Tilemap.ObjectsIn` | 触到 rect 的物件（盒物件按重叠、点物件按点在 rect 内；坏/空 rect 返 nil 不 panic） |
| `Tilemap.IsSolidAt` / `Tilemap.NavCostAt` / `Tilemap.OccludedAt` / `Tilemap.AutoMaskAt` | 任意 collision 层是否盖住 (col,row)（出界不实心=空气，绝不是错）/ (col,row) 的移动代价（按层序首个非零 navigation GID；无数据或出界 ok=false）/ 任意 occlusion 层是否遮影（出界永不遮影）/ 单层 4 邻存在位掩码（N=1,E=2,S=4,W=8；图外邻格算空；坏层或出界 ok=false） |

## 24. `engine/world` — 实体世界（112 条：顶层 35 + 方法 77）

> 冻结 CPU 侧实体数学（2026-09-15 冻结口子；11-04 P3.7 批 2 步 2d 给 Transform 加 Height，只做加法）。**只算数、不画不步进**：调用方生成实体、把能力（sprite/collider/script）作为 comps 挂上、链父子、读世界变换喂既有绘制。**身份**：ID 从 1 起、同一 World 内永不重复；NoEntity(0) 表示无父、永不指名活实体；Spawned 只增不减（回放可证 id 无复用）、Count 数活的。**层级**（Godot 节点同念）：每实体一份本地变换 + 至多一个父；`WorldOf` 自根向下折叠（世界缩放=逐分量积、世界旋转=直和、世界位置=父位置+父旋转后的缩放本地偏移）；`WorldMatrix` 是同一折叠的矩阵形式（parent·local），与 WorldOf 在 1e-9 内一致；Despawn 父不删子（子回落成根、保本地变换）；成环与自父一律拒绝。**Comps**：Kind 非空命名能力、Ref 载资产 id（纯逻辑可空）；允许重复（一实体两个脚本）、RemoveComp 摘首个匹配。**Height**（P3.7 2d）：离地高度只喂投影级（精灵画的针孔深度）——刻意不进 2×3 仿射链（Local/WorldMatrix）也不进遮挡排序键（仍读脚位）；父链按直和折叠（悬空孩子骑着父母而非贴地）；prefab/scene 文件以可选键携带，版本 1.0→1.1（各自规则：追加字段升 minor），1.0 文件照载读 height 0。**生命周期**（Scene）：出生即活跃、Sleep 停放、Wake 复活、Dispose 清理。**文件**：Prefab 可复用实体模板、SceneFile 关卡文件（父先于子），Parse/Load 读 JSON、Encode/Save 写规范字节、Instantiate 盖章进 World、Open 建活 Scene + 名字→活 id 索引（文件往返逐字节一致）。**大场景可见性索引**（S85 Cull）：世界是唯一摆放真相、Cull 只缓存绘制带/脚偏移/可见半尺寸/钉住标记；Refresh 从活世界数字重建更新集（视内+钉住=可见候选、entered/exited 差分、可选驱动 Scene 睡醒、active 按 layer→feetY 排序）；Snapshot 零分配供逐帧绘制读（位置与 WorldOf 精确一致，单测钉死）；挂未导出排序类型的 6 个导出方法名不属公开 API（见 §0 注）。

#### 常量（11）

| 符号 | 说明 |
|------|------|
| `NoEntity` | 空实体 id（0）：无父、无匹配 |
| `MaxCullEntities` | 单 Cull 索引容量 32768（超出 Track 报 OutOfMemory 绝不半收录；20000 大场景实体有余量） |
| `MaxPrefabNameLen` / `MaxPrefabComps` / `MaxPrefabBytes` | prefab 名 64 字节 / 单模板 comps 16 / 单文件 64KB |
| `LifeActive` / `LifeSleeping` | 生命周期标记：运行中（Spawn 出生在此）/ 活着但停放到 Wake |
| `MaxSceneNameLen` / `MaxSceneEntities` / `MaxSceneComps` / `MaxSceneBytes` | 场景与实体名 64 字节 / 单场景文件实体 4096 / 单实体 comps 16 / 单文件 4MB |

#### 变量（2）

| 符号 | 说明 |
|------|------|
| `PrefabVersion` | prefab 文件版本（1.1：P3.7 2d 加可选 local "height" 键；1.0 文件按 core CompatibleWith 规则照载读 height 0） |
| `SceneFileVersion` | 场景文件版本（1.1：可选实体 "height" 经共享 jsonTransform；1.0 照载读 height 0） |

#### 类型（11）

| 符号 | 说明 |
|------|------|
| `ID` | 实体 id；NoEntity 永不指名活实体，在 Spawn/SetParent 里意为「无父」 |
| `Transform` | 本地摆放：位置 + 旋转（弧度逆时针）+ 缩放（可零=塌缩可负=镜像；只有非有限数被拒）+ **Height** 离地高度（只喂投影级，不进仿射链不进遮挡键，父链直和折叠） |
| `Comp` | 挂在实体上的一个能力：Kind 非空命名（"sprite"/"collider"/"script"…）、Ref 载资产 id（纯逻辑可空） |
| `World` | 全部实体的持有者：id、链接、本地变换、comps；零值可用（NewWorld 表达意图）；id 永不重复（Despawn/Clear 之后也不） |
| `Prefab` | 一个可复用实体模板：名 + 本地摆放 + 盖章序 comps；字段私有、写入过校验、读取拷贝 |
| `LifeState` | 生命周期枚举，见常量行 |
| `Scene` | 一个关卡：实体存储（World）+ 激活标记表；出生跑 active、Sleep 停放、Wake 复活、Dispose 清 comps 与标记 |
| `SceneNode` | 一个落盘实体：名 + 父名（根为 ""）+ 本地摆放 + 盖章序 comps |
| `SceneFile` | 一份关卡文件：名 + 按文件序的实体表（父先于子）；字段私有、Entities 返回深拷贝 |
| `ActiveSnap` | 更新集成员及其世界位（生产读路径）：id、世界 X/Y（与 WorldOf 精确一致，单测钉死）、绘制带、是否钉住、父 id；Snapshot 在 Refresh 时填充——逐帧调用方拷结构体即可，省去每活跃 WorldOf+三次 map 查找 |
| `Cull` | 大场景可见性索引：世界是唯一摆放真相；Cull 只缓存逐实体的绘制带、脚偏移、可见盒、钉住标记；Refresh 从活世界数字重建活动集，World 里移实体就够——绝无第二张摆放表可漂移 |

#### 函数（11）

| 符号 | 说明 |
|------|------|
| `IdentityTransform` | 中性摆放：原点、无旋转、单位缩放 |
| `LocalMatrix` | 把 t 渲染成 Translate(Pos)·Rotate(Rot)·Scale(Scale)（同 core.Mat2D 布局；render 边界处换一次） |
| `NewWorld` | 建空世界（id 从 1 发） |
| `NewPrefab` | 建带 PrefabVersion 的模板：空/超长名、非有限数 InvalidArg 不落 |
| `ParsePrefab` / `LoadPrefab` | 校验 prefab 字节（空 InvalidArg、超尺寸 OutOfMemory、坏 JSON/坏形状 BadData、外来或更新版本 VersionMismatch；缺 comps 默认空）/ 读盘（空路径 InvalidArg、缺文件 NotFound；其余同 Parse） |
| `NewScene` | 建空关卡（id 从 1 发） |
| `NewSceneFile` | 建带 SceneFileVersion 的空关卡文件：空/超长名 InvalidArg |
| `ParseScene` / `LoadScene` | 校验场景文件字节（坏形状含重名/未知或前向父/坏链接 → BadData；其余档位同上）/ 读盘（档位同 LoadPrefab） |
| `NewCull` | 在 view 上建可见性索引（view 须有限，空视图合法=只认钉住；AutoSleep 默认关、MaxActive 默认不限） |

#### 方法（77，按接收者分组）

| 符号 | 说明 |
|------|------|
| `World.Spawn` / `World.Despawn` | 在 parent 下生成实体（NoEntity=根；恒等变换无 comps；缺父 NotFound）/ 删除 id 并掉 comps（子存活回落成根保本地变换；缺 id NotFound 绝不崩） |
| `World.Alive` / `World.Count` / `World.Spawned` | 是否活实体（nil 与 NoEntity false）/ 活实体数 / 历史出生总数（含已回收——只增，回放可证 id 不复用） |
| `World.SetParent` / `World.Parent` / `World.Children` | 重挂 child 到 parent 下（NoEntity=摘成根；自父与成环 InvalidArg、缺 id NotFound、同父重挂 no-op）/ 读父（根 NoEntity）/ 直接子列表（链接序、新切片） |
| `World.SetTransform` / `World.Local` / `World.WorldOf` / `World.WorldMatrix` | 整体替换本地变换（非有限 InvalidArg 不落）/ 读本地变换 / 世界变换（自根向下折叠） / 世界矩阵（parent·local 逐级折叠；无旋转或等比链与 LocalMatrix(WorldOf) 在 1e-9 内一致；平移恒一致） |
| `World.AddComp` / `World.RemoveComp` / `World.HasComp` / `World.Comps` | 挂 comp（空 Kind InvalidArg；允许重复——两个脚本按序排队）/ 摘该 kind 的首个 comp（实体没带 NotFound）/ 是否至少带一个该 kind / comp 列表（按挂序、新切片） |
| `World.Clear` | 清全部实体与 comp；id 发放永不回卷（下一次 Spawn 仍发高于历史所有 id 的新 id） |
| `Prefab.Name` / `Prefab.Version` / `Prefab.Local` / `Prefab.SetLocal` / `Prefab.AddComp` / `Prefab.Comps` / `Prefab.Equal` | 模板名 / 文件版本 / 本地摆放 / 整体替换摆放（非有限 InvalidArg 不动）/ 追加 comp（盖章序；空 Kind InvalidArg、超 MaxPrefabComps OutOfMemory）/ comps 拷贝（盖章序）/ 名、版本、摆放、comps 序全同才等 |
| `Prefab.Instantiate` | 把模板盖进 w（parent 下 NoEntity=根）：一次 Spawn + 存的 local + comps 按序；缺父 NotFound、nil world InvalidArg；失败的盖章什么都不生——半成品实体在报错前移除 |
| `Prefab.Encode` / `Prefab.SavePrefab` | 渲染规范文件字节（模板须已带 PrefabVersion，否则 VersionMismatch；坏形状 InvalidArg、超预算 OutOfMemory；空 comp 表编码成 [] 使往返逐字节一致）/ 写盘（自动建父目录；档位同 Encode） |
| `LifeState.String` | 状态名（日志用） |
| `Scene.Spawn` / `Scene.Sleep` / `Scene.Wake` / `Scene.State` | 在 parent 下生成（出生即活跃；缺父 NotFound）/ 停放（活着但不活跃；重复 sleep no-op）/ 复活（重复 wake no-op）/ 读激活标记 |
| `Scene.IsActive` / `Scene.IsSleeping` / `Scene.Alive` / `Scene.Dispose` | 是否活着且在跑 / 是否活着且停放 / 是否活实体（两种都算）/ 销毁并清资源（comps 与生命周期条目同落；子存活为根保各自状态；缺 id NotFound） |
| `Scene.Count` / `Scene.ActiveCount` / `Scene.SleepingCount` / `Scene.Spawned` / `Scene.Clear` / `Scene.World` | 活实体数（活跃+停放）/ 活跃数 / 停放数 / 历史出生总数（含已销毁——只增，回放可证 id 不复用关卡不漏）/ 换关卡清场（id 发放不回卷）/ 暴露本关卡的摆放与 comp 存储——生死与换关一律走 Scene，禁对返回值调 Spawn/Despawn/Clear |
| `SceneFile.Name` / `SceneFile.Version` / `SceneFile.Count` / `SceneFile.Entities` | 关卡名 / 文件版本 / 落盘实体数 / 落盘实体表（文件序、comp 切片全新；nil 文件 nil） |
| `SceneFile.AddEntity` | 在父之后落盘一个实体（""=根）：重名、坏摆放、空 comp kind InvalidArg；未知父 NotFound（先落父）；超 MaxSceneEntities/MaxSceneComps OutOfMemory；失败不落任何东西 |
| `SceneFile.Equal` / `SceneFile.Encode` / `SceneFile.SaveScene` / `SceneFile.Open` | 名、版本、实体序与 comps 全同才等 / 渲染规范字节（须已带版本；空实体与 comp 表编码成 [] 使往返逐字节一致）/ 写盘 / 建活关卡：按文件序逐一生成（父先于子）、local 与 comps 盖章、出生全活跃；返回（Scene, 文件名→活 id 索引）；坏内存文件 InvalidArg/OutOfMemory 且什么都不建；落盘形状永不改动 |
| `Cull.SetView` / `Cull.View` / `Cull.SetAutoSleep` / `Cull.AutoSleep` / `Cull.SetMaxActive` / `Cull.MaxActive` | 换视图框（非有限 InvalidArg 保旧；空视图合法=只认钉住）/ 当前视图 / 切使能（开=Refresh 唤醒每个可见睡眠者并把退出且未钉的经 Scene 入睡；关=Refresh 不写 Scene 只报集合）/ 读使能 / 帽定更新集（超 n 的 Refresh 保绘制序前 n、其余计 Trimmed；0=不限；负数 InvalidArg 保旧）/ 读帽（0=不限） |
| `Cull.Track` / `Cull.Update` / `Cull.Untrack` | 登记 id 的绘制带/脚偏移/可见半尺寸（0/0=点，如同出生标记；钉住走 Awake 不在此处；NoEntity 与重复 InvalidArg、超 MaxCullEntities OutOfMemory；实体不必已活——死 id 在下次 Refresh 静默掉并计 Dropped）/ 改写已登记 id 的带/脚偏移/半尺寸（未知 NotFound、坏数不动）/ 忘掉 id（World 实体不动，只掉索引条目） |
| `Cull.Awake` / `Cull.Release` / `Cull.IsAwake` | 钉住 id（出画也留在更新集——相机目标、玩家）/ 摘钉 / 读钉（未知 false） |
| `Cull.Refresh` | 从活世界数字重建更新集：每个已登记实体折叠世界位、视内+钉住成可见候选、与上次差分 entered/exited、使能开则驱动 Scene 睡醒、最终 actives 按 layer→feetY 排序；死 id 静默掉计 Dropped；nil World InvalidArg 不动；nil Scene=纯查询（不读睡态不写使能） |
| `Cull.Snapshot` | 更新集+世界位（绘制序），由索引 scratch 背书：仅到下次 Refresh/Reset 前有效、禁止长持；零分配——逐帧绘制读它而不是逐活跃付 WorldOf |
| `Cull.Active` / `Cull.ActiveCount` / `Cull.Entered` / `Cull.Exited` / `Cull.IsActive` / `Cull.InView` | 更新集（绘制序：layer 升序→feetY 升序、平局稳定；新切片）/ 更新集数 / 上次 Refresh 后新进可见集的 id（id 序）/ 新退出的 id / id 是否在当前更新集（线性扫——抽查用，别放逐帧循环）/ id 是否在当前可见集（视内或钉住——即使睡着不在更新集也算） |
| `Cull.Count` / `Cull.Dropped` / `Cull.Trimmed` / `Cull.Refreshes` / `Cull.LastRefreshUs` / `Cull.Reset` | 已登记实体数 / 上次 Refresh 掉的死 id 数 / 超 MaxActive 被裁的活跃数 / Refresh 调用计数（含失败） / 上次 Refresh 微秒成本（全量 20000 折叠 + active-only 排序；仅参考——门禁由测试与窗量）/ 清空跟踪与计数（视图、使能、帽保留——换关卡同帧重登记） |

---

## 修订表

| 日期 | 修订 |
|------|------|
| 2026-11-14 | 建账：按 go/ast 扫描冻结 engine 树（根包 + 24 子包）全部公开 API 共 1842 条（顶层 903 + 导出类型的导出方法 939）入目录，逐条中文说明（源 doc 注释整理 + 无注释 72 条对照实现补写）；公开面判定=顶层导出符号与**导出类型**的导出方法，挂未导出接收者类型的 17 个导出方法名（nav 5、physics 6、world 6）包外不可达、不计入；`scripts/apidoc` 扩展 engine 域机器校验（本文件与代码逐符号对账，非零退出即漏）。 |

