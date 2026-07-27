# 全局编码硬规则（gpui）

> **版本：1.1** | 日期：2026-07-27  
> **性质：仓库级纪律** — 所有 `ui` / `render` / `gpu` / `examples` 必须遵守  
> **真源交叉：** [`ENGINE_FLUTTER_SKIA_ARCH.md`](./ENGINE_FLUTTER_SKIA_ARCH.md) §1  

---

## 1. 禁止 CGO（硬）

| 规则 | 说明 |
|------|------|
| **禁止 `import "C"`** | 任意 Go 包（含 `examples/`）均不得使用 cgo |
| **禁止 `#cgo` 指令** | 不得链接 `.c` / 系统 C 库生成代码进包 |
| **`CGO_ENABLED=0` 必须可构建** | `CGO_ENABLED=0 go build ./...`（及示例）应通过 |
| **原生库调用只用 purego** | `github.com/ebitengine/purego`：`Dlopen` / `RegisterLibFunc` / `NewCallback` / `SyscallN` |
| **gpu/wgpu** | 已走 purego/syscall 绑定 `libwgpu_native`；不得改回 cgo |

### 为什么

- 交叉编译、静态检查、无 C 工具链环境一致  
- 避免 cgo 线程模型与 Go 调度踩坑  
- 与现有 `gpu/rwgpu`、X11 示例路径一致  

### 允许

| 允许 | 说明 |
|------|------|
| purego 调 `libX11` / `libwayland-client` / `libwgpu_native` 等 | 运行时 `Dlopen` |
| 仓库内 **纯 Go** 实现协议（如 Wayland wire） | 若不用系统 `.so` |
| 外部 **预编译** `.so` 作为运行时依赖 | 不在 `go build` 时编译 C |

### 禁止的借口

- 「只有 examples 用一下 cgo」→ **仍禁止**  
- 「wayland-scanner 生成 C 再 cgo」→ **仍禁止**；改 purego 或纯 Go  
- 「测试里 cgo」→ **仍禁止**  

---

## 2. 模块依赖（硬，摘要）

> **`ui/painting` 已删除：** UI 直接 `import render`。树遍历游标为 `rendering.PaintContext`（`DC *render.Context`、Origin、CompositeOnly）；**怎么画由 UI 调用 `pc.DC.*` 控制**，render 负责执行。


```text
ui → render → gpu → libwgpu_native / 系统句柄
禁止 ui → gpu
```

详见架构真源 §1。

---

## 3. 平台句柄与 Surface

| 规则 | 说明 |
|------|------|
| **句柄类型与 Surface 后端一致** | X11 `Display*`+`Window` → Xlib surface；`wl_display*`+`wl_surface*` → Wayland surface |
| **由 `NativeSurface.Kind` / `PresentPlatform` 驱动** | 不得仅凭 `WAYLAND_DISPLAY` 猜测句柄类型 |
| **关窗顺序** | 先停 Raster / `PresentTarget.Close`，再 `DestroyWindow` / disconnect |

---

## 4. 检查建议

```bash
# 无 cgo 全量编译（按模块裁剪 tags 时仍应无 import C）
CGO_ENABLED=0 go build ./ui/... ./render/... ./examples/...

# 禁止 import "C"
! grep -R --include='*.go' 'import "C"' ui render examples gpu 2>/dev/null

# ui 不依赖 gpu
go test ./ui -run TestNoGPUImport -count=1
```

---

## 5. 架构 vs 示例（硬）

**问题分层：架构问题与示例问题必须分开定位、分开改。**

| 层级 | 路径 | 放什么 | 不放什么 |
|------|------|--------|----------|
| **架构 / 库** | `ui/` · `render/` · `gpu/` | 可复用机制、契约、单测 | 演示场景、固定色块布局、示例文案 |
| **示例 / 烟囱** | `examples/` | 接真窗、拼场景、验收 JSON、宿主 glue | 反向污染 `ui/` 的 demo 包 |

### 5.1 `ui/` 只承载机制

- L2 能力落在 **机制包**：`ui/gestures` · `ui/focus` · `ui/overlay` · `ui/animation` · `ui/theme` · `ui/semantics` 等  
- 包名表达 **职责**（gesture arena、focus manager），**不是**阶段口号或示例名  
- **禁止** 在 `ui/` 下新增「demo / shell / showcase / playground」式包（例如已删除的 `ui/l2shell`）  
- 库内单测只测 **包契约**；跨机制拼装的烟囱测放在 `examples/<name>/`（`go test` 同目录即可）

### 5.2 `examples/` 只做验收与接线

- 命名：`examples/ui_l1_*` / `examples/ui_l2_*` 表示 **阶段验收示例**，不是库的一部分  
- 示例可依赖 `ui/*` 公共 API + `examples/exhost`；**不得** 把示例场景图再导出成 `ui` 子包  
- 示例里的布局常量、颜色、夹紧、快捷键文案 = **示例问题**，默认改 `examples/...`  
- 真窗无输入 / IDLE 空转 / 句柄与 Surface 不一致 = **架构或宿主契约问题**，改 `ui/platform` · `ui/embedder` · `ui/scheduler` · `examples/exhost` 等，**不要** 在 demo 场景里打补丁假装修好了库

### 5.3 修 bug 时怎么分

```text
现象在示例里看到
    │
    ├─ 机制单测/契约也能复现？ ──→ 架构问题 → 改 ui/render/gpu + 库内测
    │
    ├─ 仅该示例布局/文案/拼装？ ──→ 示例问题 → 只改 examples/...
    │
    └─ 宿主未交付事件 / Present 策略？ ──→ 宿主或 embedder 契约 → 改 exhost / platform / embedder
                                           （仍不是「往 ui 塞 demo 包」）
```

| 示例现象 | 优先怀疑 | 典型落点 |
|----------|----------|----------|
| 点了没反应，合成 `EventPointer` 单测却过 | 宿主未选事件 / 未转换 / IDLE 不读队列 | `examples/exhost` |
| 静止仍狂刷 Present | 调度 IDLE / Expose 回路 | `ui/scheduler` · `ui/embedder` · host WaitEvents |
| 色块位置、拖拽夹紧、按钮文案 | 示例场景 | `examples/ui_l2_shell/scene.go` |
| Arena / Focus / Overlay API 行为错 | L2 机制 | `ui/gestures` · `ui/focus` · `ui/overlay` |

### 5.4 禁止的借口

- 「方便示例复用，先放 `ui/l2shell`」→ **禁止**；复用请抽 **真正的机制**，不是 demo 场景  
- 「示例问题也改库，一起 dirty」→ **禁止混修**；PR/说明里写清是架构还是示例  
- 「examples 可以 cgo / 可以 ui→gpu」→ **仍禁止**（§1、§2 全仓适用）

---

## 修订

| 版本 | 日期 | 说明 |
|------|------|------|
| 1.1 | 2026-07-27 | §5：架构 vs 示例边界；禁止 ui 内 demo 包；修 bug 分层 |
| 1.0 | 2026-07-27 | 首版：禁止 CGO、强制 purego；句柄/Surface 一致 |
