# E3 评估：6 轮 NO_JSON/BUILD_FAIL 复核——2 随 C2 销账、2 契约错配、2 陈旧不复现（2026-11-11）

> 任务：§2.2 待办池 E3（P1.4/P1.7：6 轮采不了档沿用 P1.4 登记，随 C2 及对应窗修复）。
> 性质：诊断复测，零生产代码改动。停下报告待拍板处置。

## 1 逐轮定性（P1.4 → 现行树 2026-11-11）

| 轮 | P1.4 状态与真因 | 现行树复测 | 定性 |
|---|---|---|---|
| t_stress native/go | BUILD_FAIL：`binary missing`（C2：引用不存在的 `ui/kit/button`/`ui/kit/icon`） | `examples/ui_wr_t_stress` 目录不存在 | **随 C2 销账**（示例整删，2026-11-09 已实证；BUILD_FAIL 需求随之消失） |
| zz_soak_control native/go | NO_JSON（stderr 有健康 CTRL 指标：presents 10229+、p95 18–20） | 显式 Xwayland :3，`CTRL_WINDOWS=2 RUN_SECONDS=20`：rc=0，2 窗 ~56fps、p95 19.82/20.59、hitch 1/1，**stdout 0 字节** | **设计上就不出 JSON**：无 wrgate import、无 JSON 输出路径（main.go 137 行），多窗 soak 控制窗，指标走 stderr CTRL 行——P1.4 采集器「每窗必有 JSON」的预期对它不成立，**契约错配非故障** |
| r7b go | NO_JSON：`eglMakeCurrent failed: 0x3002`（Xwayland :3） | 同环境 X11 轮：backend=x11、JSON 完整、p95 18.84、rc=1 仅 golden 漂移 2.5134% | **陈旧不复现**（gles X11 MakeCurrent 已正常；Wayland 路 E1 已证） |
| r15_soak go | NO_JSON：`CreateSurface(xlib): eglCreateContext failed: 0x3000`（Xwayland :3） | 同环境 X11 轮 + 会话 Wayland 轮：两档 JSON 完整、跑满 60s，rc=1 仅设计内关闭档检查（RUN_SECONDS 60<300） | **陈旧不复现**（gles X11 建共享上下文已正常；P1.6「GL 4 窗 MakeCurrent 跑不了」同族，同判） |

## 2 复测命令与环境

```
go build -o /tmp/e1/<窗名> ./examples/<窗名>
Xwayland :3 -rootless &          # 显式隔离嵌套 X server（非桌面 Xorg；会话为 GNOME Wayland）
env -u WAYLAND_DISPLAY DISPLAY=:3 GPUI_BACKEND=go RUN_SECONDS=60 /tmp/e1/<窗名> -auto-only
GPUI_BACKEND=go RUN_SECONDS=60 /tmp/e1/<窗名> -auto-only        # Wayland 对照
env -u WAYLAND_DISPLAY DISPLAY=:3 RUN_SECONDS=20 CTRL_WINDOWS=2 /tmp/e1/zz_soak_control
```

原始输出：`/tmp/e1/e3_*.{json,out,stderr}` → 归档 `evidence/P2.6/e3/`。r15_soak 的 JSON 打印在门禁检查之前（`main.go:473` 先于 `:491` 时长 FAIL），60s 短跑即可完整验证建面+出报告。

## 3 归因判读

1. **GL/X11 EGL 失败族（r7b/r15，P1.6 同族 4 窗）陈旧**：0x3000（CreateContext）/0x3002（MakeCurrent）在现行树不复现。时间窗（2026-10-29 → 11-11）内 GL 路已合入：P4.1 GLES resolve READ 绑定修复 + glClearDepth double 签名修复（`gpu/gwgpu/gles/`）、M5 格式参数化 22 文件（含 gles capabilities）；亦不排除 P1.4 采集环境 contention（该轮 c3 elapsed 92–211s 异常同期）——**未 bisect，候选留档**（同 E1 口径）。
2. **zz_soak_control 契约错配**：`zz_` 前缀窗本就在 wrgate 门禁体系外（P0.2 仍按「开窗即真窗」计入 162），采集器按 JSON 统一判档对它过严。其 stderr CTRL 指标两次采集（P1.4、本轮）均健康。
3. **BUILD_FAIL 两轮无独立问题**：C2 已实证销账。

## 4 拍板与处置（2026-11-11，用户拍板方向 A）

- **方向 A（定性销账，零代码）已执行**：t_stress×2 记随 C2 销账；`baseline/windows.md` §7 修订表追加 zz_soak_control「设计上不出 JSON、按 stderr CTRL 指标判档」勘误（表内条目只增不改）；r7b/r15 go 记陈旧不复现；原始输出归档 `evidence/P2.6/e3/`；E3 ✅。
- 方向 B（改窗适配采集器）、C（bisect EGL 根因）未采纳；候选改动清单（§3.1）留档供回溯。

## 5 顺带发现

- r7b golden 漂移 2.5134% 在 X11 轮同值复现（E1 已归 22 窗漂移专项）。
- r15_soak go X11/Wayland 两档 p50≈17.3–17.5（vsync 锁定）、无异常尾巴（60s 短跑，未过 300s 全档，不作为 soak 结论，仅证管线健康）。
