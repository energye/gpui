# game_chase_large — 追车大包（S90 厚门，V3 收口）

七合一真窗：追车底（镜头跟随＋粒子拖尾＋粒子池）＋六个大件：tilemap Large
百万瓦片流（S84）、world Cull 两万实体（S85）、physics Broadphase 千盒场（S86）、
nav 烘焙/寻路/队列（S87）、asset Budget 大包（S88）、anim LOD＋audio voices（S89）。
一个世界一个镜头：红车跑大场中路，镜头跟车，所有系统同坐标。绘制一批一次交；
重查询隔帧跑＋绘制封顶，窗口放大成本固定。

## Run

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=30 go run ./examples/engine/chase_large -auto-only
go run ./examples/engine/chase_large -manual-seconds 60
go run ./examples/engine/chase_large
```

## Window / Close duration

1200×800，`RUN_SECONDS<5 → FAIL`，功能轮跑 30 秒（10 分钟×3＋8 小时长跑另起，不在本轮）。

## Visible effect

整窗即内容，无分区壳，指标浮在左上角，金图只比指标带下面的纯画面。

| Region | Expectation |
|---|---|
| 地面 | 可见瓦片 64px 棋盘＋噪点，大车过处流式进出 |
| 路 | 中路深带＋上下黄线，红车循环＋拖尾 |
| 路边怪 | 裁剪集三层三色，隔行采样，窗口再大层层都有 |
| 物理 | Yard 青点＋视口框 |
| 导航 | 黄斜线＋起点红点（live 走，采集冻起点） |
| 右侧条 | LOD 绿/蓝柱＋声音白条 |

## Gates

- `present_count>=1`，七项自检逻辑＋像素＋离屏金全过，窗金 0 差
- 稳态帧率 `fps_interval>=55`，门禁跑锁 1200x800（手动可拖大，内容自扩）
- 业务：`moved>0`，`world_active>0`，`phys_region>=0`，`nav_moved>0`，`lod=101/3/896`，`voices=64/32`
- 绘制封顶：世界点 800、物理点 600（逻辑全量，画面采样），心跳 cull/LOD/voices 隔帧
