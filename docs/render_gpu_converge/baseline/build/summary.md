# P1.1 六目标交叉编译基线（2026-10-28，P1.1b 修复后刷新）

> 原始输出：`baseline/build/<os>-<arch>.txt`。环境：go1.25.5 linux/amd64，`CGO_ENABLED=0`。
> 本目录在 P1.1 采集后，已由 P1.1b（B1–B6 跨平台构建修复）刷新为**修复后基线**；
> 修复前（4 目标红）的原始输出留在提交 `0b97eeb3` 的同一路径下，可随时 `git show` 对照。
> 修复过程与前后对照见 `evidence/P1.1b/summary.md`。

> 说明：`go build` 碰到包加载错误时只打印一个代表包，所以每份证据同时用
> `go list -e -f '{{.ImportPath}}|{{range .DepsErrors}}{{.Err}} ; {{end}}'` 枚举全部加载失败（各文件 §6/§7），
> 编译错误（undefined 等）以 `go build` 原始输出为准（§3/§5/§8）。

## 执行命令

每个目标按同一脚本跑 8 段，`os/arch` 见文件名：

1. `GOOS=<os> GOARCH=<arch> CGO_ENABLED=0 go build ./...`（原始，预期含已知问题包）
2. 两个已知问题包单独构建：`./examples/ui_wr_t_stress`、`./ui/scene/internal/gpupixel`（预期红，登记）
3. `go build <go list -e ./... 去掉两个已知问题包与零 Go 文件目录>`（默认标签主体）
4. `go build -tags nogpu ./...`（原始）
5. `go build -tags nogpu <同 3 的过滤>`（nogpu 主体）
6. `go list -e ...` 默认标签全部加载失败清单（去已知问题包）
7. 同 6，加 `-tags nogpu`
8. `go build -tags nogpu <库范围：去掉 examples/、tools/ 与已知问题包>`（隔离“消费方导入”与“库本身”）

> 第 3/5/8 段会先从清单里去掉「零 Go 文件目录」（如 darwin 下的 `gpu/gwgpu/gles/egl`、`examples/ui_ime_probe`），
> 否则显式列出会报 `build constraints exclude all Go files`，那是平台空目录的正常状态，不是构建缺陷。

## 结果（修复后）

| 目标 | 默认主体（§3） | 默认加载失败（§6） | nogpu 主体（§5） | nogpu 加载失败（§7） | nogpu 库范围（§8） | 结论 |
|---|---|---|---|---|---|---|
| linux/amd64 | 351 包 **PASS** | 0 | 347 包 **PASS** | 0 | 141 包 **PASS** | ✅ |
| linux/arm64 | 351 包 **PASS** | 0 | 347 包 **PASS** | 0 | 141 包 **PASS** | ✅ |
| windows/amd64 | 357 包 **PASS** | 0 | 353 包 **PASS** | 0 | 140 包 **PASS** | ✅ |
| darwin/amd64 | 349 包 **PASS** | 0 | 345 包 **PASS** | 0 | 140 包 **PASS** | ✅ |
| darwin/arm64 | 349 包 **PASS** | 0 | 345 包 **PASS** | 0 | 140 包 **PASS** | ✅ |
| js/wasm | 不支持（登记） | — | 不支持（登记） | — | 不支持（登记） | 见 `js-wasm.txt` |

两个已知问题包在所有目标上均按预期红：`ui_wr_t_stress`（`ui/kit/button`、`ui/kit/icon` 不存在）、
`ui/scene/internal/gpupixel`（`no non-test Go files`）。这两项不属 P1.1b 范围，仍是基线已知问题。

## 红项 B1–B6（P1.1b 已修，保留登记以便回溯）

| 编号 | 修复前现象 | 位置 | 修复 |
|---|---|---|---|
| B1 | `-tags nogpu` 下 163 个包加载失败（161 examples + 2 tools） | 各 `main.go` 的 `_ "…/render/gpu"`；根因是 `render/gpu` 在 nogpu 下成为「无文件包」 | `render/gpu` 增 `//go:build nogpu` 空实现；`stage_chase` 的 GPU 专用调用按标签拆分 |
| B2 | `ui/platform` 在 darwin/windows 编译失败 | `wayland_csd_font.go:36,203,207` | 文件与测试补 `//go:build linux` |
| B3 | `gpu/gwgpu/gles` 在 darwin 编译失败 | `command_encoder.go` 等 5 个未打标签共享文件 | 补 `(windows \|\| linux) && !(js && wasm)` |
| B4 | js/wasm 下 214 个包加载失败（gles 链） | `render/backend.go`、`present_construct.go`、`present_frame.go`、`present_target.go`、`present_swapchain_gl.go` | 合并到 `render/register_gles.go` 并按平台门禁 |
| B5 | `gpu/rwgpu` 在 windows 编译失败 | `adapter.go`、`bindgroup.go`、`buffer.go`、`command.go` | `loader_windows.go` 补 `call1..call6`/`resolvedFnPtr`，共享代码去掉 `*unixProc` 断言 |
| B6 | `gpu/gwgpu/ffishim` 在 windows 编译失败 | `ffishim.go:113,122,131` | 拆 `libload_unix.go` / `libload_windows.go` |

## js/wasm：登记为不支持（2026-10-28 用户决定）

- 现象：排除 `ui_wr_t_stress` 后，345 个包整体编译在 `../purego/struct_other.go:21:46: undefined: syscallArgs` 处失败。
- 根因：`../purego` 只支持 `darwin || freebsd || linux || netbsd || windows`，没有 wasm 实现；
  `gpu/render/ui/engine/video` 栈 302 个包都传递依赖它，只有 44 个叶子包能单独编译。
- 结论：wasm 不做基线必绿目标；将来要做，须为浏览器 WebGPU 新写 `syscall/js` 后端（单独立项）。
- 原始输出：`js-wasm.txt`；复跑细节：`evidence/P1.1b/js-wasm.txt`。

## 结论

- 五个真机目标（linux×2、windows、darwin×2）默认 + nogpu + 库范围全绿，只剩两个登记问题包。
- js/wasm 因外部 FFI 库无 wasm 实现，登记为不支持，不再是构建门禁红项。
- Linux 行为零变化：P0.4 双后端一致性测试台复跑，6 份 `report.json` 与 `summary.md` 与修复前逐字节一致
  （`evidence/P1.1b/conformance-rerun.txt`）。
