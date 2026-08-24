# 渲染引擎工业级审查·最终汇总与问题台账（唯一结论真源）

> 审查窗口：2026-08-22 ~ 2026-08-24。范围：ui/、render/、gpu/ 生产代码（约 830 个非测试 Go 文件、34.8 万行）+ docs 真源 + examples 验收体系。
> 本文件是全部审查与复核的**唯一结论出口**：历轮（分模块详审 → 交叉复核 → 终审抽验 → 两轮复核）的判定已全部合并进第二节台账，中间过程文档已并入本文档与 git 历史。
> 配套文档见下方「文档地图」；修复排期见 REPAIR_PLAN.md。

---

## 一句话直话结论

**这个引擎当前不能称为工业级 GUI 组件库 / 通用 2D 图形库；但不是空架子——默认单窗口路径的渲染质量与工程骨架接近可发布水准，离工业级差的是 5 项 P0 和 35 项 P1，其中绝大多数有清晰行号和修法。综合评分 61/100。**

---

## 文档地图（docs/review/ 的固定分类）

| 层 | 文件 | 内容 |
|---|---|---|
| 结论层 | **本文件** | 最终结论、全量问题台账（含每条的验证状态）、评分、真实性与遗漏确认 |
| 计划层 | REPAIR_PLAN.md | 修复批次排期、验收标准、进度登记（唯一排期真源） |
| 详审层 | R1~R7 共 7 份 | 分模块深审原始报告（widget/测试体系、gpu 后端、ui 分层、scene/io/svg、2D API、文本栈、GPU 管线），行号级证据 |
| 过程层 | archive/ 子目录（14 份） | ROUND2 交叉复核、ROUND3 终审、RECHECK 两轮复核——有效结论已全部并入本文件台账，原件存 docs/review/archive/ 可溯 |

---

## 一、两个目标问题的回答

### 能否实现工业级 GUI 组件库？
**现状不能，架构可以。** Flutter 式分层（RO 树/Layer 树/合成器）、on-demand 调度、软件回退光栅都真实存在且能跑能测；差的是 3 项内存/并发安全 P0（一票否决）、性能地基（全帧重绘）和增量呈现断链。工作量估计：P0 清零数周级；单窗口桌面可发布 1-2 个月；多窗口高 DPI 可达性的组件库还需 platform 层专项。

### 能否作为通用 2D 图形库？
**绘图层合格，图形库不合格。** 作为 UI 引擎内嵌绘图后端质量尚可；作为独立 2D 库缺硬能力：布尔运算是像素采样近似（须重写扫描线算法）、clip 忽略绕向、无成熟 GPU 滤镜管线、渐变语义偏离 Skia。

---

## 二、最终问题台账（经四轮验证定版）

> 判定标注：【成立】多条独立证据链证实；【成立·表述修正】问题真实但原描述范围/位置有偏差；【挂账】真实存在但归属待裁决。
> 行号为最近一轮复核实测行号，动手前以行号核对报告（RECHECK2_linecheck，git 历史）校准。

### P0（5 项，全部多轮证实）

| # | 问题 | 位置 | 验证轮次 |
|---|---|---|---|
| X01 | stringViewToString 用 unsafe.String 包 C 内存零拷贝，调用后 :546 即 free（use-after-free）；回调路径同病 | gpu/rwgpu/adapter.go:556-571 | 初审→终审→复核源码级→动态测试环境佐证 |
| X02 | UI/raster 双线程无所有权纪律：光栅线程闭包清 UI 脏标志（裸 bool）+ BoundaryCache 无锁被两线程读写 | pipeline_app.go:1280（原引 1199 已漂移）; boundary_cache.go:27-48 | 初审→交叉归并→复核调用链逐环坐实 |
| X04 | 布尔运算逐像素采样近似（性能灾难架构）：主罪名成立；「EvenOdd 必错」「2048 静默截断」两条具体危害在动态实测中未复现（五角星自并集中心洞保留；30000px 斜率法精确生效），降级为「特定构造下待复现」；另有差集挖洞不生效的真红测佐证该族问题真实 | path_boolean.go:62-100; 差集红测 TestS3c_M3_PathBooleanDifference | 初审→对抗攻击→动态行为实测（本轮修正表述） |
| X05 | GPU 位图主路（MSDF/GlyphMask，默认 Auto 走此路）绕过 shaping，连字/kerning/阿拉伯变形全丢；矢量档与显式 DrawShapedGlyphs 有完整 shaping（生产可达：旋转/斜切文本自动路由进矢量分支） | internal/gpu/gpu_text.go:179 → face.go:160-220; 对照 text.Shape 动态实测出连字 GID | 初审→复核收窄→对抗攻击维持→动态行为实测坐实差异 |
| X06 | 每帧全树重建+无条件全量重录；增量 LayerCache 零生产调用者（死代码） | layer_build.go:20/222/242; layer_cache.go:31 | 初审→grep 复核→编译红灯实测（tmp_vlprobe 引用不存在符号） |

