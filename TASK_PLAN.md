# GPUI 开发任务计划

> 由 AI 会话维护，新会话读取此文档了解当前进度。

---

## 阶段一：代码迁移与基础清理 ✅

| 阶段项 | 状态 | 完成的事情 |
|--------|:----:|-----------|
| 迁移核心库 | ✅ 完成 | 将 gg, wgpu, naga, gputypes, gpucontext 从 gogpu 生态迁移到 `github.com/energye/gpui` |
| 排除无关库 | ✅ 完成 | 排除 gogpu(窗口/平台), ui(GUI工具包), webgpu(Rust FFI), goffi(待替换) |
| 清理 wgpu 冗余代码 | ✅ 完成 | 删除 cmd/, examples/, *_rust*.go, *_browser*.go, hal/noop/ |
| 清理 gg 冗余代码 | ✅ 完成 | 删除 cmd/, 依赖 gogpu 的示例(10个) |
| 清理 naga 冗余代码 | ✅ 完成 | 删除 cmd/, snapshot/ |
| 合并单模块 | ✅ 完成 | 删除子模块 go.mod, 合并为 `github.com/energye/gpui` 单模块 |
| 替换导入路径 | ✅ 完成 | `github.com/gogpu/*` → `github.com/energye/gpui/*` (1141处替换) |
| 编译验证 | ✅ 完成 | `go build ./...` 通过 |
| 保留示例程序 | ✅ 完成 | 恢复不依赖 gogpu 的 gg 示例(19个) + wgpu 示例(7个) |

---

## 阶段二：goffi 替换为 purego 🔄

| 阶段项 | 状态 | 完成的事情 |
|--------|:----:|-----------|
| Vulkan 后端替换 | ⏳ 待开始 | `hal/vulkan/vk/` 6个文件, 571次 CallFunction |
| Metal 后端替换 | ⏳ 待开始 | `hal/metal/` 2个文件, 5次 CallFunction + 13次 NewCallback |
| GLES 后端替换 | ⏳ 待开始 | `hal/gles/egl/` 3个文件 + `hal/gles/gl/context_linux.go`, 133次 CallFunction |
| Software 后端替换 | ⏳ 待开始 | `hal/software/blit_*.go` 3个文件, 49次 CallFunction + 3次 NewCallback |
| 清理 goffi 依赖 | ⏳ 待开始 | 从 go.mod 移除 `github.com/go-webgpu/goffi` |
| 编译验证 | ⏳ 待开始 | `go build ./...` 通过 |

**替换范围总览：**

| 后端 | 文件数 | CallFunction | NewCallback | LoadLibrary | GetSymbol |
|------|:-----:|:------------:|:-----------:|:-----------:|:---------:|
| Vulkan | 6 | 571 | 2 | 1 | 1 |
| Metal | 2 | 5 | 13 | 3 | 8 |
| GLES | 4 | 133 | 0 | 8 | 28 |
| Software | 3 | 49 | 3 | 7 | 21 |
| **总计** | **15** | **758** | **18** | **19** | **58** |

**无需替换的 GPU 后端：**
- `hal/dx12/` — 全部用 `syscall.Syscall`，零 goffi ✅
- `hal/gles/gl/context.go` — Windows GL 用 `syscall.SyscallN` (103次) ✅
- `hal/software/blit_windows.go` — Windows 用 `syscall` ✅

---

## 阶段三：代码清理与优化

| 阶段项 | 状态 | 完成的事情 |
|--------|:----:|-----------|
| 清理测试文件 | ⏳ 待开始 | 处理引用已删除代码的测试文件 |
| 清理无用外部依赖 | ⏳ 待开始 | `go mod tidy` 清理 |
| 目录重命名 | ⏳ 待开始 | `gg/`, `wgpu/`, `naga/` → 更合适的名称 |
| 代码审查与优化 | ⏳ 待开始 | 检查 go vet 警告 |

---

## 阶段四：框架集成

| 阶段项 | 状态 | 完成的事情 |
|--------|:----:|-----------|
| 验证示例程序运行 | ⏳ 待开始 | 确保 gg/wgpu 示例可编译运行 |
| 编写 GPUI 封装层 | ⏳ 待开始 | 提供简洁的渲染接口 |
| 集成测试 | ⏳ 待开始 | 跨平台(Win/Lin/Mac) GPU 渲染测试 |

---

## 附录：关键文件索引

### 需要替换 goffi 的文件列表

**Vulkan 后端（6 个文件）：**
- `wgpu/hal/vulkan/vk/commands_gen.go` — 565 次 CallFunction（自动生成）
- `wgpu/hal/vulkan/vk/signatures.go` — 0 次 CallFunction（67 次 PrepareCallInterface）
- `wgpu/hal/vulkan/vk/loader.go` — 2 次 CallFunction
- `wgpu/hal/vulkan/vk/commands_manual.go` — 3 次 CallFunction
- `wgpu/hal/vulkan/adapter.go` — 1 次 CallFunction
- `wgpu/hal/vulkan/debug.go` — 2 次 NewCallback

**Metal 后端（2 个文件）：**
- `wgpu/hal/metal/objc.go` — 3 次 CallFunction + 13 次 NewCallback
- `wgpu/hal/metal/metal.go` — 2 次 CallFunction

**GLES 后端（4 个文件）：**
- `wgpu/hal/gles/egl/egl.go` — 22 次 CallFunction
- `wgpu/hal/gles/egl/display.go` — 4 次 CallFunction
- `wgpu/hal/gles/egl/wayland_egl.go` — 3 次 CallFunction
- `wgpu/hal/gles/gl/context_linux.go` — 104 次 CallFunction

**Software 后端（3 个文件）：**
- `wgpu/hal/software/blit_linux.go` — 9 次 CallFunction
- `wgpu/hal/software/blit_darwin.go` — 10 次 CallFunction
- `wgpu/hal/software/blit_wayland.go` — 30 次 CallFunction + 3 次 NewCallback