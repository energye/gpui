# testdata — 标准数据与场景说明

## 核心思路

**一份场景说明（画法清单）→ Go 与 CanvasKit 都能画 → 和标准图比较。**

```text
testdata/scenes/*.json     # 画什么（通用）
       │
       ├─ stdgate/scene (Go/gpui)  → actual PNG
       └─ oraclejs (CanvasKit)     → 优先作为标准图
              │
testdata/refs/standard.json + standard/*.png
```

## 目录

| 路径 | 含义 |
|------|------|
| `scenes/*.json` | 场景说明（画法） |
| `fonts/DejaVuSans.ttf` | 两边共用字体 |
| `refs/standard.json` | 标准索引（容差、oracle） |
| `refs/standard/*.png` | 标准像素 |
| `oraclejs/` | CanvasKit 生成器 |

Go 包（不在 testdata 下，避免 go 忽略）：

| 包 | 作用 |
|----|------|
| `stdgate/scene` | 读场景、用 gpui 画 |
| `stdgate/compare` | 像素比较 |
| `stdgate/cmd/scenerun` | CLI 画场景 |
| `stdgate/cmd/refcapture` | 从 tmp/comp 装标准图（旧路径） |

## 场景 op（第一批）

`clear` `fill_rect` `fill_rrect` `fill_circle` `stroke_line`  
`clip_rect` `clip_rrect` `clip_path` `reset_clip`  
`layer_begin` `layer_end` `set_blend`  
`fill_text` `push` `pop` `translate` `scale` `rotate`  
`draw_image_solid`

## 生成 / 更新标准数据

```bash
# 1) gpui 按场景出图
export WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
export LD_LIBRARY_PATH=$PWD/lib:$LD_LIBRARY_PATH
export DISPLAY=:1
go run ./stdgate/cmd/scenerun -dir testdata/scenes -out tmp/scenes_gpui -font-root .

# 2) CanvasKit 出图；够近则用 CK，否则用同一场景的 gpui 图（oracle=gpui-scene）
cd testdata/oraclejs && npm ci && node gen.mjs --from-scenes
```

## 测试

```bash
go test -count=1 ./stdgate/scene -timeout 120s
GPUI_REQUIRE_COMP_REFS=1 go test -count=1 ./render -run 'TestP1_Comp_D0' -timeout 120s
```

已接入场景说明的用例：D01 D02 D03 D04 D05 D07 + S01。  
其余 D 仍为历史 gpui-capture 标准图，可逐步改成场景说明。
