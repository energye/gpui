# 生产级 2.5D 动画背景能力差距表

日期：2026-09-14
范围：生产级 2.5D 游戏背景，不是某一个示例的那点东西。
结论先说：现在能直接用的只有两成，八成还没做。

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

### 1. 镜头和远近

近大远小、镜头推拉跟随震动、远近景错开动，游戏叫相机和视差。

- 1.1 真透视变换：现在矩阵只有 6 个数，只能平移缩放旋转错切，算不出透视除法。位置：`render/matrix.go`、`render/internal/image/affine.go`。要做：加透视矩阵类型，CPU 和显卡两条路算出同一个结果。
- 1.2 前后遮挡：游戏要的深浅值排序接口现在没有，画出来只看谁后画谁盖谁。源码里显卡确实用了深度比较，但只给任意路径裁剪用（`render/internal/gpu/shaders/depth_clip.wgsl`、`stencil_renderer.go`、`glyph_mask_pipeline.go`、`convex_renderer.go`里的 `DepthCompare=GreaterEqual`），不对游戏开放。要做：加深度值，远的先画近的后画，显卡打开深度比较。
- 1.3 镜头能力：推拉、跟随、震动、变焦、远近景错开动，外加限位、平滑跟随、锚点、无限重复衔接。现在场景层只有平移加旋转加缩放（`ui/scene/layer.go` 的 `TransformLayer`：`TX,TY,Rotation,SX,SY,CX,CY`，另有 `OffsetLayer`），不是相机，只能每个示例自己手算。要做：引擎里给一套镜头，位置、缩放、角度、震动、限位、平滑、锚点一次说清；视差层支持重复镜像，长路不露缝。
- 1.4 梯形贴图对齐：`render/m4_extensions.go` 的 `DrawImageQuad`，显卡能画梯形，CPU 直接拉成方块。要做：CPU 补真透视采样，或者明确不支持时报错，不要静默画错。

### 2. 小图一次多画

很多树人石头一次交给显卡，游戏叫精灵合批和图集。

- 2.1 精灵合批：入口现在是逐个排队（`render/vertices.go` 的 `DrawAtlas` 对每个块调一次 `QueueImageDraw`），会话层只合并连续同图同透明度同过滤同视口的块（`render/internal/gpu/image_pipeline.go` 的 `canMergeImageDraw` 共用绑定组一次画多个四边形，另有 `canMergeGPUTextureDraw`）。跨图合批、旋转染色、自动排序都没有。要做：同一张大图的一批小块，一次提交，少几次调用。
- 2.2 旋转和染色：现在 `AtlasSprite` 里只有位置和透明度，转不了换不了颜色。要做：加旋转角度、缩放中心、整体染色，外加翻转、轴心、单图过滤（近的清、远的糊各管各）。
- 2.3 前后排序：按高低自动决定谁盖谁，外加层号、世界和界面分层（特效别飘到界面上）。现在没有，只有渲染层内部给图层排号（`render/render/layers.go` 的 `zOrder`，管图层不管精灵），游戏精灵全靠手写顺序。要做：按世界高度排序再画，层号大的盖住小的。
- 2.4 序列帧播放器：几张图轮着放形成动作，外加多套动作库（待机跑跳各一套）。现在没有，得手写计时切换。要做：帧号、时长、循环 ping-pong、事件回调，多动作切换。

### 3. 大图和显存

大图片怎么存、远了糊不糊、加载卡不卡。

- 3.1 游戏压缩图：ASTC、ETC、Basis、KTX。读图入口现在只有 PNG、JPG、WebP（`render/internal/image/io.go` 的 `LoadPNG/LoadJPEG/LoadWebP/LoadImage/Decode`），图片格式也只有 8 位系（`render/internal/image/format.go`：`Gray8/Gray16/RGB8/RGBA8` 系，无浮点无压缩块格式）。显卡格式枚举里虽有 BC1-7、ETC2、ASTC 常量（`gpu/types/texture.go`），但没接解码上传链路。要做：至少支持一种显卡直接读的压缩格式。
- 3.2 远处防闪：CPU 有多级缩小链 `render/internal/image/mipmap.go`，显卡贴图主采样器是双线性放大加双线性缩小加最近多级（`render/internal/gpu/image_pipeline.go`：`MagFilter=Linear,MinFilter=Linear,MipmapFilter=Nearest`，没写各向异性即默认 1），其它管线多处写死 `Anisotropy:1`，只有文字管线给了 4（`render/internal/gpu/text_pipeline.go`）。要做：显卡补多级渐远，各向异性可调，CPU 和显卡缩出来一致。
- 3.3 后台边玩边加载：现在读图是同步卡住读。要做：异步解码、流式进显存、大地图不卡顿。

### 4. 让东西动起来

动作自然不僵硬那套，游戏叫动画系统。

- 4.1 变速曲线：快慢变化。现在只有 `Tick(dt)`，时间大力还会被钳住，见 `ui/scheduler/ticker.go`。要做：常用缓动曲线库。
- 4.2 关键帧时间线：按时间轴摆动作，外加事件軌（到第几帧出刀光、播声音、调方法、显隐）。现在没有。注意 `ui/kit/timeline` 是界面时间线组件，不是动画时间线，别被名字误判为已有。要做：时间轴、关键帧、插值、循环、事件回调。
- 4.3 骨骼蒙皮：人物走跑跳换动作，对齐 Spine 结构：骨头、插槽、皮肤、网格附件、权重、绘制次序、IK和约束、物理惯性。现在源码搜骨骼、Spine、IK 全是空，搜到带骨架字样的只是文字描边测试（`hbSkeleton`、`Skeleton` 指字形骨架）。要做：骨骼、蒙皮权重、反向带动、动作混合、状态机，保证真美术资源能直接用。

### 5. 火烟水气和后期

氛围感那套，游戏叫粒子和全屏后期。

- 5.1 粒子发射器：火烟雨雪落叶，外加发射形状（点锥盒环）、乱流、子发射器。现在没有。要做：发射数量、速度、寿命、重力、颜色变化、形状、乱流、子发射。
- 5.2 显卡粒子和拖尾：几千个小点一起动不卡，拖尾带宽窄变化、颜色渐变、拐角接头。现在没有。要做：显卡算位置，拖尾按轨迹画，宽窄颜色跟走。
- 5.3 扭曲热浪：水面晃、空气抖。现在没有。要做：按噪声偏移采样。
- 5.4 发光暗角调色：现在 `render/filter_ops.go` 只有模糊、阴影、颜色矩阵等 6 种，搜发光、暗角、调色、色调映射都是空。要做：发光、暗角、颜色查找表、色调映射，外加自定义材质钩子（美术要特殊描边溶解能接进来）。
- 5.5 自定义材质钩子（新补）：特效不够时美术自己写一段小着色器。现在没有。要做：统一钩子，只碰游戏层，不碰界面主路。

