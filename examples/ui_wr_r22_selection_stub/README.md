# ui_wr_r22_selection_stub — R22 选区/光标局部脏预留

独立真窗（U5）：证明「选区/光标局部脏预留」—— 真引擎选区 API 存在且可跑，
stub 空跑只脏 lane 局部，将来做选区时不会全窗重刷。

## 实现点六维（U20）

| 维度 | 内容 |
|------|------|
| 正确性 | 真引擎 API `textinput.Editor.SetSelection` 驱动 A/B stub 状态机；lane 视觉镜像同一状态（无示例自造假 API） |
| 脏区 | 相位切换只重绘 lane 局部（selA/selB 改色 + 光标 `MoveTo` + 状态字，全 paint-only）；`layout_count` 基线后漂移 ≤4 |
| 缓存 | dense 板全程 untouched；Golden 静态掩码盖顶栏/图例/dense |
| 边界 | resize 使探针几何失效后重解；终帧 Recover 精确回到选区 A（selB 尾回到 lane 底色 + 光标回家，无残留） |
| 失败模式 | API 缺失、stub 调用 <2、layout 漂移超帽、任一像素断言挂、次跑起 Golden 非 0、fps<55 → FAIL |
| 窗内如何看出 | Spike 高亮变宽+光标藏，Recover 窄高亮+光标回；HUD 实时 SEL 状态 + stub 计数 |

## Run

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=5 go run ./examples/ui_wr_r22_selection_stub
```

## Window / Close duration

- 1200×800 逻辑像素（§2 主表 R22）
- 关闭用时长 **5s**（§2 主表 / §2.5 正确性档）
- `RUN_SECONDS<5`（U16）→ FAIL 退出
- 需 GPU 真窗（needs_gpu_window）；无环境 → exit 1，禁止 CPU stub

## Visible effect

| Region | 预期 |
|--------|------|
| STUB 板（左上 560×300）| 标题 + 预留行文字 + 选区 lane：Steady/Recover 窄高亮 + 光标在 76；Spike 宽高亮 + 光标跟到选区尾 156 |
| DENSE 板（右上 300×300）| 4×4 色格 + 8 标签 + 注脚 **全程静止** |
| HOT 板（左下 250×250）| 绿盘持续呼吸缩放（与选区无关的动画热点）|
| 状态字（STUB 板内）| `SEL=A caret=on` ↔ `SEL=B caret=off` 随相位跳 |
| LiveHUD（底部）| fps / p95 / SEL / stub计数 / paint 实时 |

## Gates

按 §2 主表 R22（字段可 0；API 存在）+ §2.6.2（stub 空跑）+ §2.2 全族：

- `selection_api_present == true`（真 `Editor.SetSelection` 存在且调用成功）
- `selection_stub_calls >= 2`（相位驱动的 A/B 切换真实发生）
- `selection_dirty_px == 0` + `selection_fields_zero == true`（预留语义：专用引擎计数器尚无，字段按 §2 保持 0，显式声明非省略）
- 相位切换后 `layout_count` 漂移 ≤4（切换全是 paint-only：高亮改色 + 光标 `MoveTo`，永不触发布局；layout 风暴会超帽百倍）
- `paint_count` 只做观测（`paint_rate_per_sec`）：full_paint 策略按设计每帧重绘，累计值随跑长线性涨，不适合当门禁
- 快照 `SnapshotAsync` 在 ticker 协程外发射：它的 500ms 有界等待是量测开销，真应用不会阻塞 UI 循环去等快照；完成证据是双文件存在 + 非空，不是发射 flag
- `present_count ≥ 1`；`policy=full_paint`（正确性窗，不冒充 retained）
- `fps_wall ≥ 55`（持续 HOT tick）；`interval_p95_ms ≤ 22`；`hitch_rate_per_min ≤ 5`
- `vsync_source` 必须存在（fallback 如实报告）
- cpu_ui/cpu_raster 非双 0；rss_* 字段齐
- 像素断言 6/6；Golden 静态掩码次跑起逐位 0；稳态+恢复双快照落地
- RSS 只做观测：5s 窗处启动均衡爬坡内（Go 堆 + GPU 驱动 + 着色器缓存一次性建立），斜率数值大属预期；泄漏 verdict 归 R15 soak

## 像素断言点（无竞态设计）

终帧恒为 Recover（选区 A 已恢复），期望全部精确、无跨帧竞态：

| 断言 | 形态 | 期望来源 | 容差依据 |
|------|------|----------|----------|
| dense_cell_00_exact | F0 静态 | 色格 (0,0) 基色 (0.25,0.40,0.62) | ≤8/通道（理论色合成舍入） |
| selection_lane_blend_exact | F5 半透明 | `0.45·sel+(0.55·lane)` 公式解 | ≤12/通道（混合，附公式） |
| selection_b_hidden_restored | F0 恢复 | selB 尾 = lane 底色（藏态恢复） | ≤8/通道 |
| caret_bar_exact | F0 静态 | 光标橙 (1.0,0.65,0.20) | ≤8/通道 |
| pulse_disc_center_invariant | F3 变换不变点 | 呼吸盘中心恒在盘内 | ≤8/通道 |
| dense_note_text_density | F6 文字区域 | 注脚区非底色像素 ≥120 | 区域级（避单点落字隙） |

Golden：顶栏带 + 图例 + dense 矩形掩码，同引擎基线默认零容差；
首跑产基线（first_run，不定 PASS），次跑起逐位对比。

## U17/U18/U20 自检

- U17：多区域（STUB+DENSE+HOT+HUD）· 静态密集（4×4 色格+8 标签+预留行文字）· 动态热点（HOT 呼吸 + 选区相位切换）· 能力专属（选区 A/B 局部脏预留）· 三相位（Steady/Spike/Recover）
- U18：LiveHUD 实时 fps/p95/policy + SEL/stub/paint 计数 + gate 预览色
- U20：见上方六维表
