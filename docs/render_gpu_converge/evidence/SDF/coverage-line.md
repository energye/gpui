# SDF 覆盖线：GPU shader 覆盖精度（2026-11-10，评估/登记）

## 现象与现状

- CPU 侧：批 4f 已改成**解析行积分**（圆/椭圆/圆角矩形行截面闭式解），
  `sdf_coverage_quality_test.go` 锁 avg≤2 / max≤8，实测 circle 0.26 / ellipse 0.02 / rrect 0.01。
- GPU 侧：`sdf_render.wgsl` 仍是**单点采样 + 0.75px smoothstep**，没有解析覆盖，
  曲线边缘与 CPU 不一致（P4.2 实测 AA 带 max 18，典型 9）。

## 为什么还没修（历史实证）

- 批 4f 试过两版 shader：解析行积分版（asin/atan2 原函数）与 32×32 子采样版。
  两版 **naga 编译均通过**，但本机 NVIDIA 驱动**整条 shader 静默拒绝**
  （画面全空、零错误输出）⇒ 按「shader 改动必须真窗像素验收」纪律回滚。
- 结论：不是数学问题，是本机驱动/管线对该 shader 形态的可见性不可用；
  再次尝试必须先解决「驱动拒绝时能被看见」的诊断前置。

## 候选路线（按风险排序）

1. **覆盖 LUT 纹理**：预计算 1D 距离→覆盖率表（256 项）上传为纹理，shader 只做
   查表 + 双线性插值。逻辑最简、无超越函数，绕过驱动对复杂分支的拒绝。
2. **4x MSAA 覆盖率**：用 4 个采样点的覆盖数近似（0/0.25/…/1），与 CPU 解析仍有
   量化差（±0.125），但比 smoothstep 好；需确认本机 MSAA 管线可用。
3. 维持 CPU 兜底：对精度敏感的形状强制 CPU（现 `SDFAccelerator` 的 cpuFallback 已是解析版）。

## 验收前置

- 真窗像素断言（不是只跑单测）：同一形状 GPU vs CPU 边缘带逐像素，avg/max 有上限；
- 驱动拒绝可见性：shader 建好后先用 1×1 探针画布确认非空再进真窗；
- Golden 重种只在该窗（预计仅 SDF 边缘带变化）。

## 2026-11-11 新增验证项：morphology 传播放大器（K2 改判归此）

- 证据：`evidence/P2.6/k2-erode-edge-dissection.md`——GPU 栅格化整数边界列
  AA 覆盖差（`(60,55)` cpu=255 vs gpu=237、`(59,55)` cpu=25 vs gpu=42，
  白矩形左缘），被 erode/dilate 的 min/max 窗沿半径传播（1px 输入差 →
  5px 传播带，404px、max17）。窗口语义两侧本就一致（resvg 口径逐条对齐）。
- 验收追加：覆盖线闭合后跑 erode(5)/dilate(5) 白矩形 CPU vs GPU 全图
  逐像素对拍，diff px 收敛到 0 或 AA 带 ≤2（对拍脚本形态见 K2 证据）；
  `ui_wr_p34_filters` A 卡探针 cpu 档应自然转绿（维持「cpu 组合非验收档」
  口径不放宽）。

