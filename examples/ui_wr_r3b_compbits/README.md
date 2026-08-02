# ui_wr_r3b_compbits — R3b 合成位 / 边界发现

## Run

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
go run ./examples/ui_wr_r3b_compbits                 # 默认无限运行，手动关窗退出
RUN_SECONDS=8 go run ./examples/ui_wr_r3b_compbits   # 8s 后自动关闭并输出门禁 JSON
```

## Window / Close duration

1200×800 · 默认**无限运行**（手动关窗退出）；传 `RUN_SECONDS` 按指定秒数自动关闭（`<5 → FAIL (U16)`）。

## 界面布局与预期效果

窗口纵向分区：**顶栏 48px → 左图例 260px → Body (284,60) 904×656 → 底部 HUD 72px**。

```
+--------------------------------------------------------------+
| TOP: ui_wr_r3b_compbits  R3b 合成位/边界发现                  |
+----------+---------------------------------------------------+
| LEGEND   | 主链 a(200)     sib s(140)   s2(120)    底部色条   |
| (260px)  |  └b(150)       └hotS蓝块    └hotS2橙块  30x30x6    |
|          |   └c(100)       └sib static              非boundary|
|          |    └hot红块  └d(60) "d"                             |
+----------+---------------------------------------------------+
| HUD: R3b | phase | fps | p95 | boundaries | depth | 门禁       |
+--------------------------------------------------------------+
```

| # | 区域 | 位置（窗口绝对坐标） | 内容 | 预期效果 |
|---|------|----------------------|------|----------|
| 1 | 主链 a→b→c→d | (314,90) 起，a 200×200 → b 150×150 → c 100×100 → d 60×60 | 4 层 RepaintBoundary + 每层静态色块/标签（n1-0..2） | 静态内容永不闪；链深度贡献 max_depth=4 |
| 2 | c 内 hot 红块 | (474,190) 附近，60×60 | 每 tick 变色红块 | **Steady 相位闪**；只有 c 层 rerecord |
| 3 | d 内 "d" 标签 | (474,250) 附近 | 白色小字 | **Recover 相位闪**（d 是唯一活层） |
| 4 | 兄弟 s | (544,90) 起 140×140，hotS 蓝块 50×50 + 静态块/标签 | 独立旁路 nest | **Steady 相位 hotS 闪**；与其他 nest 互不污染 |
| 5 | 兄弟 s2 | (714,90) 起 120×120，hotS2 橙块 44×44 | 第二旁路 nest | **Spike 相位 hotS2 闪**；其余静 |
| 6 | a 层 | (314,90) 200×200 | 主链根 boundary | **Spike 相位闪**（外层脏 → 整链重录） |
| 7 | 底部色条 | (304,530) 起，30×30×6 | 非 boundary 色块 | 永不闪 |
| 8 | 相位行为 | — | Steady 2s → Spike 2s → Recover | 三相位各激活不同层，肉眼可确认脏区隔离 |
| 9 | HUD | (0,728) | fps/p95/boundaries/depth + 门禁 | **每 ~0.1s 刷新**：fps≈59.9、boundaries≈6、depth≈4；相位名实时变化 |

### 判断标准

- 静态内容全程不闪 = compositing bits 未泄漏到无关层。
- 各相位仅预期层闪 = 增量传播正确（clean 子树跳过遍历）。
- HUD depth=4 常驻 = 深链发现正确。

## Gates

- `boundary_count ≥ 3`（主链 4 + 旁路 2）
- `boundary_max_depth ≥ 2`（实测 4）
- `root NeedsCompositing=true`（R3b 门禁第 3 项：NeedsCompositing 正确传播）
- `present_policy = full_paint`
- §2.2 全族 A–J JSON 输出