原 X03（光栅线程原地写共享层树）经对抗复核确认**误报摘除**：被写对象每帧新建、channel 交接自带同步；其有效成分已在 X02 中。

### P1（35 项，按修复批次分组）

**批 A 并发安全（3 项，全部成立）**
| 编号 | 问题 | 位置 |
|---|---|---|
| A1 | acquire 超时 hung 恢复只有 500ms 限流重试，无升级/放弃策略 | gpu/webgpu/swapchain.go:946,1009-1021 |
| A2 | NextLayerID 裸 uint64++ 非原子 | ui/scene/layer.go:21-30 |
| A3 | HbShaper mu 只护 faces map，XScale 直写共享 hbFont 无保护 | render/text/shaper_hb.go:67-68 |

**批 B 算错结果类（13 项：12 成立 + B12 部分成立）**
| 编号 | 问题 | 位置 |
|---|---|---|
| B1 | clip 忽略绕向：交点两两配对 even-odd，dir 字段是死代码 | render/internal/clip/mask.go:199-224,291-325 |
| B2 | Path.Reset 不清 boundsValid，damage 越滚越大 | render/path.go:178-185 |
| B3 | Path.Append 不合并边界 | render/path.go:187-196 |
| B4 | damage 不含线宽外扩，粗描边残影 | render/context.go:1167/1180（原引 1162/1175 为注释行） |
| B5 | focal 渐变忽略 StartRadius | render/gradient_radial.go:129-192 |
| B6 | CPU 渐变不受 CTM：设备坐标直采，paint 链路无逆变换 | render/software.go:531,597,984 |
| B7 | bidi 半套：层级拍平 0/1、run 不做视觉重排（无 UAX#9 L2） | render/text/segment.go:80-84; layout.go:276-337 |
| B8 | IME delete_surrounding 负偏移被 editor 的 Start>=0 门槛吞掉 | wayland_textinput_linux.go:411-426; editor.go:345-348 |
| B9 | preedit 字段错位：caret 塞在 Start，editor 却读 End（End 是 commit 标志恒 0/1） | wayland_textinput_linux.go:380-395; editor.go:353-359 |
| B10 | 可变字体 GPU 光栅化丢 variations（wght=700 画成 regular；shape 层有 varHash 但光栅化层丢） | glyph_mask_engine.go:673-697; gpu_text.go:200-210,430-442 |
| B11 | scene CPU tile 丢图层 alpha（`_ = alpha` TODO） | render/scene/renderer.go:717-722 |
| B12 | recording 回放丢字体（face=nil）；clip 洞在 raster 后端（SetClip 只塞路径不生效、ClearClip 空体），非回放分发层 | recorder.go:173-183; recording/backends/raster/backend.go:119-136 |
| B13 | 缓存指纹弱哈希族：textContentKey 截断 64 字节 / Encoding.Hash 不含 brushes / glyph 键漏 variations（字段存在无人赋值） | boundary_cache.go:420-443; encoding.go:752-800; text.go:1010-1016 |

