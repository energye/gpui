# P2.4e 裁剪/混合枚举/呈现入口收敛 + 既有缺陷修复 收口

- 完成日期：2026-10-31
- 依据：PLAN §7.3 P2.4e（B2/B3/B4 三套各收敛为一）+ §2.2 池带入 I1/C7/I2/C3/G2/J1/J2
- 一句话结论：**四子步（e1 G2 老抽象拆解/J1J2 防护、e2 B2 裁剪收敛、e3 B3 混合枚举收敛、e4 B4 呈现单计划化+I2+C3）+ 红修战役 13 批全部落地；gpu 包与 render 包整包+逐文件全绿；三个真产品 bug（整页上传饥饿、上下文所有权、Tab 空环吞键）修复；剩余 2 个已立案专项（misc_draw wgpu 执行态、SeatCapCB 回调 robustness）需专项工具/审查，不在本步闭合。**

## 1 四子步成果

| 子步 | 提交 | 成果 |
|---|---|---|
| e1 | `59670060` | G2 四老抽象文件整删 ~2100 行（生产零可达孤儿簇）、RectAllocator 簇全表面搬移、GPUTexture 家族删、p0 校准改写直连 hal（真机 4 用例 PASS）；J1 idle 回调边沿触发+repool 查重、J2 drain 观察 token+rwgpu.FutureForget |
| e2 | `39e71c79` | B2：scene.ClipState/ClipStack 冗余实现删（-162 行），ClipDepth/ClipBounds 能力保留（Tag 序列派生），语义逐点保真 |
| e3 | `e1605d51` | B3：internal/blend.BlendMode 改绑 image.BlendMode 别名（值域合一）、三张转换表删、唯一边界表 ToPaintBlendMode；scene 线格式值不变零重录 |
| e4 | `03a630be`+`c66041f1` | B4：present() 单计划化（PlanFramePresent 唯一决策 + PresentFramePlan 直通）、平行 clipPresentDamage 删；I2 测试期望改绑+MarkNeedsReconfigure 口；C3 ime probe 补 EventCloseRequested；C7 拆半（Destroy 自死锁修复落地、hub 开窗转正） |
| 红修战役 | `762f2e4e`..`9cca54e6` | 批次1-13：gpu 包共享布局测试缝（12 文件）、session SetMaskBindLayout+25 处注入、buildImageResources 时序对齐、releaseShared 生命周期修复（abandon 前置）、resetAccelerator 保存/恢复、s6_9 门禁设计修正（同日基线重采+0.3ms 绝对下限+P2 regress-only）、m5/filter_present CPU 强制、X11/seat 环境类 2/3 |

## 2 三个真产品 bug（红修战役收获）

1. **整页上传永久饥饿**：glyph mask atlas 整页重建（compaction）成本恒超单帧预算，最大者先裁策略使其每帧被推迟=该页文字永不上屏。修复：整页条目豁免预算（Skia 按需整页上传对比）。
2. **上下文所有权**：Wayland ownsContext 下共享 Device 的 AdapterContext 被首窗关闭连带销毁（其余窗 SIGSEGV）。修复：AdapterContext 引用计数（refs/retain/release，双端同步）。
3. **Tab 空环吞键**：FocusManager 空环时 Tab 被无条件消费。修复：实际移动才算消费（Flutter/HTML 惯例）。

## 3 门禁结果

- gpu 包：整包 + 逐文件全绿（共享布局测试缝、session mask 注入、预算钉改写后）
- render 包：**整包 0 红**（起点 8+17 条：耦合类 6、CPU 像素类 2、尾部 17 同类、SIGSEGV 1——全部清零或定性立案）
- ui 包：全绿（depcheck 豁免、focus Tab 语义、X11/seat 环境类 Skipf 2）
- s6_9：逐文件绿（同日基线重采 + 0.3ms 绝对下限 + P2 regress-only 默认）
- 构建五目标、vet、conformance 双后端 0/0、-race：各批次全绿（详见各批次提交与 red-fix-campaign.md）

## 4 遗留（已立案，不属本步闭合）

1. **misc_draw wgpu 执行态专项**：effect surface + 高级混色的 dual-tex 执行时 dst 采样黑（探针链三路完成，需 GPU capture 级工具）——red-fix-campaign.md 续7-11。
2. **SeatCapCB 回调 robustness**：C→Go 回调派发链间歇失败（flaky 实证，二分指认被复跑推翻）——robustness 专项。
3. **G3**（在册）：TestPresentDamageX11 环境车道。
4. **I1**：解剖完成（三环境三态），修复方向待 wgpu 呈现异步化能力评估。
5. **X11/seat 环境类 1 项**：SeatCapCBFirstSilent 的 sweep 耦合（flaky 同源）。