### 6. 灯光

白天黑夜、手电光影。

- 6.1 2D 灯光：哪里亮哪里暗，外加灯片（手电形状）、强度颜色、只照哪层、全局夜色。现在没有。要做：点光、方向光、范围衰减、灯片、强度颜色、分层照、分层不照、全局调制。
- 6.2 法线受光：平面图有立体感。现在没有。要做：法线贴图参与受光计算。
- 6.3 投影和环境遮挡：影子、角落变暗。现在没有。要做：投影、遮挡系数。

### 7. 大地图

大世界怎么装下，游戏叫瓦片和分区。

- 7.1 瓦片地图和斜 45 度地图：含 TMX 解析，外加对象层（摆怪摆箱）、碰撞导航遮挡层、自动拼（路沿自己接上）、无线大图分区、预加载半径。要做：瓦片装载、斜 45 度投影、对象层、自动拼、碰撞层。
- 7.2 分区加载和视野剔除：只加载只画镜头里那块。现在没有。要做：按块装卸，镜头外不画。
- 7.3 远近切换：远了用简单版，近了用精细版。现在没有。要做：按距离换细节等级。

### 8. 帧节奏和内存

长时间跑不崩不慢。

- 8.1 固定步长加补间：快慢机器动作一致。现在时间步长不固定。要做：逻辑固定步长，渲染按余量插值。
- 8.2 路径顶点复用池：路径对象现在靠示例自己 `Reset`（`render/path.go` 有 `NewPath/Reset/Clone`，无对象池），顶点侧上下文有草稿复用（`render/vertices.go`、`render/context.go` 的 `vertDevScratch` 等），不是完整的池化。要做：路径、顶点缓冲复用。
- 8.3 动态世界局部更新：`ui/scene/picture.go` 的留存只认矩形路径文字图片，`DrawAtlas`、`DrawVertices`、`DrawMesh` 进不了留存；`ui/rendering/boundary_cache.go` 只会存静的；脏区超 16 块就合并，见 `render/context.go`。会动的东西现在每帧全重画。要做：动态层脏区细分，精灵层独立更新。

### 9. 颜色和两边一致

换机器画面不变。

- 9.1 高动态颜色和亮度流程：主链路还是 8 位。图片格式只有 8 位系，主渲染目标是 `RGBA8Unorm/BGRA8Unorm`（`render/render/target.go`、`layers.go`），`gpu/types` 里虽有 `R16Float/RGBA16Float` 等常量但主链路没用。要做：浮点目标、正经亮度流程、色调映射。
- 9.2 CPU 回退画质：`render/vertices.go` 的渐变在 CPU 是取平均填纯色，图集在 CPU 是逐个画。要做：CPU 和显卡像素对齐，或者回退时明确降级标记。

## 参考引擎（只抄做法，不搬代码）

画画那半抄 Skia，玩法那半抄游戏引擎。原因一句话：Skia 是画布库，没有镜头、骨头、大地图这些玩法。

| 参考谁 | 用来定什么 | 怎么用 |
|---|---|---|
| Skia（含 SkCanvas、SkParticles、SkImageFilter） | 透视矩阵思想、贴图多级渐远加各向异性、合批思路、滤镜链、粒子模块思想 | 只看接口和算法，Go 重写一遍接到现有 `render/` 接口上 |
| Godot 4（Camera2D、ParallaxBackground、Sprite2D、AnimatedSprite2D、AnimationPlayer、AnimationTree、CPUParticles2D、GPUParticles2D、Line2D、TileMap、TileSet、Skeleton2D、PointLight2D、LightOccluder2D、CanvasModulate） | 镜头、视差、前后排序、序列帧、变速曲线、时间轴、状态机、粒子、拖尾、瓦片、骨头、2D 灯光影子、固定步长 | 主参考。只看节点有哪些属性、数怎么算，不搬 C++ 代码 |
| Cocos Creator（含 Sprite、Animation、TiledMap、Spine 挂接） | 同一份玩法的第二意见，防只看一家走偏 | Godot 看不懂的地方拿它对一遍 |
| Spine 数据格式（含骨骼、插槽、蒙皮、IK、权重） | 骨头数据长啥样 | 只认 JSON 结构，保证以后美术资源能直接用 |
| Tiled TMX 数据格式（含图块、图层、对象） | 瓦片地图数据长啥样 | 只认 TMX 结构，保证地图资源能直接用 |
| Basis Universal、KTX2、ASTC 公开格式（含 Khronos 采样器规范） | 压缩图和远处防闪 | 只看头和块怎么摆，解码找现成 Go 库包一层 |
| Fix Your Timestep（Gaffer 经典做法） | 固定步长加渲染插值 | 逻辑固定步长跑，画画按余量插值，快慢机器动作一致 |

## 还差的另一半：工业级支撑（8 块）

画画那 9 块只是半张表。工业级是拿来做项目、养美术、长期跑的，还差下面 8 块。现状一句话：界面那套底子有，游戏那套活没有。`ui/animation` 只有界面动画的控制器和曲线，`ui/input` 加手势加焦点是给按钮输入框用的，`ui/scheduler` 加 `ui/raster` 是按需画界面的，不是游戏固定步长的。音频、碰撞、寻路、存档、脚本这些包名全搜不到，就是没有。

### 10. 世界和对象

游戏里的关卡、人、怪、箱子怎么放、谁是谁的爹、开局读档怎么一次装出来，游戏叫实体和场景。

- 10.1 实体挂件：一个东西挂几个能力（位置、贴图、碰撞、脚本）。现在界面那棵 `RenderObject` 树只能画静的，不管游戏命。要做：实体号、挂件增删、父子变换继承。
- 10.2 预制和场景：策划摆好的东西存成文件，开局一次装出来。现在没有。要做：预制文件、场景文件、加载存盘。
- 10.3 生命周期：出生、激活、休眠、销毁，跨关不漏不炸。现在没有。要做：统一开关，销毁清资源。

### 11. 主循环和时间（`game/step` 的另一半）

