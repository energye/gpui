# ui_wr_ime_r1_editor — IME R1 Editor 四元组 (G1 同源同数同判据)

**Run:**
```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
go run ./examples/ui_wr_ime_r1_editor
# 手动关闭：点击窗口 X；不自动退出
```

**Window:** 1200×800 · 手动关闭 · 三栏布局：左中文族测试 | 中纯文本 | 右异形边框

**布局：**
| 区域 | 内容 | 说明 |
|---|---|---|
| 顶栏 | `R1 Editor 四元组 — text+selection+composing+affinity+batch+surrogate+4000` | 标题 |
| 左 340px | **中文族测试清单**：A-J 10族阈值 + 8场景中文列表 | 用中文写当前R的族测试都有什么 |
| 中 460px | **上：R1能力说明**（5行）+ 分隔线 + **下：R1示例真实测试**：输入区460×72+四行探针 | 上说明能干什么，下跑真实IME |
| 右 360px | **动态不规则边框图形**：8边形wobble多边形，双层描边，中心圆点，每帧 `t+=0.08` 形变 | 专门绘制动态异形 |
| 底栏 HUD 72px | R1 ID + phase + fps/p95 + policy + paint + tick/epoch + gate预览 | wrkit LiveHUD |

**左栏中文清单：**
- A 帧时 fps/p95/hitch 仅告警
- B 管线 build p95 <5ms
- C 脏区 允许≈1（full_paint）
- D CPU <40%（5s窗）
- E 内存 slope=off（短窗）
- F GPU fallback==0 硬
- G 图文 hit 可跳过
- H 首帧 <1000ms 硬
- I 回归 可选 <10%
- J 正确性 vet0+四元组探针 硬
- 8场景：1哨兵 2中英混排 3surrogate 4四锚点 5嵌套batch 6往返 7no-op 8越界拒绝

**中栏可见（上下分，人工）：**
- 上 125px：`R1 能力：Editor 四元组（对齐 Flutter TextInputModel）` + 5行：四元组组成/能干什么/affinity与哨兵/边界与截断/状态与钳制 + 灰分隔线
- 下：`R1 人工真实测试（点输入框获焦，手打）` + 提示“请依次手试：打字→退格→方向键→选区拖→拼音预编辑→Esc取消→长文粘贴” + 460×72可交互输入框（键盘/鼠标/IME真实走 Editor，**光标高度与文本同居中，不压字**）+ 四行实时探针/文本快照/4锚点/手工tick
- 最下 140px：**多字号光标自适应（同屏 12/16/20/24px）**：四行 `12px 样例 Aa你好😀 R1` 同文本不同字号，光标统一在第4字符后，高度 `ascent+descent` 随字号自适应，`x=前缀Measure` 精算不劈字，验证不同字号/样式下光标不混在文本上

**右栏可见：**
- 外层8边形：`rx=38%w ry=32%h` 半径 `1+0.18*sin(t+ i*0.9)` wobble，t每帧+0.08，描边橙 2.5px，填充蓝 0.18
- 内层8边形：0.55倍半径，反向旋转，青描边 1.8px
- 中心黄点 4px

**Scenarios (8,同源于 r1_editor_metrics_test.go):** 同上

**Gates (§10.4.1 R1):** 同前表，A仅告警，其余按10.4.1阈值

**Extra JSON:** `probe_count`, `probes[]`, `surround_probes`, `layout="左中文族测试|中纯文本|右异形边框"`
