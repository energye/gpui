# render/gpu 收敛扫描：索引与读法

> 日期：2026-09-17。范围：`render/`、`gpu/` 只读扫描，不改代码。
> 方法：四路并行只读（导出统计、调用关系、池/重试对照、测试缺口），口径见各文件头。
> 诚实说明：grep 只能定位嫌疑，不能定逻辑；`^(var|const)` 单行式会漏掉块式声明；`descriptor_browser.go` 等双实现 build tag 会 double count；800 文件全量逐行未读完，本批是“全量统计 + 嫌疑抽查”，不是终审。

## 文档切分

| 文件 | 装什么 | 不装什么 |
|---|---|---|
| `doc_package_map.md`（已有） | 尺子对包，23 份半尺子 + 22 份非尺子 + E 节无尺子代码反向查规则 | 不装扫描结果 |
| `00_README.md`（本文件） | 索引、方法、口径、局限 | 不装明细 |
| `01_api_render.md` | render 各子包规模 + 建纹理/建缓冲/建设备/放资源等多入口嫌疑表 | 不定删留结论 |
| `02_api_gpu.md` | gpu 各子包规模 + rwgpu 经 webgpu 转发的调用关系 + 三个特别核对 | 不定删留结论 |
| `03_overlap_pools_retry.md` | 池/缓存/延迟释放/预算/重试全对照 + 重叠点（只写读到的） | 不写修复方案 |
| `04_dead_unwired.md` | 无人用/仅测试/目录过时（正反两面：成立的 + 反例） | 不删代码 |
| `05_test_gaps.md` | 测试数量、Skip 空转、只断没崩、金图容差位置 | 不判定假绿，只标信号 |

## 收敛方案与任务（已迁到 `docs/` 根目录）

> 本目录只保留 00–05 的只读审计；方案与任务台账是活文档，放 `docs/` 根目录，与其它 `ENGINE_*` 计划同级。

| 文件 | 装什么 | 不装什么 |
|---|---|---|
| `docs/ENGINE_RENDER_GPU_CONVERGE_PLAN.md` | 标准选型、去重/合并决策、包组织目标、双后端矩阵、缺失/半成品登记、批次与门禁 | 不含代码改动 |
| `docs/ENGINE_RENDER_GPU_CONVERGE_TASKS.md` | **任务状态唯一真源**：可执行任务清单、依赖、开工/完成标准、统一门禁、证据目录、即时同步规则、状态流水 | 不重复 PLAN 的方案论证 |
| `baseline/thresholds.md` | **P0.1 验收阈值与环境清单**：首帧、滚动帧率、p95/p99、显存上限、金图容差的写死数字与来源；基线机实测配置 | 不装逐窗金图清单（归 P0.2 `baseline/windows.md`） |
| `baseline/windows.md` | **P0.2 真窗与金图清单**：162 真窗 + 28 headless 逐窗金图/容差/运行档/平台后端/能力标签；PLAN §5/§6 能力→窗映射；P1.4 采集顺序与自校验脚本 | 不装阈值数字（归 P0.1）；不装采集结果（归 `baseline/windows/`） |
| `baseline/conformance/` | **P0.4 双后端一致性证据**：`summary.md`（回归/负向命令 + PASS 表 + 已知差异表）、`<scene>/<backend>/{report.json,frame.png,run.txt}`、`negative/` 一套转红证据 | 不装阈值（归 P0.1）；不做像素相等金图（归 P1.4 真窗） |
| `baseline/build/` | **P1.1 六目标交叉编译证据**：`summary.md`（结果表 + 红项 B1–B6）、`<os>-<arch>.txt` 8 段原始输出（默认/nogpu × 原始/过滤/库范围 + 加载失败枚举） | 不装 vet/单测结果（归 P1.2/P1.3） |

## 当前进度（P0 已收口；P1 基线冻结进行中，当前 P1.1b）

> 状态真源是 `docs/ENGINE_RENDER_GPU_CONVERGE_TASKS.md` §2；本表只是索引镜像，两边不一致以台账为准。

