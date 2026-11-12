# O6b 第一批：7 能力代表窗 ×120s（2026-11-12）

> 运行：单窗串行、自关、当前 Wayland 桌面；每窗 `RUN_SECONDS=120`。
> RSS 判定：60s 预热 + 60s 测量（O3 口径，线 30000KB/min）。
> 缺内部 RSS 的 P3.x 窗用外置 `tools/probe.py`（0.25s `/proc` + gctrace + smaps）；
> 有内部 RSS 的窗直接取 JSON `rss_slope_steady_kb_per_min`。

| 能力 | 窗 | 测量来源 | 60–120s RSS | 60–120s 活堆 | 判定 | 窗自身门禁 |
|---|---|---|---:|---:|---|---|
| base/idle | `ui_base_idle` | 窗内 | **38** | — | PASS | 仅金图 0.8489%（pre-O2 同值） |
| 文本 | `ui_wr_p33_richtext` | 外置 | **202** | 882 | PASS | 仅金图 0.1722% |
| 图片/异步 | `ui_wr_r10_async_image` | 窗内 | **3800** | — | PASS | 仅金图 1.2455%（既有） |
| 滤镜 | `ui_wr_p34_filters` | 外置 | **94** | 236 | PASS | **PASS**（golden 0.0000%） |
| 图层/合成 | `ui_wr_c2_retained_scene` | 窗内 | **2628** | — | PASS | 仅金图 1.7369% |
| 2.5D 场景 | `ui_wr_p37_scene`（live 模式） | 外置 | **9468** | −970 | PASS | auto 固定帧金图 2.4366%；live 无门禁 |
| 视频 | `video_vc2_soak` | 外置 | **5083** | −1696 | PASS | fps 58.9/p95 22.21（性能门，非内存） |

## 发现

1. **7/7 通过 120s RSS 判据**（最大 p37 9468KB/min，线 30000）；活堆 60–120s 全部走平
   （p33/p34 微增 <1MB/min，p37/video 负斜率）。本批无需 pprof 归因。
2. **测量覆盖不统一**：`ui_wr_p33_richtext`/`ui_wr_p34_filters` 的 JSON `rss_*` 全 0
   （未接 ProcessTracker）；`ui_wr_p37_scene` 的 `-auto-only` 是固定帧门禁（~6s），
   不能跑 120s。**O6 决定：内存矩阵统一用外置 `probe.py` + `GODEBUG=gctrace` + smaps，
   窗自身 JSON/gate 只负责渲染与像素**——这就是“测量平台化”，不改每个窗。
3. 金图漂移（idle 0.8489/r10 1.2455/c2 1.7369/p37 2.4366/p33 0.1722）与
   video p95 22.21 属既有环境/基线类，归 O7，不混进内存结论；idle/r10 已用
   pre-O2 对照确认同值。
4. 运行写脏的 current snapshot（idle/r10/c2/p33/p37 testdata PNG）已还原。

## 下一步（待拍板）

- **O6b-2**：剩余能力窗 120s（text m5/color font、mip/animplay、r20 filter、
  r18 savelayer、2.5D mat_persp、HDR、SVG、mesh、MSAA、p35 几何、4k/retained video）
  + 每能力一档 go 抽检；预计 15–20 轮 ×120s。
- **O6c**：2h soak 选 2 扇（建议 `ui_text_m4_virtual` + `video_vc2_soak`），
  需隔离显示；时长/窗清单待用户确认。
- **O6d**：22 窗金图 + O5 回归 + 台账/证据收口。

## O6b 完整矩阵（26 native + 6 go 抽样，全部 120s）

> 统一判据：60–120s RSS steady ≤30000KB/min。外置 probe（RSS 0.25s + gctrace + smaps）；
> 标 * 的用窗内 `rss_slope_steady_kb_per_min`（已接 ProcessTracker）。全部 PASS。

| 能力 | 窗 | native RSS | go RSS |
|---|---|---:|---:|
| base/idle | ui_base_idle | 38* | — |
| 文本 | ui_wr_p33_richtext | 202 | 225 |
| 文本 | ui_text_m5_anycase | 2472 | — |
| 文本/彩色字形 | ui_wr_p38_color_font | 1656 | — |
| 图片/异步 | ui_wr_r10_async_image | 3800* | 1533 |
| 图片/mip | ui_wr_p36_mipmap | 524 | — |
| 图片/动图 | ui_wr_p36_animplay | 1847 | — |
| 滤镜 16 节点 | ui_wr_p34_filters | 94 | 2529 |
| 滤镜子树 | ui_wr_r20_filter | 372 | — |
| 图层/合成 | ui_wr_c2_retained_scene | 2628* | 2169 |
| SaveLayer | ui_wr_r18_savelayer | 938 | — |
| 2.5D 场景 | ui_wr_p37_scene | 9468 | 9544 |
| 2.5D 材质×透视 | ui_wr_p37_mat_persp | 4389 | — |
| 视频 soak | video_vc2_soak | 5083 | 5020 |
| 视频 retained | video_retained_golden | 921 | — |
| HDR | ui_wr_m5_hdr | 1397 | — |
| SVG 综合 | ui_wr_p38_svg_showcase | 197 | — |
| SVG 滤镜 | ui_wr_p38_svg_filter | 30 | — |
| mesh 低层 | ui_wr_p39_meshgradient | 1107 | — |
| mesh 高层网格 | ui_wr_p39a_meshgradient_grid | 1212 | — |
| MSAA 动画 | ui_wr_p39b_msaa_anim | 189 | — |
| MSAA even-odd | ui_wr_p41_evenodd_msaa | 2588 | — |
| 几何 conic | ui_wr_p35_conic | 475 | — |
| 几何 PathEffect | ui_wr_p35_patheffect | 1328 | — |
| 几何透视 | ui_wr_p35_persp | 762 | — |
| 几何四角 RRect | ui_wr_p35_rrect4 | 171 | — |

- **26/26 native + 6/6 go 全 PASS**（最大 `ui_wr_p37_scene` 9544KB/min，线 30000）。
- 活堆 60–120s 全部 <2MB/min（p37 go −7219 为回落）；无需 pprof 归因。
- go 抽检覆盖 text/image/filters/layers/2.5D/video 六族；其余能力 native 单档，
  同一共享渲染层已由这些族覆盖（O6d 收尾再按需补）。
- 运行写脏的 testdata current snapshot 全部还原，未混入提交。
