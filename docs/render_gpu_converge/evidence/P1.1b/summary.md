# P1.1b 跨平台构建修复：运行汇总

> 由 `/tmp/p11b-evidence.sh` 采集；每目标一份 `<os>-<arch>.txt`（8 段原始输出）。修复前基线见 `baseline/build/`（提交 `0b97eeb3`），本目录是修复后证据，同号输出会随基线一起刷新。

## 1. 背景与范围

P1.1 六目标跨界编译基线（2026-10-28）在 windows/darwin×2/js-wasm 上红，红项 B1–B6。用户决定在 P1.2 之前插队修复（P1.1b），修完复跑六目标，让 P1 基线 tag 全绿。

边界：只动构建标签/平台门禁与 Windows 加载层，不改 Linux 绘制、缓存、释放路径。

## 2. 修复清单

| 项 | 根因 | 改动 | 验证 |
|---|---|---|---|
| B1 | `render/gpu` 全部文件打 `!nogpu`，`-tags nogpu` 下变成「无文件包」，163 个消费方的空白导入直接加载失败 | 新增 `render/gpu/gpu_nogpu.go`（`//go:build nogpu` 空包），导入恒安全、默认构建不变；唯一调用 GPU 专用 API 的消费方 `examples/engine/stage_chase` 拆 `resetGPUAccelerator`（`!nogpu`/`nogpu` 各一份） | `-tags nogpu ./...` 由 163 包加载失败 → 0（仅剩已登记问题包） |
| B2 | `wayland_csd_font.go` 缺 linux 标签却引用 `csdBitmapGlyphH`（仅 linux 定义） | 文件与其测试补 `//go:build linux` | windows/darwin 不再报该包 |
| B3 | `gpu/gwgpu/gles` 五个未打标签文件引用 `gl.*`（`gl` 只有 linux/windows 上下文） | `command_compute.go`、`command_encoder.go`、`command_execute.go`、`command_renderpass.go`、`ffiutil.go` 补 `(windows || linux) && !(js && wasm)` | darwin 下该包不再编译失败 |
| B4 | `render` 四处重复 `_ ".../gles"` 无条件注册，gles 无 darwin/wasm 实现 | 合并到 `render/register_gles.go`（由只剩注册一行的 `present_target.go` 改名而来）并打同一平台标签；`backend.go`/`present_construct.go`/`present_frame.go` 去掉重复导入；`BackendGo` 注释写明仅 linux/windows 可用 | darwin/wasm 下 render 不再拉起 gles |
| B5 | `gpu/rwgpu` 的 `unixProc`/`call1..call6` 只在 `loader_unix.go` 定义 | `loader_windows.go` 补 `call1..call6`（走 `syscall.LazyProc` + `syscall.SyscallN`）与平台无关的 `resolvedFnPtr`；`adapter.go`、`device_misc.go` 的 `*unixProc` 断言改为 `resolvedFnPtr`；顺带修 `gpu/rwgpu/examples/mrt/main.go` 的 `rrwgpu.` 拼写（13 处，Windows 示例从未编译过） | windows 下 rwgpu 与 mrt 示例编译通过 |
| B6 | `ffishim` 用 `purego.Dlopen/Dlsym/Dlclose`（仅 unix） | 拆成 `libload_unix.go`（`purego`）与 `libload_windows.go`（`golang.org/x/sys/windows` 的 `LoadLibrary`/`GetProcAddress`/`FreeLibrary`） | windows 下 ffishim 编译通过 |

## 3. 修复后结果（`<os>-<arch>.txt`）

| 目标 | 默认主体 | `-tags nogpu` 主体 | nogpu 库范围 | 结论 |
|---|---|---|---|---|
| linux/amd64 | 351 包 PASS | 347 包 PASS | 141 包 PASS | ✅ |
| linux/arm64 | 351 包 PASS | 347 包 PASS | 141 包 PASS | ✅ |
| windows/amd64 | 357 包 PASS | 353 包 PASS | 140 包 PASS | ✅ |
| darwin/amd64 | 349 包 PASS | 345 包 PASS | 140 包 PASS | ✅ |
| darwin/arm64 | 349 包 PASS | 345 包 PASS | 140 包 PASS | ✅ |
| js/wasm | — | — | — | 登记为不支持（见 `js-wasm.txt`） |

