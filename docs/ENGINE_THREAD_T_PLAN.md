# 三线重设计立项 T（唯一工作文档）

> 状态：2026-09-16 立项，未开工。细案在本文件里逐条展开，
> `ENGINE_FLUTTER_SKIA_ARCH` §5、`ENGINE_UI_RENDER_BASE` §22.2、
> `ENGINE_ARCH_OVERVIEW` §7 只留一句话指过来，不各记一份。
> E2/E3/E5 保留当基线；E1 稀疏包已撤销不再碰；示例测试冻结等三线稳后重构。

## 1. 背景一句话

button 真窗点按拖拽慢半拍——界面线程建包 + 画纹理（`scene.RasterizeDirty`，
`ui/embedder/pipeline_app.go` 约1176行）串行，下一批鼠标事件只能等整套跑完。
标准要三线各干各的，现状光栅线只管送（`ui/raster/Loop`）、不管画。
本库开发阶段无用户，组件示例可重写，先把基座三线写对，再修测试示例。

## 2. 三线分工（对齐 Flutter：平台 / 界面 / 光栅 / 图片各一条）

| 线 | 在哪跑 | 干什么 | 铁律 |
|----|--------|--------|------|
| 界面线 | `ui/embedder` `Run` 循环 + `ui/rendering` + `ui/scheduler` + `ui/input`/`gestures`/`focus`/`textinput` | 收原生事件、排版、录图、建 `FramePacket`，交完马上回事件循环 | 禁止 gpu Submit / 直接操作 Device；脏旗只在这条读写 |
| 光栅线 | `ui/raster/Loop`（锁系统线程）+ `ui/scene` 画与合成 + `render` 上下文 + `gpu` | 接包、画脏层纹理、合成、送屏，一次一包、新包顶旧包 | 禁止读可变 RO 树，只读封好包；`ui` 不直调 `gpu` |
| 图片线 | `ui/io` 解码池 | 解图片，不挡前两条，解完跳回界面线挂树 | 排版画画时不许同步解 |

## 3. 交接规矩

- `FramePacket`（`ui/scene/packet.go`）交出去即只读：根层指针共享 + 脏号小拷，不整树深拷（只许 `CloneShallow`）。
- 画画步骤里的大块头（路径深拷、字形、图片缓冲）建包时定死，光栅照着画，不回头问树。
- 脏旗只在界面线清；纹理缓存（`PictureTextureCache`）只在光栅线读写；显卡提交只在光栅。
- 背压：在途最多 2 包，满了扔掉没开画的旧包画新的，不 BLOCK 收事件。
- 关窗顺序：停光栅 → 等画完 → 关设备 → 拆窗。
- 快照读屏、指标计数全在光栅任务里串行做，不跨线程读。
- 纪律：`ui → render → gpu`，禁 cgo，禁示例里绕洞。

## 4. 分期（细案待逐条议）

- T0 基线冻结：E2/E3/E5 留基线，示例测试冻结，记基线数。
- T1 封包只读：交出去后不再改，单测锁“交后改树不影响已交包”。（`ui/scene` + `ui/rendering` 建包十几行）
- T2 搬画画出界面线：`RasterizeDirty` 搬进 `raster.FrameJob`，界面建完包即回循环。（`pipeline_app.go` 约40行 + `raster/loop.go` 十几行；延迟消失在这步）
- T3 合成送屏收尾：纹理缓存、损伤区、快照、指标串进光栅任务，`render` 上下文归光栅独占。（`ui/scene/textured.go` + `render/present_target.go` 调用点；`gpu` 不动）
- T4 图片线扶正：`ui/io` 工人数量与回跳规矩写死，排版画画里同步解码报错。
- T5 解冻示例测试：重构 `examples/kit/button` 事件与组件测试，不倒灌回引擎。

## 5. 门

每期 `ui/...` 逐文件跑（禁一次全量）；包相等 + 数诚实（E2 两道门）常挂；
真窗 `timeout 90 go run ./examples/kit/button -auto-only` 140 项全绿；
`CGO_ENABLED=0` 与 `ui→gpu` 禁令每期查；有一项不对回滚当期。

## 6. 修订

| 日期 | 说明 |
|------|------|
| 2026-09-16 | 立项：本文件为唯一工作文档，三处旧址改为指针 |
