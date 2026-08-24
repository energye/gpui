# 第二轮复核汇总（RECHECK2 SUMMARY）

> 日期：2026-08-24。本轮与上一轮（RECHECK）的区别：不再重复静态读码，而是五个新角度——①对上轮改判做对抗性攻击；②真跑编译/测试/行为实验的动态验证；③台账行号全量机械核对；④查漏 v2 扫上轮盲区；⑤从真源文档反向抽查 ✅ 虚标。
> 载体：RECHECK2_adversarial / RECHECK2_dynamic / RECHECK2_linecheck / RECHECK2_gaps / RECHECK2_docs，共 427 行。

## 一句话结论

**上轮改判经受住攻击全部维持；台账行号 81% 精确命中、无一条结论被推翻；但动态验证发现 X04 台账表述需要修正（截断未复现、且实测暴露了更准确的问题形态），并新增 2 条 P1（门禁三处叠加放宽、render 根包 3 处真红测）、1 条接近虚标的 R 能力（R19）。**

## 一、对抗复核：三个改判全部维持

| 改判 | 攻击结果 | 关键证据 |
|---|---|---|
| X03 摘除 | **维持**。全目录无对象池，AddPicture 每次字面量新建，SetPicture 零调用，BoundaryCache 的 Picture 不回流层树 | layer_build.go:319/20, build.go:121-127 |
| X05 收窄 | **维持**。默认 Auto 确走位图主路；但矢量分支不止显式可达——旋转/斜切文本自动路由进 text.Shape；DrawShapedGlyphs 有生产消费者（gpu_renderer.go:247） | text.go:535-564 |
| C4 放宽 | **维持但有保质期**：真源 §5 实测 W6 ⬜ 未开始（当前 W0-W3✅、W4🔄），「retained 尚非默认」符合分期；一旦 W6 宣称完成而默认仍 full_paint，立即恢复原判 | ENGINE_UI_WIDGET_RENDER §5 |

## 二、动态验证（本轮最大价值：静态结论 vs 真跑结果）

| 项 | 结果 |
|---|---|
| 编译矩阵 | build 红灯 4 包——比台账多一个 `tmp_c4dbg`（main 重复声明），卫生清单需补；vet 红 8 包，正式包有 unreachable/unsafe/自赋值 |
| 测试矩阵 | ui/rendering、ui/embedder、text shaping 全绿；gpu/rwgpu 5 失败均为 wgpu 原生库缺失（环境性）；**render 根包 6 失败中 3 处真 FAIL**：dash 虚线无间隙、描边像素反超填充、布尔差集挖洞不生效——与台账 X04 同族但形态更具体 |
| X04 布尔实测 | EvenOdd 五角星直画 vs 自并集仅边缘 AA 差异（1029px）、**中心洞保留——「EvenOdd 必错」在自并集场景未复现**；2048 截断用斜率法测 30000px 坐标精确生效，**未复现静默截断**。X04 的「逐像素采样架构低效」属实，但两条具体危害描述需降级为「特定构造下可复现」而非「必错」 |
| X05 shaping 实测 | Shape("fi") 出连字 GID 5042，LayoutGlyphs 出 [73,76] 纯拼接——两管线行为确实不同，与台账一致 ✅ |
| 新坑记录 | 离屏画图漏调 FlushGPU 得全空假象——排查渲染问题时先查这个 |

## 三、行号机械核对（32 条）

一致 26（81%）/ 漂移 5 / 失配 1。失配是 `text.go:1008` 未写包路径（三个同名文件，实际是 render/text.go）——伪失配。漂移要点：**动手修时按实际行号找**：ConsumeNeedsPaint 在 pipeline_app.go:1280（非 1199）；MSAA LoadOpLoad 决策在 render_session.go:4827-4831（非 5368-5373）；damage 上报在 context.go:1167/:1180；shaping 缓存在 render/text/cache/shaping.go；B12 精确位置 recording/backends/raster/backend.go:119-136。

## 四、查漏 v2 与文档反向抽查的新发现

**新增 P1 两条：**
1. **wrgate 门禁三处叠加放宽点**：短跑（<5s）静默跳过 FPS/P95 门禁 + FPS 默认 55 非 60 + interval FPS 掩护 wall FPS（report.go:340-360）——掉帧真窗可拿假绿。这条动摇「门禁体系完全可信」的旧评价，R1 的 80 分应打折。
2. **render 根包 3 处真红测**：dash 无间隙、描边反超填充、布尔差集挖洞不生效——并入 X04 族修复验收。

**接近虚标一条：R19**——文档承诺「采样或截图门禁」，实际只有默认关闭的数学自检，画面清晰度零断言。另 R4b 的 damage 门禁判写死公式常量（跑成什么样都绿）。按项目自己的 U21 规范，27 个真窗 0 个 Golden 逐位对比。

**干净区确认**：shader 四后端、filters/raster 回调、SDF 加速器、34 处锁无重入、19 处 atomic 类型全对；手势竞技场状态机本身无 bug（但「孤岛」判断加重：NewScrollable 生产零调用 + 全栈无人发 PointerCancel）。

**跨文档矛盾 5 处**：基座文档把已完成的 retained 当未来事、API 目录对裁剪一处说坏两处说好、「26 窗」实为 27 等，明细见 RECHECK2_docs.md。

## 五、台账修订（相对 RECHECK_SUMMARY 的增量)

- P0 维持 **5 项**不变（X03 摘除经攻击确认；X04 表述修正：「逐像素采样近似+性能灾难」为主罪名，「EvenOdd 必错」「2048 静默截断」降级为待定构复现的从罪）；
- P1 由 32 → **35 项**：+wrgate 门禁放宽点、+render 根包 3 红测（并族计 1）、+R19 采样门禁缺失；
- P2 新增 4 条（animation Stop 一刀切、AnimatedOpacity.Mutations 无上限、shader 缺跨后端金标语料、vsync 泄漏等上轮已记不重计）；
- 工程卫生清单补 tmp_c4dbg;
- 综合评分微调：62/100 → **61/100**（门禁可信度折价），工业级资格判断不变。
- 修复计划 REPAIR_PLAN 相应调整：第五批 5.3 扩大为「C2/R4b/R19 三处公式化或缺失的门禁一起改实测」；第四批 4.1 金标集追加 dash/描边/差集挖洞三个红测场景。

## 六、方法论备注（供下一轮复核参考）

本轮证明了静态审查的两个系统性风险：①行号会漂移（代码演进），引用必须带包路径；②「必错/恒错」级别的断言必须动态复现才能写进台账。下一轮若再做，建议直接以修复过程中的 -race/Golden 截图作为验证载体，不再单独组织复核。
