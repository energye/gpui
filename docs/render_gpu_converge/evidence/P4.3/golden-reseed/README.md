# P4.3 drawshadow 金图重种记录（2026-11-10）

> 用户拍板 A（修滤波源帧内累积 + 重种金图）。本目录归档旧图/新图与逐项数据；
> 门禁金图路径仍是 `examples/ui_wr_p41_drawshadow/testdata/goldens/`。

## 0 为什么要重种

旧金图（07:55 采集）包含一个通用错误：同一帧内多笔滤镜共用的 `filterSrcView`
未清空，上一笔阴影剪影被带进下一笔，PopLayer 各自原位重画 → 阴影被叠加 1..N 次
（A≈5–6、B≈5–6、C1≈3–4、D1≈1.8、D2=1）。修复后阴影回到源 alpha 0.25
（CPU 契约，`context_misc_draw_test.go:189`），画面必然变化。根因与证据见
`../root-cause2.md`、`../root-cause2-fix.md`。

## 1 文件与 sha256

| 文件 | sha256 | 说明 |
|---|---|---|
| `p41_drawshadow_final_old_native.png` | `de362880b9e38462f279722dd1b226ae3709ba67213ba67e370b277dec405b1e` | 旧金图（native），07:55 |
| `p41_drawshadow_final_new_native.png` | `1727e489dd4c894d6346c040c11b69aec5fc2c01a42678220e8298985b0914b6` | 新金图（native），修复后 |
| `p41_drawshadow_final_old_go.png` | `1f1d4a01c8591ae1685eadc1cd7cbb2e3d242d2872d37b219faaf4f3671ba242` | 旧金图（`GPUI_BACKEND=go`） |
| `p41_drawshadow_final_new_go.png` | `29c7ca7e2fc6664ca4a091dc60d1b192d9b3e2fff9875e2ca18d4684e9da6546` | 新金图（`GPUI_BACKEND=go`） |

## 2 重种前后的差异（整图 1200×800 = 960000 px）

| 运行 | 旧→新差异 px | 占比 | max channel diff | 首跑 golden 门禁 | 二次跑 |
|---|---|---|---|---|---|
| native | 278666 | 29.0277% | 225 | 35.2519%（预期红） | **0.0000%** |
| go | 278677 | 29.0289% | 225 | 35.2519%（预期红） | **0.0000%** |

门禁百分比分母是 shell 内容区 763584 px；整图分母是 960000 px。

## 3 修复后的关键像素（证明是 25% 契约，不是调参）

| 点 | 旧 | 新 | 含义 |
|---|---|---|---|
| A 卡本体下缘 (500,268) | 53（α=0.792） | **191（α=0.251）** | 单次 25% 阴影 |
| A 卡本体 (500,160) | (3,3,7) | **(11,11,17)** | 本体不再被叠加阴影压暗 |
| C1 (400,510) | 100 | **192** | elevation 4 卡片回到 25% |
| C2 (400,625) | 102 | **189** | elevation 12 |
| D 黑 (850,540) | 154 | **191** | elevation 10 |

## 4 新 go 与 native 的差

仅 173 px，bbox `(239,741)-(397,775)` = 底部 HUD fps 文字（门禁 `ShellContentRects`
已遮蔽），不是画面差异。

## 5 复现命令

```bash
cd <repo>
WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so go run ./examples/ui_wr_p41_drawshadow -auto-only
GPUI_BACKEND=go go run ./examples/ui_wr_p41_drawshadow -auto-only
```

单窗、应用自己走 `Run → Close` 标准关闭；跑前跑后查 `xdpyinfo :0`。