说明：
- 「原始 `./...`」段仍为 exit 1，唯一原因是已登记问题包 `examples/ui_wr_t_stress`（引用不存在的 `ui/kit/button`、`ui/kit/icon`）；`ui/scene/internal/gpupixel` 仍只有测试文件。两者不属本任务，保留原样。
- 默认/nogpu 的加载失败计数（第 6/7 段）均为 0（已排除上述两个问题包）。
- 包数比 P1.1 多，是因为 P1.1 的包清单含「零 Go 文件目录」，显式列出时会报 `build constraints exclude all Go files`（如 darwin 下的 `gles/egl`、`examples/ui_ime_probe`）；本目录清单已过滤零文件目录，只统计真正参与编译的包。

## 4. js/wasm（用户决定：登记为不支持）

- 现象：排除 `ui_wr_t_stress` 后，345 个有 Go 文件的包整体编译在 `purego` 处失败：`../purego/struct_other.go:21:46: undefined: syscallArgs`。
- 根因：`../purego` 的构建标签只覆盖 `darwin || freebsd || linux || netbsd || windows`，没有任何 wasm 实现；`gpu/render/ui/engine/video` 栈 302 个包都传递依赖它，只有 44 个叶子包能单独编译。
- 决定（2026-10-28）：wasm 不再作为基线必绿目标；将来若需要 wasm，须为浏览器 WebGPU 新写 `syscall/js` 后端（等同新增后端能力，单独立项）。
- 原始输出与依赖面统计：`js-wasm.txt`。

## 5. 行为零变化守卫（Linux）

P0.4 双后端一致性测试台复跑：3 场景 × native/go 全 PASS，6 份 `report.json` 与 `summary.md` 与基线**逐字节一致**（含 K1 的 +2 条/+131072 字节账本放宽值）。详见 `conformance-rerun.txt`。

## 6. 旁证：rwgpu 测试段错误是既存问题（非 B5 引入）

复跑 `go test ./gpu/rwgpu/` 时 `TestQueue_OnSubmittedWorkDone` 崩溃，为区分是否由 B5 改动引入，做了修复前/后同一用例的对照：

| 树 | 首行结果 | 崩溃栈（自上而下） |
|---|---|---|
| 修复前（HEAD `0b97eeb3`，隔离 worktree） | `queue_workdone_test.go:45: OnSubmittedWorkDone: wgpu returned null future` | `SIGSEGV PC=0x1` → `unixProc.Call`（`loader_unix.go:96`）→ `Device.Release.func1`（`device_misc.go:314`）→ `releaseNativeHandle`（`safety.go:285`）→ `Device.Release`（`device_misc.go:313`）→ `queue_workdone_test.go:31` cleanup |
| 修复后（本工作区） | 同一行、同一错误文案 | 完全相同的帧序与行号；`releaseNativeHandle` 第三参回调地址同为 `0x6a03c0` |

- 结论：与 B5 无关。B5 只改了 `RequestAdapter`/`RequestDevice` 两处 `*unixProc` 断言（换成 `resolvedFnPtr`），崩溃走的是 `Device.Release` 的 `procDeviceRelease` 路径，未触及。
- 已按 P1.3 既有问题 **T1** 登记到台账：P1.3 逐文件按 FAIL 记录，释放路径健壮性归 P2.4c/P2.4e，P1 冻结期内不改。
- 复现命令：

```bash
GOWORK=off WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so \
  go test ./gpu/rwgpu/ -run '^TestQueue_OnSubmittedWorkDone$' -count=1 -v
```

## 7. 复现

```bash
# 单目标 8 段采集（输出到指定目录）
/tmp/p11b-evidence.sh linux/amd64 /tmp/p11b-evidence

# 一致性复跑
WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so \
GPUI_CONFORMANCE_OUT=/tmp/p11b-conformance \
  go test ./render -run '^TestConformanceDualBackend$' -count=1 -v
```

> 采集脚本为本机临时脚本，未入库；命令口径与 `baseline/build/summary.md` 一致，可照此重写。