- **2026-11-11 闭合实测**：erode(5) 404px/max 17 → 404px/**max 5**、dilate(5) 548px/max 5，
  逐像素全部落在形态学边缘 1px 环带（带外 0）。原「≤2」按单变量 LUT 下限修正为
  「带外 0 + 带内 ≤8」（固定 d 真值散布≈11/255 ⇒ 任何 d→coverage 单变量映射 max 下限≈5.5/255）。

## 状态

**登记，未动代码**（本机驱动限制 + 需真窗验收）。P4.2 的 AA 带容差（≤24）
是当前上界；本条闭合后应收紧到 ≤8。

> **2026-11-11 更新**：路线 A 已实现并走完验收链（见文末实现批），AA 带容差已 **24→8** 收紧；
> 上方「登记」文字保留为历史口径。

## 2026-11-11 前置①落地：驱动拒绝可见性探针（提交 `2d83cf72`）

`render/internal/gpu/sdf_shader_probe_test.go`（带 lib）：五复杂度签名
（control/trans asin+atan2/loop 32×32/array 动态索引/**lut 256×1 纹理查表**）
逐级显形 module→pipeline→draw→readback——**五腿全绿，单一构造签名均被驱动
接受，LUT 构造放行**；批 4f 的拒绝与 shader 总体形态相关（大函数+全量结构
混合），非单一构造。本探针成为 shader 改动的标准闸门。

## 2026-11-11 前置②：CPU 侧精度实验（数据定稿，临时脚本已删）

三方对拍（真值 = `exactCirclePixelArea` 解析行积分，圆，d∈[-1.5,1.5] 全角采样）：

| r | M0 现状(1.5px hermite) avg/max | M1 线性1px avg/max | 固定d真值散布 |
|---|---|---|---|
| 3 | 3.36 / **18.89** | 2.06 / 14.14 | 11.20 |
| 8 | 3.15 / 18.89 | 1.73 / 11.44 | 11.03 |
| 64 | 3.07 / 18.89 | 1.61 / 10.95 | 10.95 |

**三个决定性结论**：
1. M1 线性 1px 把 avg 3.07→1.61（达标 ≤2）但 max 11-14 **超 max≤8 门**——
   线性斜率与真值分布不匹配。
2. **固定 d 真值散布 ≈ 10.95-11.20，对半径无关** ⇒ 最优单变量函数（每 d 取
   真值中位数）的 max 误差下限 ≈ 5.5/255——**max≤8 数学可达但仅限中位数
   映射 LUT**（M1 的额外误差是斜率不匹配）。
3. 散布对形状/半径无关 ⇒ **一张 256 项 LUT 圆/椭圆/rrect 归一化通吃**；中位数
   映射无解析式，必须数值预计算（逐 d 在圆上采样解析真值取中位数）——正是
   「LUT 存 CPU 解析预计算表」的形态。预期收益 avg 3.07→≈1.0、max 18.89→≈6。

**实现批方案（路线 A 定稿）**：
1. Go 预计算 256 项 LUT（d∈[-1,1] 域，逐项圆周采样解析真值取中位数，uint8）；
2. `sdf_render.wgsl` aa 段 `aa_coverage` 改 LUT 纹理采样（线性插值硬件做）；
3. `sdf_render.go` LUT 纹理/采样器/绑定（管线布局加一组，RecordDraws/
   PassBindLedger 跟上）；
4. 验收链：Go 单测（LUT 表 vs 解析真值 avg≤2/max≤8 钉）→ 探针过 → 真窗
   SDF 窗像素断言 + 金图重种（边缘带预期变化）→ P4.2 对拍扩 erode/dilate
   传播腿（K2 验证项）→ AA 带容差 24→8 收紧。

> **落实口径（2026-11-11 实现批）**：表最终是 **1024 项 midrange**，不是本节写的 256 项中位数——
> 256 项在 r=3 陡降区量化误差超门；midrange 最小化 max 误差，中位数只最小化均值、留单侧尾。
> 实测数据见文末「实现批」一节。

## 2026-11-11 实现批：覆盖 LUT 落地 + 验收链（未提交前记录，代码在批内）

> 路线 A 落实。代码：`shaders/sdf_render.wgsl`（group(3) 声明 + `aa_coverage` 改查表）
> 、`render/internal/gpu/sdf_render.go`（`covLut*` 五字段 + `ensureCoverageLUT`：
> 1024×1 RGBA8 = 4KB 纹理 + filtering sampler + bind group + 第 4 个 BGL；`RecordDraws` SetBindGroup(3)；
> `destroyPipeline` 逆创建序释放，VRAM 账本由 hal CreateTexture/Destroy 双端自动记）。

**落实修正（相对「前置②」方案文本）**：表最终定为 **1024 项 midrange**，不是 256 项中位数——
256 项在 r=3 陡降区量化误差超门；midrange 最小化 max 误差，中位数只最小化均值、
留单侧尾（见 `render/sdf_coverage_lut.go` 注释与分层质量钉）。

### 验收数据（全部实测，日志 `impl/*.log`）

| 项 | 旧（0.75px smoothstep） | 新（1024 midrange LUT） | 判定 |
|---|---|---|---|
| 驱动可见性探针（五腿） | — | control/trans/loop/array/**lut** 全绿 | 放行（`TestSDFShaderDriverProbe`） |
| P4.2 对拍 SDF 形状 AA 带 max | 18 | **1**（circle_blur 等） | 达标；全场景 edge_max=6（阴影边），门禁 24→**8** 已收紧且 PASS |
| K2 白矩形 erode(5) CPU vs GPU | 404 px / max **17** | 404 px / max **5**，带外 **0** | 振幅收 3.4×，全部落 1px 环带 |
| K2 dilate(5) | 同族 | 548 px / max **5**，带外 **0** | 同上 |
| K2 raw 矩形边缘 | cpu(60,55)=255 vs gpu=237 | cpu=255 vs gpu=249 | 输入覆盖差 18→6 |
| 真窗 p35_rrect4 native | golden 2.9210%（预期变化） | **0.0000%**（重种后复验，logic 6/6，354 presents） | ✅ |
| 真窗 p35_rrect4 `GPUI_BACKEND=go` | M2 回归检查项 | **0.0000%**（351 presents） | ✅ 双后端同 shader 一致 |

**K2 验收口径修正（需拍板）**：原登记「diff px 收敛到 0 或 AA 带 ≤2」在单变量 LUT 下**数学不可达**——
固定 d 的真值散布对半径无关（≈11/255），任何 d→coverage 单变量映射的 max 误差下限 ≈5.5/255；
实测 404 px 全部在形态学边缘的 1px 环带内、带外 0、max 5。永久钉
`render/k2_morphology_propagation_test.go` 按「带外 0 + 带内 ≤8」断言（旧值 max 17 必红）。

### 全 SDF 触及窗回归（金图核对）

方法：`DrawCircle/DrawOval/DrawEllipse/DrawRRect` 全仓扫描 → 有金图门禁的窗逐个跑；
每个变化窗再做 **A/B 反证**（把 wgsl aa 段 Edit 回 smoothstep、同窗重跑、再 Edit 回来）——
下表「旧 shader」列= 同一棵树上旧 AA 段的 golden diff，用来证明差异确由 LUT 引起、金图本身没过期。

| 窗 | 旧 shader diff | 新 LUT diff | 处置 |
|---|---|---|---|
| ui_ant_verify（msaa1 / msaa4） | 0.0000% / 0.0000% | 2.0428% / 2.0428% | **重种**，双档复验 0.0000% |
| ui_wr_p34_filters | 0.0000% | 2.1256% | **重种**，复验 0.0000%（pixel 29/29） |
| ui_render_base_pathshape | 0.0000% | 1.1473% | **重种**，复验 0.0000%（logic 14/14、pixel 11/11） |
| ui_render_base_pathlow | 0.0000% | 1.6725% | **重种**，复验 0.0000%（pixel 7/7） |
| ui_render_base_imagepattern | 0.0000% | 1.3844% | **重种**，复验 0.0000%（logic 18/18、pixel 25/25） |
| ui_render_base_svg | 0.0000% | 0.2915% | **重种**，复验 0.0000%（SVG 烘焙恒 CPU 本身未变，差在卡片圆角边；该窗 screen-check 环境红既有） |
| ui_wr_p35_rrect4 | 2.9210%（旧金图 vs 新 LUT） | 重种后 0.0000% | 已在上表 |
| ui_render_base_recording | 2.5893% | 4.2275% | **不重种**：金图在旧 shader 下也已过期（既有红），另立 |
| ui_wr_ime_r1_editor | 0.0847% | 2.7563% | **不重种**：同上（旧 shader 已 0.0847% 既有漂移） |
| ui_render_base_sweepgradient | 24.1642% + logic 8/14 红 | 25.2975% | **不重种**：既有红（logic 与 SDF 无关） |
| engine/anim --case=fsm | 73.6752% | 73.6747% | **不重种**：既有红（动画相位/窗口金图过期，与 SDF 无关） |
| ui_base_multiwin_cap | 未跑 | 未跑 | 8 窗批量，按 GPU 安全纪律须隔离显示；本批不跑，登记待复验 |

**无内部回归证据**（逐像素，不用 mean）：对每个重种窗做旧/新快照差分，统计「7×7 邻域变化 ≤2」
的平坦区差像素——pathshape/pathlow/imagepattern/ant 内区 **0 枚**；p34 内区 36 枚、max **1**
（落在缓变 AA 带，pixel 29/29 全过）；全部窗的最外一列 254→255（旧 smoothstep 在窗口边界
留 1 级残差，新表补满）。即：差异只在边缘/边界覆盖带，无内容位移、无整片压暗。

### 门禁

`go build ./...`、`go vet ./render/...`、`go build/vet -tags nogpu ./render/...`、`go run ./scripts/apidoc` 全 rc=0；
`render/internal/gpu` SDF 家族（创建/销毁/重建/纹理幂等）ok；`TestSDFFilled*Coverage` 质量钉 ok。
