# W0 — 控件层脚手架与第一批绘制

> 版本：1.0 | 日期：2026-07-15  
> 状态：**W0 关闭**  
> 依赖：S5.5 入口 ✅ · S6.0–S6.9 主路径 ✅  
> 架构：`widget → render.Context → gpu/webgpu → rwgpu → libwgpu_native`

---

## 1. 目标

1. 新建 **`github.com/energye/gpui/widget`** 包：只画、不另起光栅化  
2. 落地第一批控件 API：**Button / Input / Modal / ListRow / TableCell**  
3. 主题 token（`Theme` / `DefaultTheme`）+ `Rect` / hit-test  
4. CPU 像素结构测试 + **真 GPU present** 门禁（`GPUOps>0`，`cpu_fallback_ops=0`）  
5. 写入主线 W 章；**不**实现完整布局引擎 / 事件循环

**非目标**：完整 Ant Design 语义、无障碍、动画系统、复杂布局、PDF/YUV。

---

## 2. API 一览

| 类型 | 职责 |
|------|------|
| `Theme` / `DefaultTheme` | 颜色、圆角、字号、控件高度 |
| `Rect` / `Align` | 几何与对齐 |
| `Button` | Primary/Default/Danger/Text；Hover/Focus/Disabled |
| `Input` | Label + Field + Placeholder/Error/Focus |
| `Modal` | Host 遮罩 + Panel + OK/Cancel 按钮几何 |
| `ListRow` | 列表行（选中/头像占位） |
| `TableCell` | 表头/单元格 + 网格线 |

绘制入口统一：`Draw(dc *render.Context, th Theme)`。

---

## 3. 测试

| 测试 | 作用 |
|------|------|
| `TestTheme*` / `TestRect*` | token / 几何 |
| `TestButton*` / `TestInput*` / `TestModal*` / `TestList*` | CPU 绘制与 hit |
| `TestComposeFormShell_CPU` | 表单壳组合 |
| `TestW0_FirstBatch_PresentGPU` | 全套控件 present + steady form p50 软预算 |
| `TestW0_ButtonDamage_PresentAuto` | `PresentFrameAuto` 脏区路径 |

```bash
export WGPU_NATIVE_PATH=/home/yanghy/app/projects/gogpu/gpui/lib/libwgpu_native.so
export GOCACHE=/tmp/gpui-go-cache
export LD_LIBRARY_PATH=/home/yanghy/app/projects/gogpu/gpui/lib:$LD_LIBRARY_PATH

go test -count=1 ./widget -timeout 120s
```

本机参考：`TestW0_FirstBatch_PresentGPU` steady form **p50≈20ms**（软门 ≤33.4ms）；kitchen-sink+modal 仅正确性 present，不宣称 60fps。

---

## 4. 不变量

1. **禁止** widget 内直接调用 `rwgpu` / 自建 pixmap 当作最终呈现  
2. 必须可与 `BeginFrame` / `Invalidate` / `PresentFrameAuto` 共存  
3. GPU 测试 `import _ "github.com/energye/gpui/render/gpu"`  
4. 回归不得削弱 render 像素/Comp 门禁  

---

## 5. 退出条件

| 条件 | 状态 |
|------|------|
| `widget` 包可编译 | ✅ |
| 第一批 5 控件 Draw API | ✅ |
| CPU + GPU 测试绿 | ✅ |
| 主线文档 W 章 | ✅ |
| 无 silent CPU 宣称 | ✅ |

**W0 关闭。** 下一：**W1 状态与命中契约**（pressed/hover 统一、焦点环、禁用态矩阵）与 **W2 组合壳 demo**（form/list/table 场景用控件拼装）。
