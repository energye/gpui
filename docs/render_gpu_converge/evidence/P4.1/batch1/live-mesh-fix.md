# P4.1 批 1 补正：mesh 窗 live 帧丢网格（2026-11-10）

> 用户人工窗发现：`ui_wr_p41_mesh_paint_blend` 四张卡只看到底色（1 白 / 2 蓝 / 3 灰 / 4 白），
> 没有任何三角形。修法为示例层 `pc.DC` 契约，不动引擎。

## 0 根因

- `examples/ui_wr_p41_mesh_paint_blend/main.go` 的 `drawGouraudMesh` / `drawSolidMesh`
  用 `pc.LiveContext()` 取 `*render.Context`；live 帧的 OnPaint 拿到的是 **Recorder**
  （`recordFullFrame → presentRecordedFrame`），`LiveContext()` 断言失败后直接 return
  → live 帧只剩卡片底色。快照路径的 `pc.DC` 是活 Context，`LiveContext()` 返回它，
  所以金图/探针一直是绿的，漏掉了 live 帧。
- 同批其它四扇 p41 窗已按 P4.1 批 4 补正改成 `pc.DC`；本窗是漏网的一扇。
  `p39/p39a/p39b` 的 `LiveContext()` 用法有 `pc.DC.DrawMeshGradient/DrawMeshPatch`
  else 回退，不受影响。

## 1 修法（示例层，标准 Canvas 契约）

两个 helper 改为 `dc := pc.DC`（`render.Canvas`）：live Recorder 录
`DrawVerticesCommand`/`DrawMeshCommand`，快照 Context 直画；与
`ui_wr_p41_evenodd_msaa/main.go:77` 的补正注释同一口径。无引擎/API 改动。

## 2 证据

| 项 | 修前 | 修后 |
|---|---|---|
| `ui_wr_p41_mesh_paint_blend -auto-only` `picture_op_count` | 63 | **67**（live 录制多 4 条 mesh 命令） |
| `gpu_ops` | 22687 | **23654**（live 帧真的回放网格） |
| logic probes | 6/6 | 6/6 |
| pixel A/C/D | 过 | 过 |
| pixel B_plus_centroid | FAIL（got 纯蓝） | **仍 FAIL**（P4.1 #3 GPU Porter-Duff 层，非本修） |
| golden | 0.0000% | **0.0000%**（快照路径不变，未重种） |
| `go build ./...` / `go vet` | — | ✅ |

## 3 影响面

- 只改 `examples/ui_wr_p41_mesh_paint_blend/main.go` 两处；快照/金图/探针不变；
  live 帧 A/C/D 现在会画出三角形，B 仍纯蓝直到 #3 收口。
- 其它 `LiveContext()` 示例（p39/p39a/p39b）有 `pc.DC` 回退，不受影响。
- 无公开 API / catalog / apidoc / recording 改动。

## 4 边界

- `DrawVertices`/`DrawMesh` 的 **live 回放**（Recorder→Playback→CanvasBackend）
  没有专门 round-trip 测试；命令与分发链已在（`recording/recorder.go:339-341`、
  `backend_canvas.go:455,462`）。本次靠 `picture_op_count` + `gpu_ops` + 人工 live
  窗确认；测试缺口登记 P4.1 台账。
- live FBO 抓帧本机 Wayland 不可用（同 P4.3 记录），人工窗由用户看 A/C/D 三角形。
