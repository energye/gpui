# M2 销账：rrect4 go 档角点 1-LSB 分叉已被 P4.1 批 4 顺带消除

> 台账来源：§2.2 M2（D12 对账登记 2026-11-03：「p35_rrect4 go 档 0.0038%
> 29px 1-LSB，gles 角点覆盖率量化分叉，非 D 批引入」）。销账日期 2026-11-11。

## 复现与三点二分（2026-11-11，独立 worktree，同显示同参数）

| 提交点 | golden | 说明 |
|---|---|---|
| `17574a71`（D 批 HEAD，M2 登记引用点） | **0.0038%** | 分叉复现，与登记逐值一致 |
| `4f8b4381`（P3.9，GLES baseVertex 修复） | 0.0038% | 仍在 |
| `9798875b`（**P4.1 批 4**：MSAA even-odd stencil + GLES 4x 链三洞修复） | **0.0000%** | **分叉消失** |
| HEAD（`aafaffdc`+）主树今天 go 档 | 0.0000% | 现状绿 |

⇒ 修复者提交 = **`9798875b`**（P4.1 批 4 收口）。该批的 GLES 链修复中，
**glClearDepth/glDepthRange double 签名按 float32 传参的修复（「深度门控
从未生效，1x 被 mask 掩盖」）为主嫌**：ClipRRect 的深度门控路径修复前从未
真正生效，rrect4 角点像素走的是失效门控下的覆盖路径 → 1-LSB 量化差；
门控修复后角点与 native 一致。`9798875b` 是原子提交，不再内部拆分归因。

## 处置

- M2 行 ✅ 销账：分叉已消除（三点二分 A/B 证据），无需新代码；金图未重种
  （单份 `p35_rrect4_final.png`，P3.5 后未动，今天 go 档 diff=0.0000%）。
- 「gles 角点覆盖率量化」主题的剩余风险面并入 SDF 覆盖线观察
  （GPU 覆盖量化的族，K2 改判同日已登记 morphology 传播验证项）。
- 附注：worktree A/B 需要 go.work（父级 go.work 未跟踪、只列 ./gpui）——
  worktree 内手建 `use (.; ../purego)` 后可跑；两次教训（父级 go.work、
  cp 到 worktree 的相对路径）已记。

## 门禁

- 单窗标准关闭（RUN_SECONDS=30 自退出，wayland :0），每点退出干净。
- 台账 M2 行同步销账；§7 追加流水。