逻辑跑多快、卡了追不追、暂停慢放加速怎么做。

- 11.1 固定步长：快慢机器动作一致。画画那块 8.1 只管插值，这里管逻辑节拍。要做：逻辑按固定步长跑，画画按余量插值（主抄 Fix Your Timestep）。
- 11.2 时间控制：暂停、慢放、加速、时缩放。现在 `Tick(dt)` 没有这套。要做：全局时间倍率，分层时间（世界停了界面还能动）。

### 12. 资源管线

美术图、地图、骨头、声音怎么进来、压缩、拼大图、边玩边下、用完谁扔，游戏叫资产管线。

- 12.1 资产管理：异步加载、引用计数、用完释放、预算淘汰，外加依赖跟踪（骨头换图、地图换块知道重载谁）、导入管线、版本哈希。现在只有单张读 PNG、JPG、WebP，有图片池和显存清理，没有整套账。要做：统一资产号，后台加载，前台不卡，依赖对得上。
- 12.2 图集打包：小图拼大图，省提交次数。现在只有运行时按块取，没有打包工具。要做：离线打包工具，加旋转留白。
- 12.3 热重载：美术改完不用重启游戏直接看。现在没有。要做：文件一变就重载，只换那一块。

### 13. 输入映射

手柄摇杆、多点触摸、按键改键、连招缓存、录像回放，游戏叫动作映射。

- 13.1 动作映射：跳跃、攻击这种动作，不绑死某个键，外加死区、手柄震动、捏放手势。现在界面输入只认点和键，不认动作。要做：动作名对多键多柄多触摸，可改键，死区可调。
- 13.2 输入缓冲：连招按快了不丢，多点触摸不打架。现在没有。要做：按压队列，触摸号跟踪。

### 14. 碰撞物理

地面斜坡、跳和落、谁撞谁、触发开门。2.5D 用 2D 碰撞就够，不用上大物理引擎。

- 14.1 碰撞盒：谁和谁撞、撞完弹开还是穿过，外加射线检测（跳跃探头、子弹判线）、移动平台。现在完全没有。要做：盒和圆，层和分组，触发器，射线，移动平台带着走。
- 14.2 平台和斜坡：站得住、滑不滑、跳得上。现在没有。要做：斜坡站立，平台单向上跳。

### 15. 音频

脚步远近大小、音乐切换、混音不爆。

- 15.1 2D 位置音：离得远声音小，左右有别。现在没有。要做：位置、范围、衰减。
- 15.2 音乐和混音：切换淡入淡出，多声不炸，外加总线、闪避（爆炸压音乐）。现在没有。要做：音乐栈，混音上限，总线闪避。

### 16. 存档和设置

存盘读盘、画质档、语言。

- 16.1 存档：进度、背包、关卡星，外加槽位、版本迁移。现在没有。要做：存档文件，版本号，坏档不崩，升版能迁。
- 16.2 画质分档：高低端机器各跑各的。现在没有。要做：高、中、低三档，粒子、灯光、分辨率跟档走。

### 17. 工具和看病

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
game/anim/statemachine.go  动作混合状态机（对 4.3，主抄 Godot AnimationTree）
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
render/m4_extensions.go    修梯形 CPU 回退真透视采样（对 1.4）
render/vertices.go         修 CPU 渐变真渐变（对 9.2）
render/internal/gpu/image_pipeline.go  加按图采样开关（对 3.2）
```

暂缓不动（动了全崩，先放着）：

```text
6 数矩阵改透视（对 1.1 底层）
全链路浮点（对 9.1）
```

实施顺序：先 `camera` 加 `sprite` 加 `anim`（零影响，游戏马上好看），再 `world` 加 `step` 加 `asset`（游戏才能跑起来），再 `input` 加 `physics`（人物才能走和跳），再 `particle` 加 `tilemap` 加 `light`，再 `audio` 加 `save` 加 `debug` 加 `tools`（养得住），最后骨头和压缩图（要先定数据格式）。

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
| 1 镜头远近 | 近大远小推拉震动视差 | `game/camera` | 没有，要新做 |
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

看法一句话：上一波没绿，下一波别开。同一波里标【可并行】的可以一起开，标【串行】的必须一个做完再做下一个。

修订2026-09-15：按能力表每行自己的前置重排，52行全在（8.1/11.1合测，R3/R4/R5是render底步骤不占能力行）。原来P1b长串拆成并行波，1.1放回已完成，2.3/2.4/7.1/3.1提前到只靠底座那波。

```text
W0 已绿（不用重做）
  0.0 core ＋ 1.1上层透视投影【已完成】

W1 只靠底座【波内全可并行，可一起开】
  1.3a镜头纯算，1.3b视差画出（只靠0.0+1.1，1.1已绿所以可进本波），1.4/R1梯形CPU，
  2.3排序，2.4序列帧，3.1压缩图认头，4.1变速，7.1瓦片斜45度，
  8.1+11.1固定步长合测，8.2复用池，13.1动作映射，14.1a碰撞体纯算，
  15.1位置音，15.2音乐混音，16.1存档
  注：R1在本波，R2必须等R1绿，所以9.2/R2不在本波。

W2 靠W1【波内全可并行】
  R2/9.2 CPU渐变【串行：等R1】，4.2时间轴（等4.1），11.2时间倍率（等8.1），
  13.2输入缓冲（等13.1），7.2分区（等7.1+1.3a视口），
  14.1b射线移动平台（等14.1a+7.1），14.2平台斜坡（等14.1a+7.1），12.1资产管理（等3.1）

W3 靠W2【波内全可并行】
  R3采样按图开关【串行：等R2】，10.1实体挂件（等8.1+11.2），7.3远近切换（等7.2），
  12.2拼大图（等12.1），12.3热重载（等12.1），3.3边玩边加载（等12.1）

W4 靠W3【波内全可并行】
  R4图集加字段【串行：等R3】，2.1合批（等R3），3.2防闪开关（等R3），
  10.2预制场景（等10.1），10.3生命周期（等10.1）

W5 靠W4【波内全可并行】
  R5深度分支【串行：等R4】，2.2图集旋转染色（等R4），8.3动态更新（等2.1），
  5.1粒子发射（等2.1），5.3扭曲（等2.1），5.4发光暗角调色（等2.1）

W6 靠W5【波内全可并行】
  1.2前后遮挡（等R5），4.3骨骼（等12.1+2.2），5.2显卡粒子拖尾（等5.1），
  5.5材质钩子（等5.4），6.1灯光夜色（等5.4）

