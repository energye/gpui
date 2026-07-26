# 全局编码硬规则（gpui）

> **版本：1.0** | 日期：2026-07-27  
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

## 修订

| 版本 | 日期 | 说明 |
|------|------|------|
| 1.0 | 2026-07-27 | 首版：禁止 CGO、强制 purego；句柄/Surface 一致 |
