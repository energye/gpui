# P4.3 三组量测：实测记录（2026-11-10）

## 1 mask 池化

- 口径：headless，三种 mask 尺寸各跑两次，读 HeapAlloc 增量（含 Context/Mask/上传缓冲）。
- 实测：64 → 218.9KB / 139.6KB；256 → 1.235MB / 1.235MB；1024 → 18.94MB / 18.93MB。
- 判读：**同尺寸第二次没有下降**，说明当前用量随尺寸线性增长且没有被复用抵消；
  但这口径包含 Context 与掩码上传缓冲，**不能单独归因到 `maskR8`**。
- 结论：先加 `maskR8` 分配计数（cap/len 变化）再判是否值得池化；在拿到该计数前不改代码。
  粗看 1024² 一档 18.9MB 已接近一条 filter-graph 预算，值得优先确认。

## 2 backdrop 损伤区

- 真窗 `ui_render_base_backdrop`（native，6s，标准关闭）：`frame_raster_ms 153.2`、
  `damage_area_px 960000`（整窗）、`gpu_ops 8724`、`frame_flushes 11`；
  池：device-surface 15.36MB（hits 101/misses 3）、filter-graph 19.2MB（hits 151/misses 5）。
- 对照 `ui_render_base_maskops`：`frame_raster_ms 154.3`、`damage 960000`、`frame_flushes 3`。
- 判读：backdrop 的 **frame_flushes 11 vs 3** 是显著特征（每层整幅 blit + 分段提交），
  damage 仍是整窗（backdrop 全屏场景本就满刷）。要证明「backdrop 静止可增量」需要
  一个局部 backdrop 的场景，当前两窗都不具备 ⇒ 需新增/改造测量场景后再判。

## 3 抖动

- 口径：headless 96×96 线性渐变，dither off vs on。
- 实测：**6400/9216 像素变化，最大差 2 级**（只有 ±1~2，未出现色带跳变），无报错。
- 判读：dither 的扰动幅度受控；是否破坏「零回读 GPU 路」尚未量（需要
  `frame_flushes`/读回计数对照），下一轮用真窗 JSON 的开/关对比补上。

## 结论（本轮）

- backdrop：拿到首组基线（flush 11 / raster 153ms / 两池命中率），下一步需要一个
  局部 backdrop 场景才能判 damage 优化。
- mask：口径需要先加 `maskR8` 计数，暂不动代码。
- dither：幅度实测 ≤2，可继续用；零回读影响待补。
