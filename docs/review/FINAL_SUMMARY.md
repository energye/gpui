# 渲染引擎工业级能力审查·最终汇总（FINAL SUMMARY）

> 审查日期：2026-08-22 ~ 2026-08-23
> 审查范围：`ui/`、`render/`、`gpu/` 全部生产代码（约 830 个非测试 Go 文件、34.8 万行）+ docs 真源 + examples 验收体系。
> 审查方式：**三轮以上递进总结**
> - 第 1 轮：7 个并行深审任务分模块产出独立文档 `R1–R7`；
> - 第 2 轮：交叉复核 7 份文档，排查矛盾、归并重复、识别漏审，产出 `ROUND2_cross_review.md`；
> - 第 3 轮：对证据链最弱的 12 条结论做源码级终审抽验 + 漏审模块快扫，产出 `ROUND3_final_review.md`。
> 三轮全部落盘于 `docs/review/`，本文件为最终汇总。

---

## 一句话直话结论

**这个引擎当前不能称为工业级 GUI 组件库 / 通用 2D 图形库；但它不是空架子——默认单窗口路径的渲染质量与工程骨架已接近可发布水准，离工业级差的是一批明确的 P0（约 6 项）和 P1（约 30 项），其中大半有清晰行号和修法。**

---

## 一、分模块评分总表

> 说明：第 1 轮各文档评分口径不一（40 分制 / 50 分制 / 10 分制），此处按 ROUND2 M1 裁定统一折算为 **百分制（10 分制×10）**。

| 模块 | 折算分 | 直话结论 | 文档 |
|---|---|---|---|
| R1 widget 层 + 测试验收体系 | 80 | 验收体系基本诚实可信（26 个真窗全实存、指标无假值）；widget 地基合格但撑不起组件库 | R1 |
| R2 gpu 后端封装层 | 62.5 | 功能可用但有一个 P0 内存安全洞（C 字符串 use-after-free）和一批 WGSL 布局不合规 | R2 |
| R3 ui/rendering+embedder | 64 | Flutter 式骨架是真的，但每帧全树重建、增量缓存是死代码、两处跨线程竞争 | R3 |
| R4 scene/io/svg/recording | 48 | 不能算工业级：跨线程共享层树无同步、三套显示列表并存、recording/surface/svg 零消费者 | R4 |
| R5 2D 绘图 API | 63.75 | 光栅化核心扎实可当 UI 引擎绘图层；但布尔运算是假货、裁剪语义错，挡住「独立 2D 库」资格 | R5 |
| R6 文本栈 | 60 | 零件工业级、总装没完成：hinting 是真功夫，但主绘制管线根本没接 shaping | R6 |
| R7 GPU 提交管线 | 58 | 默认 1x 主路径可发布；增量呈现整链是死的、非默认开关不可信 | R7 |
| **加权综合** | **≈62/100** | 「能跑的演示级引擎 + 工业级的零件」，缺总装与安全收尾 | — |

## 二、最终问题台账（经三轮验证后的定版）

### P0——挡住「工业级」资格的 6 项（全部三轮维持，无一推翻）

| # | 问题 | 位置 | 影响 |
|---|---|---|---|
| X01 | C 字符串 `unsafe.String` 未拷贝即返回（use-after-free） | gpu/rwgpu/adapter.go:556-571 | 随机崩溃/内存损坏 |
| X02 | UI/raster 双线程无所有权纪律：BoundaryCache 无锁读写 + raster 线程清 UI 脏标志 | boundary_cache.go; pipeline_app.go:1199 | 数据竞争、丢帧、panic |
| X03 | 跨线程共享缓存条目/脏标志原地写（第 3 轮已收窄范围：层树本身每帧新建，风险集中在缓存条目与同一 pkt 的两段访问） | rasterize.go:50-53 等 | 同上，需 `-race` 真窗实锤 |
| X04 | 路径布尔运算是逐像素采样近似：EvenOdd 规则必错、2048 上限静默截断 | path_boolean.go:62-100 | 功能性错误结果 |
| X05 | 主绘制管线绕过 shaping：GPU 文字路径只有 cmap+advance，连字/kerning/阿拉伯变形全丢 | internal/gpu/gpu_text.go:179 → face.go:160-220 | 复杂文种显示错误 |
| X06 | 每帧全树重建+全量重录；增量 LayerCache 是零调用死代码 | layer_build.go; layer_cache.go | 性能地基缺失 |

### P1——约 30 项，按修复批次分组（详见 ROUND2 第四节）