**批 C 死链/死代码/不一致类（10 项，全部成立；C4 属分期设计现状附保质期条款）**
| 编号 | 问题 | 位置 |
|---|---|---|
| C1 | OS 增量呈现丢参数（PresentWithDamage 收 rects 直接 _）；DamageRectSetter 仅测试 mock | gpu/webgpu/surface.go:312-314; ggcanvas/canvas.go:630 |
| C2 | MSAA 中帧 LoadOpLoad 读已 discard 内存（规范级 UB，需 4x MSAA 真机复现） | render/internal/gpu/render_session.go:4827-4831（决策本体）,1002-1020 |
| C3 | Auto 选管线死路由：ClipDepth/OverlapFactor 判据生产不填充，且 Auto 每帧空转惰性初始化 | render/pipeline_mode.go:60,65; gpu_render_context.go:1457-1458 |
| C4 | 默认 full_paint——事实成立但属分期设计（注释自述 W6 才全局默认 retained）。**保质期条款：W6 宣称完成时若默认仍 full_paint，恢复为缺陷** | ui/embedder/pipeline_app.go:365-371,176-186 |
| C5 | compositor-only 动画断头：ClassifyDirty/Mutations 无生产消费者 | ui/scene/compositing.go:11; packet.go:56-67（目录已修正） |
| C6 | viewport 滚动保留路径丢平移（对照直绘路径有 -scroll） | ui/rendering/layer_build.go:124-139 vs viewport.go:296-299 |
| C7 | Spinner 在 retained 下消失：recordLeafContent 只认三叶且无 OnPaint 兜底 | layer_build.go:263-297; spinner.go:60-94 |
| C8 | WGSL uniform 数组步长按自然对齐非 16 字节规范 | gpu/shader/wgsl/internal/lower/lower.go:818-831 |
| C9 | WGSL MatrixStride 非 uniform 16 字节要求；uniformStructTypes 声明后从未写入 | spirv/internal/codegen/backend.go:1610-1636,155/196/236 |
| C10 | Transform 子 PaintContext 手搓丢字段（LayerBudget/BoundaryCache 等） | ui/rendering/transform.go:113-121 |

**批 D 性能/资源类（4 项成立 + D5 门禁公式化成立）**
| 编号 | 问题 | 位置 |
|---|---|---|
| D1 | Buffer.Map 兜底 goroutine 纯自旋烧核 | gpu/rwgpu/map_pending.go:190-204 |
| D2 | 渐变逐像素 math.Pow（查表优化已写好未上线，约快 200 倍） | gradient.go:98-133; internal/color/convert.go:8-23 |
| D3 | 备选字体按 rune×size 反复整读文件无上限；死检查 fallbackKey 恒不命中 | render/text/fontscan_fallback.go:100-127 |
| D5 | 真窗门禁公式估算：sumRatio 写死几何常数（C2/R4b 同族），改场景参数门禁自动绿 | examples/ui_wr_c2_retained_scene/main.go:57-66 |

**批 E 工程卫生（2 项成立，动态验证扩充）**
| 编号 | 问题 | 位置 |
|---|---|---|
| E1 | 构建红灯探针目录：tmp_vlprobe、render/tmp_ref_stroke（+wind 子包）、**tmp_c4dbg（动态验证新发现）** | 各目录 main 重复声明/引用不存在符号 |
| E2 | shaper.go 注释矛盾：头部与 SetShaper 都说默认 OwnShaper，实际 HbShaper，同文件 :16-19 又说实话 | render/text/shaper.go:7,20-21,28-29 |

**复核新增（N/M 系列，5 项）**
| 编号 | 级别 | 问题 | 位置 |
|---|---|---|---|
| N1 | P1 | Wayland/X11 DPI 缩放恒为 1.0：scale 只读不写、协议侧未监听 wl_output.scale/fractional_scale，HiDPI 整链错误分辨率 | ui/platform/wayland_linux.go:1377,715; x11_linux.go:404 |
| M1 | P1 | wrgate 门禁三处叠加放宽：短跑(<5s)静默跳过 FPS/P95 门禁 + 默认 55 非 60 + interval FPS 掩护 wall FPS——掉帧真窗可拿假绿 | examples/wrgate/report.go:340-360 |
| M5 | P1(挂账) | render 根包 3 处真红测：dash 无间隙、描边像素反超填充、布尔差集挖洞不生效；另 3 处环境依赖失败。归属需裁决（工作区有其他会话改动） | render 包 TestContext_StrokeWithDash_Rectangle 等 |
| N2/N3 | P2 | vsync 监听协程无退出泄漏；剪贴板 Get 最长阻塞 UI 线程 3 秒 | scheduler.go:100-116; wayland_clipboard_linux.go:469-505 |
| R19 | P1 | 1px 对齐能力 ✅ 接近虚标：承诺采样/截图门禁，实际仅默认关闭的数学自检 + 硬编码 snapped_lines:46 | examples/ui_wr_r19_snap/main.go:168-201 |

### P2（择要）

semantics 未接平台可达性、wgpu-native 加载无 ABI 校验、lib 目录 zip 残留、animation Stop() 一刀切 Dismissed、AnimatedOpacity.Mutations 无上限、shader 四后端缺跨后端同源金标语料、vet 正式包告警（gpu unreachable / ui/rendering 自赋值 / platform unsafe 提示）。

### 待真机/竞态实锤尾巴（随对应修复批次钉死）