W7 靠W6【波内全可并行】
  4.4状态机（等4.2+2.4+4.3），6.2法线（等6.1），6.3影子（等6.1）

W8 靠W7（P3全过）【波内2个可并行】
  17.2性能崩溃 ＋ 17.1编辑器预览【可并行】

W9 靠W8【串行收尾】
  16.2画质分档（等17.2）

P5冻结不动：9.1高动态
```

分期关门条件（每波必须全绿才进下波，备注里标原P分期，状态表那列不动）：

| 波 | 装啥（能力号） | 并行标记 | 关门线 |
|---|---|---|---|
| W0 | 0.0底座，1.1透视 | 已绿 | 单测过，无窗口要求 |
| W1 | 1.3a，1.3b，1.4/R1，2.3，2.4，3.1，4.1，7.1，8.1+11.1合测，8.2，13.1，14.1a，15.1，15.2，16.1 | 波内全可并行，可一起开 | 单测过，无窗口要求（原P0+P1a首步+P2先头部） |
| W2 | R2/9.2，4.2，11.2，13.2，7.2，14.1b，14.2，12.1 | 波内全可并行；R2串行等R1 | 单测过，老路金图不红 |
| W3 | R3，10.1，7.3，12.2，12.3，3.3 | 波内全可并行；R3串行等R2 | 单测过，主能力有独立真窗意向（窗随P2建） |
| W4 | R4，2.1，3.2，10.2，10.3 | 波内全可并行；R4串行等R3 | 每个能力有单测，显卡和CPU画出来只差抗锯齿 |
| W5 | R5，2.2，8.3，5.1，5.3，5.4 | 波内全可并行；R5串行等R4 | 同上，两边齐 |
| W6 | 1.2，4.3，5.2，5.5，6.1 | 波内全可并行 | 真窗看得出盖对、动作顺、气氛对 |
| W7 | 4.4，6.2，6.3 | 波内全可并行 | 真窗看切换脆、立体、影子对 |
| W8 | 17.2，17.1 | 2个可并行 | 长跑在线，崩留日志，预览所见即所得 |
| W9 | 16.2 | 串行收尾，等17.2 | 三档帧率有数，切档不闪 |
| 冻结 | 9.1 | 不开 | 先不动 |

一句话记：P0 算数，P1 画出来，P2 立起来，P3 有气氛，P4 养得住，P5 先放着。

## 每个能力全场景测试和完成状态

一句话：开工以能力为单位，一个能力一行，按所属模块加分期加前置加实现方式指定的地方写，接口先冻，测试与真窗按命名来，6种场景加严版验收全过才算完成，少一样就是没完。

6 种场景是啥：

- A 功能对：正常输入，输出数对得上。
- B 边界错：空、零、超大、坏数据进来，不崩不卡死。
- C 两边齐：显卡和 CPU 画出来只差抗锯齿边缘，不能一个梯形一个方块、一个渐变一个纯色。不画画的包（比如存档音频）这条记不适用。
- D 跑得动：数量、帧率、显存有数，比如多少精灵多少帧。
- E 跑得久：长时间跑显存不涨，不漏，坏档坏图不崩。
- F 真窗认：主能力有独立真窗，先自动判留下 JSON，再留着给人看。纯算数的包（比如变速曲线）只留离屏对比，不强制真窗。

状态只有三种：未开工、进行中、已完成。写已完成必须带日期加证据（测试文件加真窗名），不许口头说完。

| 编号 | 能力 | 所属模块 | 分期 | 前置 | 接口约定 | 实现方式 | 测试与真窗 | 全场景测啥 | 状态 |
|---|---|---|---|---|---|---|---|---|---|
| 0.0 | 共用底座core | game/core（7文件） | P0最前 | 无（已绿，可开工） | Vec2/Rect/Color/Time/Rand/AssetID/Result/Version | game新包，render老类型边界转 | 测core_test；窗免 | A数对，B空零超大不崩，D万次耗时有数，E长跑不漂 | 已完成（2026-09-14；game/core/core_test.go 16项全PASS＋CGO_ENABLED=0可构建；D万次约10.5ns/op；窗免-纯算数，边界往返等价即离屏依据；收敛2026-09-14：注释写清三处有意差异＋台账对账断言，金数据未动） |
| 1.1 | 上层透视投影 | game/camera/project.go | P1b | 0.0 | Project(world)→screen，DepthToScale | game新包只算数，算好喂现有梯形三角 | 测camera_project_test＋离屏；窗免 | A 算出的屏坐标对，B 深度零和负不崩，C 两边齐，D 56 段路全量重投影帧率有数，E 环线跑千圈闭合，F 离屏对比 | 已完成（2026-09-14；game/camera/camera_project_test.go 6项全PASS＋CGO_ENABLED=0可构建；D56段路114点约48.7ns/op远窄于近单调收敛；E千圈逐位一致＋往返1e-9闭合；C边界往返无损＋重放一致即两边同数；F离屏金比对project_cases.json梯形近宽远窄；窗免-纯算数；收敛2026-09-15：有限数判断收拢＋直接算式＋单测命名结构与四角/读文件去重，6项仍全PASS，金数据未动） |
| 1.2 | 前后遮挡 | game/sprite/ysort.go（深度扩展）+ render | P1b | core＋R5深度分支 | SetDepth＋Sort()，远先近后 | game排序为主，需动显卡走隔离新分支 | 测ysort_depth_test；窗game_sprite--case=depth | A 远先近后盖对，B 同深不闪，C 两边齐，D 百个遮挡帧率有数，E 长跑顺序不乱，F 真窗看盖对 | 未开工 |
| 1.3a | 镜头纯算 | game/camera/camera.go | P0 | 0.0 | Camera{pos,zoom,rot,shake,limit}＋View() | game新包只算数，不碰渲染 | 测camera_test＋离屏；窗免 | A位置缩放角度震动限位平滑锚点数对，B零负钳住，D万次耗时有数，E长跑不漂 | 未开工 |
| 1.3b | 视差画出 | game/camera/parallax.go＋project.go | P1b | 0.0＋1.1 | Parallax＋Project(world)→screen | game新包算好喂梯形三角 | 测parallax_test；窗game_camera--case=follow | A错开重复镜像对，B零负不崩，C两边齐，D全屏帧率有数，E跑千米不飘无缝，F真窗看跟随长路 | 未开工 |
| 1.4 | 梯形贴图对齐 | render/m4_extensions.go | P1a-R1 | 0.0 | DrawImageQuad透视回退，错报错 | render小修CPU回退 | 测quad_cpu_test＋离屏；窗game_quad | A 梯形贴对，B 退化四边形不崩，C 两边齐（重点），D 单图帧率有数，E 长跑不漏，F 真窗加离屏对比 | 未开工 |
| 2.1 | 精灵合批 | game/sprite/batch.go | P1b | 0.0＋R3采样开关 | Batch{Add,Flush}，同图一次交 | game新包攒批，只调现有接口 | 测batch_test；窗game_sprite--case=batch千树 | A 同图一批画对，B 空批零尺寸跳过，C 两边齐，D 千个精灵帧率调用次数有数，E 长跑不涨，F 真窗看千树 | 未开工 |
| 2.2 | 图集旋转染色翻转轴心 | game/sprite/atlas.go + render扩展 | P1b | 0.0＋R4图集扩展 | AtlasSprite＋rot/flip/pivot/tint/filter | render加新分支+game包，老路不动 | 测atlas_test；窗game_sprite--case=rot | A 角度颜色翻转轴心单图过滤对，B 空块跳过，C 两边齐，D 百块帧率有数，E 长跑不漏，F 真窗看转和换色转身脚底对齐 | 未开工 |
| 2.3 | 按高低排序加层号分层 | game/sprite/ysort.go | P1b | 0.0 | YSort(feetY)＋Layer{world,fx,ui} | game新包只算数 | 测ysort_test；窗game_sprite--case=ysort | A 脚底高度加层号排对、界面层不被盖，B 同高稳定不闪，C 两边齐，D 百个排序耗时有数，E 长跑不乱，F 真窗看前后盖特效不飘界面 | 未开工 |
| 2.4 | 序列帧多动作库 | game/sprite/flipbook.go | P1b | 0.0 | Flipbook{Play,Update,OnEvent}＋动作库 | game新包只算数 | 测flipbook_test；窗game_sprite--case=anim | A 帧号时长循环回调多套动作切换对，B 缺帧不崩，C 两边齐，D 多路播放帧率有数，E 长跑不漂，F 真窗看待机跑跳切换 | 未开工 |
| 3.1 | 压缩图 | game/tex/compressed.go | P2长链首 | 0.0 | LoadKTX2/Basis→上传，坏报Result错 | game新包独立解码，不碰老8位路 | 测compressed_test＋离屏；窗免 | A 解码上传对，B 坏文件报错不崩，C 画出来和原图差在容差内，D 大图显存有数，E 反复加载释放不涨，F 离屏对比 | 未开工 |
| 3.2 | 远处防闪开关 | game/tex/mipmap.go + render/image_pipeline.go | P1b | 0.0＋R3采样开关 | SetFilter(near/far,aniso)，按图开关 | render按图开关+game开关 | 测mipmap_test；窗game_tex--case=far | A 近清远稳，B 超小尺寸不崩，C 两边齐（重点），D 远景帧率有数，E 长跑不抖，F 真窗看远树 | 未开工 |
| 3.3 | 后台边玩边加载 | game/tex/stream.go + game/asset | P2 | 0.0＋12.1 | Stream{Request,Poll}，缺占位 | game新包后台链 | 测stream_test；窗game_tex--case=stream | A 到边加载不卡，B 缺图占位不崩，C 两边齐，D 加载时帧率有数，E 跑大地图不涨，F 真窗看边走边出 | 未开工 |
| 4.1 | 变速曲线 | game/anim/easing.go | P0 | 0.0 | Ease(t)→v，曲线库冻名 | game新包只算数 | 测easing_test＋离屏；窗免 | A 曲线值对，B 时间越界钳住，C 不适用，D 万次求值耗时有数，E 长跑不漂，F 离屏对比 | 未开工 |
| 4.2 | 关键帧时间轴加事件軌 | game/anim/timeline.go | P1b | 0.0＋4.1 | Timeline{AddKey,Sample,OnEvent} | game新包只算数 | 测timeline_test；窗game_anim--case=tl | A 插值循环事件回调（出刀光播声调方法显隐）对，B 空轨不崩，C 两边齐，D 多轨帧率有数，E 长跑不漂，F 真窗看动作事件准时 | 未开工 |
| 4.3 | 骨骼蒙皮对齐 Spine | game/anim/skeleton.go | P2长链 | 0.0＋12.1＋2.2 | Skeleton{Pose,Skin,IK}，认Spine子集 | game新包+认Spine JSON | 测skeleton_test；窗game_anim--case=sk真资源 | A 骨头插槽皮肤网格权重绘制次序IK约束物理惯性对，B 缺骨不崩，C 两边齐，D 单角色帧率有数，E 长跑不错位，F 真窗看走跑跳真资源 | 未开工 |
| 4.4 | 动作混合状态机 | game/anim/statemachine.go | P2长链 | 4.2＋2.4＋4.3 | State{CanTo,Blend}，混合时长 | game新包只算数 | 测statemachine_test；窗game_anim--case=fsm | A 切换混合对，B 非法切换不崩，C 两边齐，D 切百次耗时有数，E 长跑不卡死，F 真窗看切换 | 未开工 |
| 5.1 | 粒子发射形状乱流子发射 | game/particle/emitter.go + cpu.go | P3 | 0.0＋2.1 | Emitter{Spawn,Update}＋形状乱流子发射 | game新包+靠sprite画 | 测emitter_test；窗game_particle--case=fire | A 数量速度寿命重力颜色形状乱流子发射对，B 零发射不崩，C 两边齐，D 千粒子帧率有数，E 长跑不涨，F 真窗看火锥烟飘 | 未开工 |
| 5.2 | 显卡粒子拖尾宽窄渐变 | game/particle/gpu.go + trail.go | P3 | 5.1＋render新管线 | GPUPool＋Trail{width,grad,joint} | game新包+新管线隔离做 | 测gpu_trail_test；窗game_particle--case=trail | A 轨迹宽窄颜色接头对，B 空轨不崩，C 两边齐，D 数千点帧率有数，E 长跑不涨，F 真窗看刀光由宽到窄 | 未开工 |
| 5.3 | 扭曲热浪 | game/fx/warp.go | P3 | 0.0＋2.1 | Warp{strength,noise}，偏移采样 | game新包+新中间图隔离做 | 测warp_test；窗game_fx--case=warp | A 偏移方向对，B 强度零不崩，C 两边齐，D 全屏帧率有数，E 长跑不花，F 真窗看水晃 | 未开工 |
| 5.4 | 发光暗角调色 | game/fx/bloom.go + vignette.go + lut.go | P3 | 0.0＋2.1 | Bloom/Vignette/LUT，接滤镜链 | game新包+新中间图隔离做 | 测bloom_lut_test；窗game_fx--case=grade | A 发光暗角表对，B 空表不崩，C 两边齐（重点），D 全屏帧率有数，E 长跑不漏，F 真窗看气氛 | 未开工 |
| 5.5 | 自定义材质钩子 | game/fx/custom.go | P3 | 0.0＋5.4 | Custom{name,params}，只碰游戏层 | game新钩子只碰游戏层 | 测custom_test；窗game_fx--case=custom | A 钩子进出数对、只碰游戏层，B 坏着色器报错不崩主路，C 两边齐（CPU 明确降级标记），D 单钩子全屏帧率有数，E 反复装卸不漏，F 真窗看描边溶解 | 未开工 |
| 6.1 | 2D 灯光灯片分层夜色 | game/light/light.go | P3 | 0.0＋5.4 | Light{pos,range,cookie}＋分层＋夜色 | game新包盖在fx之后 | 测light_test；窗game_light--case=torch | A 位置范围衰减灯片强度颜色分层照全局夜色对，B 零灯不黑屏，C 两边齐，D 多灯帧率有数，E 长跑不闪，F 真窗看手电只照人不照背景 | 未开工 |
| 6.2 | 法线受光 | game/light/normal.go | P3 | 6.1 | NormalMap＋光向，受光系数 | game新包盖在fx之后 | 测normal_test；窗game_light--case=normal | A 受光方向对，B 缺法线不崩，C 两边齐，D 单图帧率有数，E 长跑不花，F 真窗看立体 | 未开工 |
| 6.3 | 投影遮挡 | game/light/shadow.go | P3 | 6.1 | Occluder＋Shadow，挡变暗 | game新包盖在fx之后 | 测shadow_test；窗game_light--case=shadow | A 影子方向对，B 无挡不崩，C 两边齐，D 多挡帧率有数，E 长跑不错位，F 真窗看影子 | 未开工 |
| 7.1 | 瓦片斜45度对象自动拼 | game/tilemap/tilemap.go + iso.go | P2 | 0.0 | Tilemap＋对象层＋自动拼，认TMX子集 | game新包只算数+认TMX | 测tilemap_test；窗game_tilemap--case=map | A 格子号对象摆位碰撞导航自动拼对，B 缺块占位不崩，C 两边齐，D 大图帧率有数，E 反复进出不涨，F 真窗看地图摆怪路沿自接 | 未开工 |
| 7.2 | 分区加载剔除 | game/tilemap/chunk.go | P2 | 7.1＋1.3视口 | Chunk{Load,Unload}＋视口剔除 | game新包只算数 | 测chunk_test；窗game_tilemap--case=chunk | A 镜头内外装卸对，B 跳跃镜头不崩，C 两边齐，D 快移帧率有数，E 跑大圈不涨，F 真窗看边走边装 | 未开工 |
| 7.3 | 远近切换 | game/tilemap/lod.go | P2 | 7.2＋1.3 | LOD{near,far}，临界回滞 | game新包只算数 | 测lod_test；窗game_tilemap--case=lod | A 远简近精切换对，B 临界不闪，C 两边齐，D 切换耗时有数，E 长跑不抖，F 真窗看远近 | 未开工 |
| 8.1 | 固定步长补间 | game/step/fixed.go | P0 | 0.0 | Fixed{Step,Interp}，步长冻死 | game新包只算数 | 测fixed_test＋离屏；窗免 | A 快慢机动作一致，B 大步钳住，C 不适用，D 步数耗时有数，E 长跑不漂，F 离屏对比 | 未开工 |
| 8.2 | 路径顶点复用 | game/step/pool.go | P0 | 0.0 | Pool{Get,Put}，路径顶点复用 | game新包纯工程池化 | 测pool_test＋离屏；窗免 | A 复用画对，B 空池不崩，C 两边齐，D 复用率内存有数，E 长跑不涨，F 离屏对比 | 未开工 |
| 8.3 | 动态局部更新 | ui/scene + game/step（动态层） | P2 | 0.0＋2.1 | DirtyLayer，动哪更哪 | scene动态层+game包 | 测dirty_test；窗game_step--case=dirty追车 | A 动哪更哪，B 全动退整屏不崩，C 两边齐，D 脏块数帧率有数，E 长跑不漏，F 真窗看追车 | 未开工 |
| 9.1 | 高动态亮度流程 | — | P5冻结 | — | 冻结不动 | 暂缓不动 | 冻结 | 暂缓，先不动，状态冻结 | 冻结 |
| 9.2 | CPU 回退画质 | render/vertices.go | P1a-R2 | 0.0 | CPU真渐变，降级标记 | render小修CPU渐变 | 测vertices_cpu_test＋离屏；窗免 | A 渐变真渐变，B 空 mesh 跳过，C 两边齐（重点），D 回退帧率有数，E 长跑不漏，F 离屏对比 | 未开工 |
| 10.1 | 实体挂件 | game/world/entity.go | P2 | 0.0＋8.1/11.2 | Entity＋Comp＋父子变换继承 | game新包只算数 | 测entity_test＋离屏；窗免 | A 父子变换继承对，B 删爹不崩，C 不适用，D 千实体耗时有数，E 反复生灭不涨，F 离屏对比 | 未开工 |
| 10.2 | 预制场景 | game/world/prefab.go + scene.go | P2 | 10.1＋1.3视口 | Prefab/Scene{Load,Save}，认场景JSON | game新包+认预制场景文件 | 测scene_test；窗game_world--case=open | A 摆好装出来对，B 缺文件报错不崩，C 两边齐，D 大场景加载时长有数，E 反复进出不涨，F 真窗看开局 | 未开工 |
| 10.3 | 生命周期 | game/world/scene.go | P2 | 10.1 | Spawn/Sleep/Dispose，清资源 | game新包只算数 | 测lifecycle_test＋离屏；窗免 | A 生灭休眠对，B 重复销毁不崩，C 不适用，D 万次生灭耗时有数，E 跨关不漏，F 离屏对比 | 未开工 |
| 11.1 | 逻辑步长 | game/step/fixed.go（同8.1） | P0 | 同8.1 | 同8.1合测 | game新包只算数，同8.1合测 | 同8.1合测 | 同 8.1，合测 | 未开工 |
| 11.2 | 时间倍率暂停 | game/step/time.go | P0 | core＋8.1 | TimeScale＋Pause，分层时间 | game新包只算数 | 测time_test；窗game_step--case=pause | A 暂停慢放加速对，B 倍率零负钳住，C 不适用，D 切换耗时有数，E 长跑不漂，F 真窗看暂停 | 未开工 |
| 12.1 | 资产管理依赖版本 | game/asset/asset.go | P2长链 | 0.0＋3.1 | Asset{Load,Ref,Unload}＋依赖版本 | game新包后台链 | 测asset_test＋离屏；窗免 | A 异步引用计数依赖跟踪版本哈希对，B 缺资产占位不崩，C 两边齐，D 百资产内存有数，E 反复加载不涨，F 离屏对比 | 未开工 |
| 12.2 | 图集打包 | game/asset/atlaspack.go | P2 | 12.1 | AtlasPack工具，拼图出JSON | 离线工具+game包 | 测atlaspack_test＋离屏；窗免 | A 拼图号对，B 超大图报错，C 画出来差在容差内，D 打包时间有数，E 重复打包稳定，F 离屏对比 | 未开工 |
| 12.3 | 热重载 | game/asset/hotreload.go | P2 | 12.1 | Watch＋Reload，只换那块 | game新包文件监听 | 测hotreload_test；窗game_asset--case=reload | A 一改就换，B 坏文件不崩，C 两边齐，D 重载耗时有数，E 反复改不涨，F 真窗看改完即看 | 未开工 |
| 13.1 | 动作映射改键死区震动 | game/input/action.go | P0 | 0.0 | Action＋改键＋死区震动 | game新包只算数 | 测action_test；窗game_input--case=remap | A 动作触多键死区震动手势对，B 空绑不崩，C 不适用，D 百次输入耗时有数，E 长跑不丢漂移可调，F 真窗看改键摇杆不漂 | 未开工 |
| 13.2 | 输入缓冲多点 | game/input/buffer.go | P0 | 13.1 | Buffer＋多点跟踪 | game新包只算数 | 测buffer_test；窗game_input--case=combo | A 连招缓存多点跟踪对，B 断触不崩，C 不适用，D 压力输入有数，E 长跑不乱，F 真窗看连招 | 未开工 |
| 14.1a | 碰撞体纯算 | game/physics/body.go | P0 | 0.0 | Body＋层分组＋触发器 | game新包只算数 | 测body_test＋离屏；窗免 | A撞和触发对，B同位不崩，D百盒耗时有数，E长跑不穿 | 未开工 |
| 14.1b | 射线移动平台画出 | game/physics/body.go | P2 | 14.1a＋7.1 | Ray＋移动平台带着走 | game新包只算数 | 测body_ray_test；窗game_physics--case=hit | A射线平台对，B同位不崩，D百盒耗时有数，E长跑不穿，F真窗看探头子弹线 | 未开工 |
| 14.2 | 平台斜坡 | game/physics/platform.go | P2 | 14.1a＋7.1 | Slope＋单向平台，站滑跳 | game新包只算数 | 测platform_test；窗game_physics--case=jump | A 站滑跳单向上跳对，B 卡角不崩，C 不适用，D 长坡耗时有数，E 长跑不掉，F 真窗看跳 | 未开工 |
| 15.1 | 位置音 | game/audio/positional.go | P0 | 0.0 | PosSound{pos,range}，远小近大 | game新包独立音频链 | 测positional_test；人工听 | A 远近左右对，B 无声不崩，C 不适用，D 多声 CPU 有数，E 长跑不爆，F 人工听 | 未开工 |
| 15.2 | 音乐混音总线闪避 | game/audio/music.go | P0 | 0.0 | Music＋Bus＋Duck，爆炸压音乐 | game新包独立音频链 | 测music_bus_test；人工听 | A 切换淡入淡出总线闪避对，B 缺曲不崩，C 不适用，D 混音上限有数，E 长跑不爆，F 人工听爆炸压音乐 | 未开工 |
| 16.1 | 存档槽位迁移 | game/save/save.go | P0 | 0.0 | Save{slot,ver,Migrate}，坏不崩 | game新包文件存读 | 测save_test＋离屏；窗免 | A 存读槽位版本迁移对，B 坏档不崩，C 不适用，D 大档时长有数，E 反复存读不坏，F 离屏对比 | 未开工 |
| 16.2 | 画质分档 | game/save/quality.go | P4 | 17.2 | Quality{高/中/低}跟档走 | game新包跟档走 | 测quality_test；窗game_save--case=q123 | A 高中低跟档走，B 切档不闪崩，C 两边齐，D 各档帧率有数，E 长跑不掉档，F 真窗看三档 | 未开工 |
| 17.1 | 编辑器预览 | tools/levelprev + particleprev | P4 | P3全过 | 关卡/粒子预览，所见即所得 | tools新工具 | 人工看预览；窗tools预览 | A 摆关调粒子所见即所得，B 空工程不崩，C 两边齐，D 大关卡打开时长有数，E 反复开不涨，F 人工看 | 未开工 |
| 17.2 | 性能崩溃帧分解 | game/debug/stats.go | P4 | P3全过 | Stats{帧/次/存/分解}＋上报 | game新包只出数 | 人工看报表＋长跑在线 | A 帧率画次显存帧分解过绘编译耗时出数对，B 无数据不崩，C 不适用，D 上报耗时有数，E 长跑在线，F 人工看报表定位到谁吃时间 | 未开工 |

规矩：这张表是唯一状态源，做完一项就把那行改成已完成，写清日期、测试文件、真窗名。口头说完不算。

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

## 验收怎么算过（严版，全场景覆盖）

一句话：数没写死、环境没锁、跑法没定、图没管住、坏路没测，都不算过。

1. 数写死（每个能力必须填过线，空着不许标绿）：
- 帧：release 包，目标 60 帧，帧时间看 p95 和 p99，不看平均糊弄。掉帧（超 20 毫秒）每分钟不许超 3 次。
- 量：精灵千个、粒子千点、碰撞百盒、实体千个为起步量，帧率和调用次数一起记。
- 存：显存和内存长跑 2 小时，涨不许超 5%，漏一次就不算过。
- 像：显卡和 CPU 差只许抗锯齿边缘，像素差超 1% 或梯形变方块、渐变变纯色，直接打回。
- 音：多声不爆，切换淡入淡出不断崖，人工听加波形看。
2. 环境锁死（不写环境，数白测）：
- 写清显卡、驱动、分辨率、开不开垂直同步、是不是 release、有没有插电、烫没烫。
- 换一台机器，照样达标才算过，只在一台上过不算。
3. 跑法统一：
- 预热 5 秒，正式跑 3 遍，去头尾，取最差那遍判，不取最好看那遍。
- 长跑至少 2 小时，中间切一次后台、缩一次窗口，回来不崩不漏。
- 每个 `*_test.go` 单独跑，一个跑完再跑下一个，长跑单独给足 `-timeout`。
4. 图管住：
- 对比图全进 `testdata/`，测试只读文件，不许现编标准数据。
- 容差写死在测试里，改基准必须说清谁改的、为啥改、记哪天。
- 基准一花，先查代码，不许直接更新图装绿。
5. 坏路必测（上线崩的多半在这）：
- 缺图坏档坏骨头坏地图：占位加报错，不崩不卡死。
- 显存不够：走清理链，不闪退，记日志。
- 切后台、窗口最小、驱动掉线：回来能画，不黑屏。
6. 状态规矩：
- 这张状态表是唯一源头，做完一项改一行，带日期、测试文件、真窗名。
- 口头说完不算，门禁绿不等于洞修完。

## 窗口清单（名字先冻，窗没建不许标绿）

一句话：能力表里写的窗口名全是待建，`examples/`里现在一个都没有。空目录不算数，主能力必须独立窗，组合窗不许顶单能力。

规矩（照现有`ui_wr_*`架子，不重造）：
- 单窗一套：`wrkit`搭壳（顶栏加图例加身体加HUD），`wrgate`判门（present加fps加p95），`wrsoak`管像素探针加对比图。游戏窗照抄`ui_wr_r15_soak`这套，不另起架子。
- 跑法两步：先自动判（`RUN_SECONDS=xx`跑完收JSON判PASS/FAIL），再留窗给人看（不设RUN_SECONDS常驻，看操作收事件改标题，关窗出汇总JSON）。一次性跑完即关的不能顶人工窗。
- 对比图：全进各窗`testdata/`，容差写死，改基准记谁为啥哪天。

| 窗（待建） | 验哪个能力 | 自动判啥 | 人工看啥 |
|---|---|---|---|
| `examples/game_camera` | 1.3镜头跟随震动视差限位平滑 | 跟随不飘、限位钳住、长路无缝JSON | 推镜头跟，抖不晕 |
| `examples/game_quad` | 1.4梯形 | 梯形两边齐JSON | 梯形不变方块 |
| `examples/game_sprite` | 2.1合批、2.2图集、2.3排序、2.4序列帧、1.2遮挡（5个case） | 千树帧率、转染色对、盖对、切换对JSON | 树多不卡，转身盖对 |
| `examples/game_tex` | 3.2防闪、3.3后台加载 | 远树不抖、边走边出JSON | 远近都清 |
| `examples/game_anim` | 4.2时间轴、4.3骨头、4.4状态机 | 事件准时、真资源不错位、切换不卡JSON | 动作顺，切换脆 |
| `examples/game_particle` | 5.1发射、5.2拖尾 | 千粒子帧率、刀光宽窄JSON | 火像火，刀有锋 |
| `examples/game_fx` | 5.3扭曲、5.4后期、5.5钩子 | 全屏帧率、气氛JSON | 水晃光晕对味 |
| `examples/game_light` | 6.1灯、6.2法线、6.3影子 | 手电只照人、影子对JSON | 夜里看得清 |
| `examples/game_tilemap` | 7.1瓦片、7.2分区、7.3远近 | 摆怪对、边走边装、切换不闪JSON | 地图大走不丢 |
| `examples/game_step` | 8.3动态更新、11.2暂停 | 动哪更哪、暂停即停JSON | 追车顺，暂停灵 |
| `examples/game_world` | 10.2预制场景 | 开局装对JSON | 开局齐 |
| `examples/game_asset` | 12.3热重载 | 一改即换JSON | 改完即看 |
| `examples/game_input` | 13.1改键、13.2连招 | 改键即用、连招不丢JSON | 按着顺手 |
| `examples/game_physics` | 14.1碰撞射线、14.2平台 | 撞门准、跳得上JSON | 跳不穿，落得住 |
| `examples/game_save` | 16.2三档 | 三档帧率JSON | 切档不闪 |
| 音频15.1/15.2、纯算数（1.1/4.1/8.1/8.2/9.2/10.1/10.3/11.1/12.1/12.2/16.1）、3.1压缩 | 窗免 | 只留离屏对比加人工听（音频） | — |

## 组合大窗（单项全绿合起来红，最贵）

先只定一关：追车关`examples/game_stage_chase`，镜头跟随（1.3）加排序遮挡（2.3/1.2）加粒子拖尾（5.2）加瓦片分区（7.2）加动态更新（8.3）一起上，跑2分钟，帧率显存像素三数全过才算P3关门。第二关（夜战灯光加后期）等追车绿了再定。

## 长跑接谁（不重造）

- 架子照抄`examples/wrsoak/soak.go`（探针加对比图）配`examples/ui_wr_r15_soak/main.go`（300秒相位循环那套），游戏长跑只换场景不换架子。
- 数进`wrgate.BuildReport`同一套JSON，显存涨超5%、漏一次、切后台缩窗口回来黑屏，直接FAIL。

## 门禁挂分期（每期跑啥，谁判）

| 期 | 跑啥 | 判 |
|---|---|---|
| P0 | 各包`*_test.go`单跑，`core`先绿 | 单测全PASS，窗免 |
| P1 | 单测加新窗自动判（`RUN_SECONDS`收JSON），render三小修回归界面金图 | 两边齐，界面金图不红 |
| P2 | 单测加独立窗自动加人工，组合只跑追车前置（世界加瓦片） | 主能力窗绿 |
| P3 | 追车关2分钟加长跑2小时 | 帧率显存像素全过 |
| P4 | 全量回归加工具预览人工看 | 长跑在线，崩留日志 |

红了谁负责：游戏层红改game/tools，render主路红先回滚游戏分支，不动主路。

## 不在这份文档里做的

- 具体某个示例的画法、美术资源、关卡设计。
- 窗口、输入法、文字排版本身的问题，那是别的文档管的。
