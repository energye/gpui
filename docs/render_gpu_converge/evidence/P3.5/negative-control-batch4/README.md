# P3.5 批 4 负向对照（PathEffect）

依据：`evidence/P3.5/assessment.md` §门禁「PathEffect 摘 Sum 顺序→Compose 钉红」+「真钉每条挂载点」。
方法：逐条摘除实现（临时改代码）→ 对应钉必须红 → 还原 → 绿。所有还原后文件与
`/tmp/*.b4bak` 备份 `diff -q` 一致（日志里的 `restored-same` 由执行记录佐证）。

| # | 摘除点 | 摘法 | 红的钉 | 日志 |
|---|--------|------|--------|------|
| 1 | `composePathEffect.FilterPath` 作用顺序（inner-first → outer-first） | 交换 inner/outer 调用次序 | `TestComposeOrderPins`：verb 数 57 vs 手工链 96 | `01-mutate-compose-order-red.log`（基线 `00`、还原 `02`） |
| 2 | CPU 主挂载点 `expandStrokeToPathSpace`（Context.Stroke 链）effect 分支 | 条件置 `false &&` | `TestStrokePathEffectCPUPath`（effect≡Dash 字段）、`TestStrokePathEffectTakesPrecedenceOverDash`（Dash 泄漏进带 effect 的描边） | `03-mutate-cpu-mount-off-red.log`（还原 `04`） |
| 3 | 第二 CPU 挂载点 `SoftwareRenderer.Stroke` effect 分支 | 条件置 `false &&` | `TestStrokePathEffectSoftwareRendererMount`（新增钉） | `05-mutate-software-mount-off-red.log`（还原 `06`） |
| 4 | GPU 展开链缓存键里摘掉 effect 哈希（`dashHash: 0`） | 键塌缩 | `TestDashGeomCacheGetOrApplyEffect`：不同图案共用同一条目 | `07-mutate-cache-effect-hash-red.log`（还原 `08`） |
| 5 | GPU 挂载点 `StrokePath` effect 分支 | 条件置 `false &&` | `TestPathEffectGPUStrokeParity`：GPU 侧 changed 0/33600（effect 被静默忽略）+ 与 CPU 分类不再一致 | `10-mutate-gpu-mount-off-red.log`（基线 `09`、还原 `11`） |
| 6 | 自审发现的正确性洞（非摘除式）：值形态自定义效果共享一个缓存键 | 先加钉（原实现必红）→ 改哈希 0=无键 + 缓存不入库 | `TestDashGeomCacheUnkeyableEffectNeverSharesAnEntry` 红→绿 + `TestPathEffectHashIsParameterStable` 加钉 | `12-unkeyable-custom-effect-red-to-green.md` |
| 7 | 盖章行走密度护栏（对齐 Skia `MAX_REASONABLE_ITERATIONS`） | 常量 100000 → 3（临时） | `TestStampEffectsRejectPathologicalDensity`：正常密度被误伤即红 | `13-mutate-governor-cap-red.log`、`14-restore-green.log`；危害定量见 `15-walk-iteration-measure.{md,go}` |

## 过程中暴露的真洞（已补，非将就）

第一轮按 assessment 口径摘 `render/software_stroke.go` 的 effect 分支时，批 4 原有两条
CPU 挂载钉**全绿**——说明它们走的是 `Context.Stroke()` →
`stroke_user_expand.go` 这条链，`SoftwareRenderer.Stroke` 的设备空间挂载点当时**没有任何钉覆盖**
（假绿风险，属于「挂了但没人验证」的洞）。补 `TestStrokePathEffectSoftwareRendererMount`
（经 `SoftwareRenderer.Stroke` 直绘：DashPathEffect 与 `Stroke.Dash` 字段逐字节同图 + 与纯描边必须不同 +
1D stamp 必须改画），再摘同一分支才拿到 #3 的红。

另：`TestComposeOrderPins` 原先只比 verb 数，第一版反向断言写反（`ComposePathEffects(dash, corner)`
的正确语义就是 corner 先、dash 后，等于手工反序链，不是「必须不等」）；改为把两个操作数次序
**各自**钉成对应手工链的逐坐标相等（bit-for-bit），顺序摘除才必然红。