X02 `-race` 真窗、X05 连字画面级对比、C2 4x MSAA 复现、FifoRelaxed 时序压测、acquire 故障注入。

---

## 三、真实性与遗漏确认（用户专项质询的回答）

### 这些问题都是真实的吗？——是，且有量化依据

| 验证强度 | 条目 | 说明 |
|---|---|---|
| 四重以上验证（初审+交叉复核+源码级复核+对抗攻击/动态实测） | X01、X02、X04、X05、X06 全部 P0；B2/B4/B8/B13 等 | 每条至少两次独立回源码定位，关键条目有行为级实验（shaping GID 对比、布尔探针）或编译级实证（死代码引用编译失败） |
| 三重验证（初审+复核+行号机械核对命中） | 批 A/C/D/E 绝大多数 | 32 条行号引用 81% 精确命中、19% 漂移但代码均在、0 条因核对被推翻 |
| 摘除的误报 | X03（1 条） | 经对抗复核三方向攻击均无法击穿翻案，维持摘除 |
| 降级/收窄 | X04 两条从罪、X05 范围、B12 定位、C4 性质 | 如实记录，不影响主定性 |
| 归属待裁决 | M5 三处红测 | 工作区存在其他会话未提交改动，不排除在途线引入，挂账不冒进 |

**误报率：37 项正式台账条目中 1 项（X03），且已被纠出并记录。**

### 批次还有遗漏吗？——有，已全部并入 REPAIR_PLAN

本轮发现的遗漏及处置：
1. **tmp_c4dbg 编译红灯**（动态验证发现）→ 并入第一批 1.2；
2. **wrgate 门禁放宽点 M1** → 第五批 5.3 扩大为「C2/R4b/R19 三处公式化或缺失门禁一并实测化 + wrgate 短跑免检/55 阈值/interval 掩护三处收紧」；
3. **render 根包 3 红测**（dash/描边/差集）→ 第四批 4.1 金标集追加这三个场景；先裁决归属；
4. **R19 采样门禁缺失** → 并入第五批门禁实测化；
5. **N1 HiDPI 缩放** → 新增第七批 platform 专项；
6. **vet 正式包告警**（unreachable/自赋值）→ 并入第一批卫生清理；
7. **能力债（非缺陷，单列不占批次）**：手势竞技场孤岛（NewScrollable 生产零调用、InputRouter 不进 arena、全栈无人产 PointerCancel）、U21 Golden 逐位对比规范零落地（要么落地进 wrgate 要么文档降格为「逻辑门禁版 ✅」）、跨文档状态矛盾 5 处（基座文档 retained 未来时、API 目录裁剪前后矛盾、「26 窗」实为 27 等）——已记录，待产品节奏决定是否排期。

---

## 四、值得肯定的部分

1. 验收体系骨架诚实：真窗目录零造假、⬜ 能力不建目录、多数门禁是真跑运行时指标（R7/R7b 甚至严于文档）；
2. shader↔CPU uniform 六组布局逐字节一致、Y 方向全链统一；
3. 文本 hinting 栈达 FreeType 对照精度；
4. shader 编译器四后端各有充分自测（SPIR-V naga 金标回归、DXIL dxcvalidator 位流对照）；
5. 34 处锁无重入风险、19 处 atomic 类型全对；
6. 文档驱动开发留下完整真源，使全部问题可溯源到行号。

## 五、评分（统一折算百分制）

| 模块 | 得分 | 一句话 |
|---|---|---|
| R1 widget+验收体系 | 76（M1 门禁放宽点发现后下调） | 地基合格，门禁需堵三个放宽点 |
| R2 gpu 后端 | 62.5 | 功能可用，一个 P0 内存洞 |
| R3 ui 分层 | 64 | Flutter 式骨架真，增量缓存死代码 |
| R4 scene/io/svg | 48 | 双实现冗余+线程竞争，不能算工业级 |
| R5 2D API | 63.75 | 光栅核心扎实，布尔假货挡路 |
| R6 文本栈 | 60 | 零件工业级，总装没完成 |
| R7 GPU 管线 | 58 | 默认主路可发布，增量呈现断链 |
| **综合** | **61/100** | 「能跑的演示级引擎 + 工业级的零件」 |

## 六、方法论备忘

静态审查的两个系统性风险在本轮被实证：①行号会漂移（引用必须带包路径，动手前以最新核对为准）；②「必错/恒错」级断言必须动态复现才准入账。后续若再复核，建议直接以修复过程的 -race/Golden 截图为载体，不再单独发文。
