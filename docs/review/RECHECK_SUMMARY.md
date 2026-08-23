# 复核终版台账（RECHECK SUMMARY）

> 日期：2026-08-23。本文件汇总 5 份复核报告（RECHECK_p0 / RECHECK_p1ab / RECHECK_p1cde / RECHECK_gaps / RECHECK_docs）的最终判定，取代此前 FINAL_SUMMARY.md 第二节的问题清单定版。
> 复核方法：37 项已知问题逐条回源码 + 反向查漏扫描薄弱区 + 对照真源文档双向核对。

## 一句话结论

**复核后：6 项 P0 → 5 项成立（1 项误报摘除）；31 项 P1 → 全部成立、2 项修正表述/路径笔误；新发现 1 条 P1（HiDPI 缩放恒为 1）和 2 条 P2；文档级对照未发现「✅ 虚标」，发现 1 处审查漏看细节和 1 处 API 目录描述与事实冲突。**

---

## 一、P0 终版（5 项，原 X03 摘除）

| # | 问题 | 判定 | 关键证据 | 相比原结论的变化 |
|---|---|---|---|---|
| X01 | stringViewToString 包 C 内存零拷贝，:546 立即 free 后仍可被读 | **成立** | gpu/rwgpu/adapter.go:556-571, :533-546, :593-594；全文件无 KeepAlive；回调路径同样暴露 | 补充：回调分支 callbackStringView 也复用此函数 |
| X02 | 光栅线程清 UI 脏标志 + BoundaryCache 无锁跨线程读写 | **成立** | pipeline_app.go:1199→:997→raster/loop.go:60-84；boundary_cache.go:27-48；UI 侧 Clear 在 :656-659 | 无变化，链路完整坐实 |
| X03 | 光栅线程原地写共享层树字段 | **误报，摘除** | 被写 PictureLayer 每帧新建不跨帧共享；pkt 走 channel 自带 happens-before | 有效成分已在 X02 中覆盖 |
| X04 | 布尔运算逐像素采样、2048 静默截断、恒 NonZero | **成立** | path_boolean.go:62-100；签名无填充规则参数 | 无变化 |
| X05 | GPU 位图主路绕过 shaping | **部分成立，表述收窄** | MSDF/GlyphMask 主路确实无 GSUB/GPOS；但矢量档 text.go:1008 走 text.Shape 完整 shaping | P0 维持（默认 Auto 档走位图主路），修复范围缩小到两条位图管线 |
| X06 | 每帧全树重建 + ui/rendering LayerCache 死代码 | **成立** | layer_build.go:20/222/242；NewLayerCache 零生产调用；tmp_vlprobe 编译失败本轮复现 | 补充说明 render/scene 有同名另一套缓存（非死代码） |

## 二、P1 终版（原 31 项全部维持 + 新增 1 项 = 32 项）

复核结果：**0 条误报**。变化如下：

- **表述/路径修正（不影响定性）**：
  - C1 实际位置是 `gpu/webgpu/surface.go:312`（非 render/surface.go）；C5 两文件实际在 `ui/scene/`（非 ui/rendering）——均为此前文档路径笔误。
  - B12「recording 丢 clip」的位置修正：SetClip 有录有发，洞在 raster 后端 SetClip 只塞 path 不生效、ClearClip 是空函数体。
  - C4 定性修正：默认 full_paint 属「分期设计」（注释自述 W6 才全局默认 retained），不算漏接。
- **新增 N1（升 P1）**：Wayland/X11 的 DPI 缩放恒为 1.0——scale 字段只读不写、协议侧没监听 wl_output.scale/fractional_scale，HiDPI 屏整条渲染链按错误分辨率出图（wayland_linux.go:1377/715, x11_linux.go:404）。属已知取舍（代码注释自认 later refinement），但按工业级标准应列 P1。
- 其余 A1-A3、B1-B11、B13、C2/C3/C6-C10、D1-D3/D5、E1/E2 全部原样成立。

## 三、P2 变化

新增 2 条：
- vsync 监听 goroutine 无退出路径，关窗后泄漏并持续唤醒（scheduler.go:100-116）；
- Wayland 剪贴板 Get 最长阻塞 UI 线程 3 秒（wayland_clipboard_linux.go:469-505）。

原有 P2（semantics 未接可达性、wgpu-native 无 ABI 校验、lib zip 残留等）确认仍在。

## 四、文档级对照结果（双向核对）

- **没有发现「真源标 ✅ 但代码没接线」的虚标**：R3✅ 指 BoundaryCache（真窗实测背书）；OS 级增量呈现真源从未宣称已通；⬜ 能力对应真窗目录确实不存在。验收状态诚实。
- **审查侧 1 处漏看**：X05 的矢量文本分支其实有 shaping（已并入上表修正）。
- **真源侧 1 处建议改口**：RENDER_API_CATALOG.md §2 布尔运算行写「自交路径也支持」，与 EvenOdd 必错的事实冲突。
- filters/raster 未被误判为缺陷（纯 init 副作用包是 AGENTS.md 已知设计）。

## 五、查漏扫描结果

崩溃/内存安全级新问题：**零**。扫过的区域及方法详见 RECHECK_gaps.md（gpu/context、ggcanvas、窗口生命周期、panic/死循环/sleep 高危模式全仓 grep、4 组定向测试全绿）。

## 六、对最终结论的影响

工业级资格判断**不变**：挡路的核心仍是内存安全（X01）、线程纪律（X02）、布尔假货（X04）、文字 shaping 断线（X05 收窄后仍占默认主路）、性能地基（X06）、增量呈现断链（C1 族）。P0 从 6 项修正为 **5 项**，P1 修为 **32 项**，总体评分 62/100 不变——因为摘除的 X03 本来就与 X02 同根，而新增的 N1（HiDPI）在桌面 Linux 上影响面不小。

*复核载体：RECHECK_p0.md / RECHECK_p1ab.md / RECHECK_p1cde.md / RECHECK_gaps.md / RECHECK_docs.md，均可溯源行号。*