| 编号 | 任务 | 状态 | 证据 / 产出 |
|---|---|---|---|
| P0.1 | 验收阈值与环境清单 | ✅ 2026-10-28 | `baseline/thresholds.md` |
| P0.2 | 真窗与金图清单 | ✅ 2026-10-28 | `baseline/windows.md`（自校验实测：162 真窗 / 28 headless；金图 114 仓内 / 8 运行期 / 40 无；缺图 0） |
| P0.3 | 归档与回滚演练 | ✅ 2026-10-28 | `baseline/rollback.md`：R1–R8 全 PASS——回退点 `876b7835` 可辨认（`baseline/` 不存在、台账下一任务 P0.1），恢复后树哈希与三文件 sha1 逐位一致、status 不变、演练 tag/worktree 清零；R5 按「相对起点无新增失败」口径（首轮两次红与原因保留在文档内）；发现 `ui_wr_t_stress` 引用不存在的包，移交 P1.1 |
| P0.4 | 双后端一致性测试台 | ✅ 2026-10-28 | `baseline/conformance/`：3 场景（`solid_gradient`/`clip_blend`/`text_image`）× 2 后端（native/go）全 PASS，`gpu_ops` 3/3/2、CPU 回退 0，跨后端像素差 ≤2/255（text_image 逐字节一致）；负向对照转红；比较器单测 PASS。已知差异 K1（每轮 +2 条/+131072 字节，双后端一致）登记 P1.6/P2.4c；metal/Win32 未验（无真机）。入口 `render/conformance_test.go`，用例 `render/testdata/conformance/` |
| P1.1 | 六目标交叉编译 + `-tags nogpu` | ✅ 2026-10-28（4 目标有红项） | `baseline/build/`：linux/amd64、linux/arm64 默认主体 351 包 PASS、nogpu 库范围 PASS；windows/darwin×2/js 红项 B1–B6（gles 未限平台、render 无条件注册 gles、rwgpu Windows 缺 unix 调用层、ffishim 用 purego.Dlopen、wayland_csd_font 缺 linux 标签、163 个消费方无条件导入 render/gpu）。用户决定插队修 → P1.1b |
| P1.1b | 跨平台构建修复（B1–B6） | 🟡 2026-10-28 开工 | 范围/验收见 TASKS §3 P1.1b；证据 `evidence/P1.1b/`；修完复跑六目标、刷新 `baseline/build/`，P0.4 测试台作 Linux 零变化守卫 |

## 先看哪个

- 要排批：先看 03（重叠）+ 04（死代码），这两张是删留依据。
- 要补测试：看 05，再回 01/02 找对应入口。
- 要对需求：回 `doc_package_map.md` 表 A，再拿 01–05 当证据。
- 要开工：读 `docs/ENGINE_RENDER_GPU_CONVERGE_PLAN.md` 定口径，再按 `docs/ENGINE_RENDER_GPU_CONVERGE_TASKS.md` 取第一个未阻塞任务。

## 多任务分工与衔接（可追溯）

> 四路并行只读扫描，一路一角色，输入输出如下。角色之间是“读同一棵树、写不同文档”，无写入冲突；交叉点（同一符号出现在两份文档）按下表归属，02 只写转发关系、03 只写池语义、04 只写调用有无。

| 角色 | 输入 | 输出文档 | 与其它角色的衔接 |
|---|---|---|---|
| render 导出盘点 | `render/` 全树 + `RENDER_API_CATALOG.md` 快照 | `01_api_render.md` | 多入口嫌疑的池/重试细节不展开，指到 03；调用有无不判定，指到 04 |
| gpu 导出盘点 | `gpu/` 全树（rwgpu/webgpu/types/context/shader） | `02_api_gpu.md` | rwgpu→webgpu 转发只写“经谁转”，转发的池语义不写（归 03） |
| 池/重试对照 | `render/internal/gpu` + `render/internal/cache\|image\|parallel` + `gpu/` | `03_overlap_pools_retry.md` | 复用 01 的入口清单做 key 对比，结论回指 01 的行号 |
| 测试缺口盘点 | `render/`、`gpu/` 的 `_test.go` + 目录 🔌/⚠️ 行 | `04_dead_unwired.md`、`05_test_gaps.md` | 04 的 5 抽查用 01/02 的调用表复核，05 的容差位置回指 03 的预算口径 |

## 本批局限（下一批继续）

- `render/text` 220 文件只做了导出计数 + 抽查，未逐函数比行为。
- `gpu/shader` 子树 200+ 文件只给了顶层精确数，子树只抽样。
- 调用关系用 grep 判“有/无”，未做编译级 callgraph；重命名/别名转发可能漏。