- **批 A 并发安全**：acquire 锁黑洞恢复策略缺失、NextLayerID 非原子、HbShaper 竞态等 3 项
- **批 B 错误结果**：clip 忽略绕向、Path.Reset 边界滚雪球、damage 无线宽外扩、focal 渐变忽略 StartRadius、CPU 渐变不受 CTM、bidi 半套、IME delete_surrounding 丢弃、变量字体 GPU 丢 variations、scene CPU tile 丢 alpha、recording 回放保真度破等约 13 项
- **批 C 死链/死代码**：OS 增量呈现断链、MSAA 中帧 LoadOpLoad UB（需真机）、Auto 选管线死路由、compositor-only 动画断头、双路径行为分叉（viewport 滚动丢失/spinner retained 消失）、WGSL uniform 步长不合规等约 10 项
- **批 D 性能/资源**：Map 忙等、渐变逐像素 pow、fallback 字体重复加载、缓存指纹弱哈希族、门禁公式估算等约 5 项
- **批 E 工程卫生**：tmp 目录破坏构建（go build ./... 红）、根目录散落二进制、假注释文化等

### P2 新增（第 3 轮快扫发现）

semantics 未接平台可达性、wgpu-native 加载无 ABI 校验、lib 目录下载残留。漏审模块（filters/raster/focus/theme/input/gpu-context/platform 窗口层/third_party）均无 P0/P1。

### 待真机复测尾巴（5 条）

X03 `-race` 真窗、X05 连字行为真窗、MSAA UB 4x 复现、FifoRelaxed 时序压测、acquire 故障注入。

## 三、「两个目标问题」的直接回答

### 1. 能否实现工业级 GUI 组件库？

**现状不能，架构可以。** 支撑判断：
- **够得着的部分**：Flutter 式分层（RO 树/Layer 树/合成器）真实存在且能跑能测；验收体系诚实（26 个真窗全实存、门禁除零保护齐全）；调度器是标准 on-demand+Vsync 模型；软件回退光栅器非玩具。
- **差的部分**：6 项 P0 里 3 项是内存/并发安全（工业级一票否决项）、1 项是性能地基（全帧重绘）、2 项是功能正确性；增量呈现/保留模式这两条工业级 GUI 的看家能力目前是断头路或死代码。
- **工作量估计**：P0 全清 ≈ 数周级专项（X02/X03 线程纪律重构最大）；到「单窗口桌面应用可发布」≈ 1-2 个月；到「多窗口高 DPI 可达性齐全的组件库」还需补 platform 窗口层专项审计 + semantics 接线。

### 2. 能否作为任意 2D 图形渲染库？

**绘图层合格，图形库不合格。** 作为内嵌 UI 引擎的绘图后端（矩形/圆角/文本/图片/渐变/裁剪）质量尚可（R5 光栅化核心评价正面）；但作为独立通用 2D 库对比 Skia 缺硬能力：
- 路径布尔运算是假实现（像素采样近似），必须重写为扫描线/曲面细分算法；
- clip 语义错误（忽略绕向）、stroke 无 hairline 回退、dash 相位语义与 SVG 不符；
- 无图像滤镜管线（filters 仅 6 个 CPU 滤镜注册且未系统审）、无模糊/阴影 GPU 路径成熟度证明；
- 渐变语义偏离 Skia（focal StartRadius、CTM 独立性）。

## 四、值得肯定的部分（避免只报忧）

1. **测试与验收体系诚实度高**（R1 评 32/40）：真窗目录零造假、JSON 指标无固定值假绿，这在自研引擎项目里少见；
2. **shader↔CPU uniform 六组布局逐字节一致**、Y 方向全链统一（R7 抽验）；
3. **文本 hinting 栈是真功夫**（FreeType 对照精度达标，R6 正面确认）;
4. **文档驱动开发**留下了完整真源与修订记录，使本次三轮审查能逐条溯源行号。

## 五、建议的修复路线（按 ROI 排序）

1. **止血**：X01 use-after-free（半天）、tmp 目录清理恢复 go build（半天）、shaper 注释矛盾修正（10 分钟）
2. **线程纪律**：X02/X03——给 UI/raster 划清所有权，BoundaryCache 加锁或改消息传递（数天~一周）
3. **文本总装**：X05 接通 shaping 管线（已有 HbShaper 与 ShapingCache 零件，缺接线）（数天）
4. **布尔重写**：X04 换扫描线实现或引入成熟算法（一周级）
5. **增量呈现**：打通 PresentWithDamage → OS partial present 链 + damage 外扩修正（一周级）
6. 批 B/C 的语义对齐项按 Skia 对照逐个清（持续）

---

*本文档由三轮审查汇总而成；所有结论可在 R1–R7、ROUND2、ROUND3 及源码行号中溯源。审查过程只读代码、未改任何引擎文件；`docs/review/` 下 9 份文档为本次会话新增产物。*
