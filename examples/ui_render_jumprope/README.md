# ui_render_jumprope — 双人摇绳 · 中央跳绳 协调动画演示真窗

按 `group-jump-rope.html` 参考实现逐行移植的渲染层 2D 协调动画真窗：
用 UI 渲染管线（RenderObject 树 + GPU Present）实现「左右两人摇长绳、中间一人连续跳绳」。
参考画布 960×600 等比放大 1.25 映射到窗口 1200×800（垂直居中），几何常量与运动数学和参考实现保持一致。

## 场景与手法

| 元素 | 手法 |
|------|------|
| 跳绳 | 钟摆模型：挂在双手连线轴上，摆幅 `R = 184·sin(πt)^0.72`（0.72 次幂让绳形饱满），y 偏移 `R·cosφ`（φ=0 垂到地面、φ=π 甩过头顶），x 叠加微幅横摆；56 采样点 + **中点二次贝塞尔**平滑成单条连续 Path（一个 subpath），两端严格钉在持绳手上；两端各有一段沿切向的深色把手 |
| 摇绳人 | 持绳手做曲柄圆周运动（半径 15，左右水平反相、垂直同相）；持绳臂肩→肘(中点下垂12)→手圆头折线；外侧臂 Q 曲线随相位 ±9 前后摆；圆角矩形躯干 + 圆头描边卡通风（填充 + 深色描边）|
| 中间跳绳人 | 骨盆/肩/头三段堆叠随跳跃起伏；滞空窗口（φ∈[0,0.95]∪[2π−0.95,2π]）内抛物线离地 + 收腿张臂；地面段起跳前/落地后高斯包络屈膝蓄力缓冲；大腿/小腿两段 + 鞋椭圆；躯干为 Q 曲线填充轮廓形 |
| 不穿身 | 绳按相位前/后分层绘制：sinφ≥0 在人前（最后画），否则在人后（人物之前画）；翻转发生在摆幅极值点（绳贴地/过顶，远离身体），换层无视觉跳变 |
| 时间同步 | 绳相位 φ、双臂曲柄、跳跃高度全部由同一个仿真时钟的纯函数导出（`ropeSim` 只累积 φ 与 simT），同步由构造保证，无独立计时器 |
| 卡通造型 | 全帧圆头线帽/圆角连接（`SetStrokeStyle`）；所有人物部件带深色描边（SVG 同款画风）；头发/眼睛/嘴齐全 |
| 循环 | φ 无限累加取模 2π，无缝循环；右上角「跳绳计数」每圈 +1 |

## 运行

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so

# 默认 20s（可调），结束打印指标 JSON
RUN_SECONDS=15 go run ./examples/ui_render_jumprope

# 结束时读回一帧 PNG（目检：绳端在手、前后分层正确、跳跃与绳相位对齐）
SNAPSHOT=tmp/jumprope_shot.png RUN_SECONDS=5 go run ./examples/ui_render_jumprope

# 冻结循环相位取单帧（0..1）：0=绳贴地+人最高，0.25=绳在手线(前层)，0.5=绳过顶，0.75=绳在手线(后层)
FREEZE_PHASE=0 SNAPSHOT=tmp/jr_p0.png RUN_SECONDS=1 go run ./examples/ui_render_jumprope
```

## 指标与门禁

- JSON：fps / interval p50·p95·p99 / hitch / gpu_ops / cpu_fallback_ops / rss / cpu。
- 门禁：有帧即过；`RUN_SECONDS>=10` 时 fps>=30（实测 ~59fps、0 hitch、0 CPU 回退）。
- 底部 LiveHUD 实时显示 fps / 呈现策略 / 圈数。

## 说明

- 全部绘制走 `PaintContext` 公开 API（FillCircle / StrokeLine / FillOval /
  FillLinearGradient / FillPath / StrokePath / SetStrokeStyle），
  底层经 render.Context → GPU 合批提交；描边走 GPU AA 光栅化路径。
- 舞台无 RepaintBoundary：全屏动态内容整窗重绘是常态（同 ui_render_particle 结论）。
- 几何与运动均为纯函数，未引入物理引擎；绳形如需更真实的鞭梢/松弛效果，
  可把采样点改为链式模拟，属增量升级，不动架构。
