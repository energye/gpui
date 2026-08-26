# ui_wr_r14_cache_budget — R14 缓存预算/淘汰（真窗）

§2 R14 主能力独立真窗（U5）。**窗 1200×800 · 关闭用时长 60s（§2.5 预算/压力档）· W6。**

## Run

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=60 go run ./examples/ui_wr_r14_cache_budget
```

- `RUN_SECONDS < 60` → FAIL（§2.5 档位时长；`<5` 直接 U16 FAIL）。
- 不带 `RUN_SECONDS` → 交互循环，像素证据关闭。
- `R14_TEX_BUDGET=<n>` 覆盖纹理 LRU 显式预算（默认 **128**；单帧活集约 107 键）。
- `R14_SNAP_DIR=<dir>` 覆盖快照目录（默认 `/tmp/r14_cache_budget`）。

## 场景（U17）

| 区域 | 内容 |
|------|------|
| LIST | 600 行虚拟列表持续滚动（Steady 900px/s → Spike 3600px/s → Recover 300px/s 循环），新行不断产生新缓存键 |
| DENSE | 右侧嵌套 boundary 静态面板（4×4 色格 + 8 标签），全程 skip 不重录 |
| HOT | 每帧自变色热点（Spike 转速 ×2），非缓存 |
| 预算 | retained 纹理 LRU 显式预算 128（高于单帧活集约 107，封住滚动跨帧累积）+ boundary 缓存代际清扫（无显式上限） |

## 可见效果

| 区域 | 预期 |
|------|------|
| 列表 | 行内容随滚动正确更新，滚入行颜色与行号一一对应（确定性 per-index 配色） |
| HUD | 能力 ID/相位/FPS/p95 + 核心计数 `ent=` / `ev=` 实时滚动 |
| 横幅 | `CACHE: entries=…/128 evictions=…` 触顶后 entries 稳在 ≤128、evictions 只增 |
| DENSE 区 | 全程静止（boundary 缓存回放）；HOT 点每帧变色 |

## 门禁（§2 R14 行 + §2.5 + §2.2 全族；硬，不许放）

| 门禁 | 阈值 |
|------|------|
| 纹理缓存条目有上限且触顶 | `texture_entries_max ≤ budget(128)` 且运行中曾达 cap |
| `cache_evictions` 真实发生 | ≥3 且逐 tick 单调（违例=0）；60s 实测量级 ~590 |
| RSS 斜率（稳态最小二乘） | `rss_slope_kb_per_min ≤ 30000` |
| fps_interval / p95 | ≥55 / ≤22ms |
| hitch_rate_per_min | ≤5 |
| CPU | 非双 0（`cpu_pct_avg>0` 且 ui/raster 至少一个 >0） |
| 像素断言 | 3/3：dense cell 色、滚入行色（精确公式）、静态区文字密度 ≥120px |
| Golden | 静态掩码逐位零容差（首跑产基线，次跑起 diff=0） |
| policy | retained；`vsync_source` 必须输出 |

## U20 实现点六维

| 维度 | 内容 |
|------|------|
| 正确性 | 被淘汰条目按需重录——滚入行显示精确 per-index 色（P2）、dense 面板在淘汰压力下保持原样（P1） |
| 脏区 | 稳态帧只重录新挂载 cell + 横幅 + HOT；dense 面板持续 skip（boundary_skip 单调） |
| 缓存 | 显式预算高于单帧活集、封住滚动换入换出的跨帧累积：entries ≤ budget、evictions 单调增 |
| 边界条件 | 预算在首个 retained 帧生效（幂等设置器）；resize 后探针几何重解析；期望值来自冻结配色 |
| 失败模式 | 无界增长（entries>budget）/ 静默淘汰（计数不涨）/ 计数回退 / 行色错 / RSS 斜率失控 → 任一即 FAIL exit 1 |
| 窗内如何看出 | HUD 实时 ent/ev 数字；横幅跟踪上限；600 行滚动；dense 区纹丝不动 |

## 取证说明（诚实边界）

- **RSS 斜率语义**：短跑（<25s）的 slope 数值含启动爬坡，只作观察；关窗证据以 60s 稳态段最小二乘为准（ProcessTracker 自动丢弃前 25% 时长）。
- **预算语义**：引擎 LRU 从不牺牲本帧已用条目（同帧牺牲会导致每帧重录风暴），因此预算低于单帧活集（~107 键）时无法绑定（条目数浮到活集数）；预算高于活集时封住跨帧累积——这是「预算管增长」的真实语义，README 与代码注释均已留痕。`CacheEntries` 是 boundary+texture 两缓存之和，封顶判定只对纹理缓存本身做（boundary 缓存另有约 27 个合法条目）。
- **present_mode 观察**：policy=retained 走 `presentPacketTextured`，但 `PresentOutcome.Mode=full`、`damage_ratio=1` —— present_target auto 判定现状，与 C5/C6 同源，W6 收口时一并处理。
