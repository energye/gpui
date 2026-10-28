# P1.1 六目标交叉编译基线（2026-10-28）

> 原始输出：`baseline/build/<os>-<arch>.txt`。环境：go1.25.5 linux/amd64，`CGO_ENABLED=0`，仓库 `1bd1a06e`。
> 说明：`go build` 碰到包加载错误时只打印一个代表包，所以每份证据同时用
> `go list -e -f '{{.ImportPath}}|{{range .DepsErrors}}{{.Err}} ; {{end}}'` 枚举全部加载失败（各文件 §6/§7），
> 编译错误（undefined 等）以 `go build` 原始输出为准（§3/§5/§8）。

## 执行命令

每个目标按同一脚本跑 8 段，`os/arch` 见文件名：

1. `GOOS=<os> GOARCH=<arch> CGO_ENABLED=0 go build ./...`（原始，预期含已知问题包）
2. 两个已知问题包单独构建：`./examples/ui_wr_t_stress`、`./ui/scene/internal/gpupixel`（预期红，登记）
3. `go build <go list -e ./... 去掉两个已知问题包>`（默认标签主体）
4. `go build -tags nogpu ./...`（原始）
5. `go build -tags nogpu <同 3 的过滤>`（nogpu 主体）
6. `go list -e ...` 默认标签全部加载失败清单（去已知问题包）
7. 同 6，加 `-tags nogpu`
8. `go build -tags nogpu <库范围：去掉 examples/、tools/ 与已知问题包>`（隔离“消费方导入”与“库本身”）

## 结果

| 目标 | 默认主体（§3） | 默认加载失败（§6） | nogpu 主体（§5） | nogpu 加载失败（§7） | nogpu 库范围（§8） | 结论 |
|---|---|---|---|---|---|---|
| linux/amd64 | 351 包 **PASS** | 0 | 346 包 FAIL | 163 | 140 包 **PASS** | 库默认绿；nogpu 红在消费方导入 |
| linux/arm64 | 351 包 **PASS** | 0 | 346 包 FAIL | 163 | 140 包 **PASS** | 同 linux/amd64 |
| windows/amd64 | 357 包 **FAIL**（3 包） | 0 | 352 包 FAIL | 163 | 139 包 **FAIL**（2 包） | rwgpu/ffishim 不能编 Windows |
| darwin/amd64 | 349 包 **FAIL**（2 包） | 0 | 344 包 FAIL | 163 | 139 包 **FAIL**（2 包） | gles 不能编 darwin |
| darwin/arm64 | 349 包 **FAIL**（2 包） | 0 | 344 包 FAIL | 163 | 139 包 **FAIL**（2 包） | 同 darwin/amd64 |
| js/wasm | 346 包 **FAIL**（加载） | 214 | 341 包 FAIL | 209 | 134 包 **FAIL**（加载） | gles/gl 在 wasm 无文件，整条依赖链断 |

两个已知问题包在所有目标上均按预期红：`ui_wr_t_stress`（`ui/kit/button`、`ui/kit/icon` 不存在）、
`ui/scene/internal/gpupixel`（`no non-test Go files`）。

## 基线红项（P1 不改代码，逐条登记）

| 编号 | 现象 | 位置 | 影响目标 | 根因 |
|---|---|---|---|---|
| B1 | `-tags nogpu` 下 163 个包加载失败（161 examples + 2 tools） | 各 `examples/*/main.go`、`tools/*/main.go` 的 `_ "github.com/energye/gpui/render/gpu"` | 全部目标 | 消费方无条件导入 GPU 注册包；`render_visual_*` 已写 `gpu_enabled.go`（`gpui_visual_gpu && !nogpu`）但 `main.go` 仍保留无条件导入，标签化只做了一半 |
| B2 | `ui/platform` 在 darwin/windows 编译失败 | `ui/platform/wayland_csd_font.go:36,203,207` | darwin、windows | 文件缺 `//go:build linux`，引用只在 `wayland_csd_painter.go`（`//go:build linux`）里定义的 `csdBitmapGlyphH` |
| B3 | `gpu/gwgpu/gles` 在 darwin 编译失败 | `gpu/gwgpu/gles/command_encoder.go:43` 等未打标签共享文件引用 `gl.Context`/`Command`/`RenderPipeline`/`Buffer`/`Texture`/`BindGroup` | darwin | 包实际只支持 `linux || windows`（`gl` 只有 `context_linux.go` 与 windows 的 `context.go`），未打标签文件没有同等约束 |
| B4 | js/wasm 下 214 个包加载失败 | `render/backend.go:23`、`render/present_construct.go:25`、`render/present_frame.go:24`、`render/present_target.go:16`、`render/present_swapchain_gl.go:18` | js/wasm | `render` 无条件注册/导入 gles；wasm 下 `gpu/gwgpu/gles/gl` 无任何文件，`render` 及其全部下游断链 |
| B5 | `gpu/rwgpu` 在 windows 编译失败 | `gpu/rwgpu/adapter.go:203`、`bindgroup.go:288,364`、`buffer.go:311,342`、`command.go:360,379,509,557,583` | windows | 共享代码用 `unixProc`/`call1..call5`，只在 `loader_unix.go`（`linux || darwin`）定义；`loader_windows.go` 只有 `Library`/`Proc.Call`，没有等价自由函数。webgpu 的 facade 依赖 rwgpu，故 Windows webgpu 无法编译 |
| B6 | `gpu/gwgpu/ffishim` 在 windows 编译失败 | `gpu/gwgpu/ffishim/ffishim.go:113,122,131` | windows | 使用 `purego.Dlopen`/`Dlsym`/`Dlclose`（purego 仅 unix 提供）；Windows 需走 `purego.LoadLibrary`/`GetProcAddress` 或按平台拆文件 |

## 结论

- P1.1 的采集目标完成：六目标 × 默认/nogpu × 原始/过滤/库范围的退出码与失败清单全部落盘。
- linux/amd64、linux/arm64 默认标签主体绿；库本身在 nogpu 下绿。
- 四个目标（windows、darwin×2、js）存在真实构建红项（B2–B6），`-tags nogpu` 的消费方导入红项（B1）跨全目标一致。
- 这些红项不改变 Linux 行为，但阻塞 PLAN §7.7「构建标签回归」与 P2 每子步的「全平台编译」门禁；
  修复需改代码，不能在 P1 冻结期内做，建议单列任务排到 P2 起点前（详见台账 P1.1 执行结果与报告）。
