# game_input（S60 输入窗，双 case）

S60 原话：新建 `examples/game_input --case=remap/combo`，调 `engine/input` 真包。
测试：action/buffer 单测＋自动改键即用连招不丢 JSON＋人工按。

本窗只调包、不改包：所有输入语义来自 `engine/input`（`Map` 动作映射、
`Buffer` 按压缓存、`TouchTracker` 多点跟踪）；本目录只有喂数与画面。

## 跑法

```sh
go run ./examples/game_input --case=remap -auto-only
go run ./examples/game_input --case=combo -auto-only
go run ./examples/game_input --case=remap -manual-seconds 30
go run ./examples/game_input --case=combo -manual-seconds 30
RUN_SECONDS=8 go run ./examples/game_input --case=remap -auto-only
```

无窗冻结金图（只写缺失文件、不开窗，第二遍即比对）：

```sh
go run ./examples/game_input --case=remap -write-golden
go run ./examples/game_input --case=combo -write-golden
```

`--case` 只认 `remap`/`combo`，其他拼写直接报错退出。

## 两 case 说明

- `remap`（13.1 动作映射）：左栏 6 个动作强度条（跳/攻/左/右/上/下），
  中盘为移动向量箭头，右栏为末键/死区/改键/脚本计数。
  自动窗走 7 步脚本：1 秒空格按（同 tick 读数=1，即 0 帧延迟）、
  2 秒松开归零、3 秒把 attack 改绑到 75（动作表不变，即改键不改码）、
  3.5 秒旧键 74 按下无声、4 秒新键 75 按下即响、
  5 秒死区调到 0.6（漂移杆被门住、数字键仍满格）、6 秒清零归零。
- `combo`（13.2 输入缓冲）：左栏 64 格缓存条（亮黄=窗内有效、
  暗红=已过期未剪、灰=空）加触点蓝块，中盘同左，右栏为连/丢/满/零计数。
  自动窗走固定逻辑时钟（`comboBase=10000ms`，结论不随墙钟抖）：
  jump@base、attack@base+50，先后取中=连招 2 中；
  300ms 后取旧=过期丢 1；70 次压入只留 64（满 64 不涨）；
  Clear 后长度与有效数双零（终态归零）；
  触点 Begin/Move/End 后未知 End 报错但跟踪器照用（断触不崩）。

人工按：空格=跳，`j`=攻，`wasd`（大小写都行，小写是窗里配的方便键，
探针不受影响）=移动；鼠标点按走 0 号触点，真触屏走触点号；
每次真事件都打日志并改标题计数，关窗或超时后出汇总 JSON。

## 门（-auto-only 内置，任一条红即 exit 1）

| 门 | remap | combo |
|---|---|---|
| 自检三证据 | 19 快照＋4 向量＋像素＋金图全过 | 6 序列＋3 触点序列＋像素＋金图全过 |
| wrgate | `MinPresents: 1`（标准 schema 门全带） | 同左 |
| 脚本 | 7/7 步全中，`latency_frames=0` | 连中 2、过期丢 2 类、`cap64=64`、`terminal_zero=1`、`touch_ok=1` |
| 金图 | `testdata/input_remap_golden.png` 零容差 | `testdata/input_combo_golden.png` 零容差 |

像素探针容差 `probePixelTol=8` 写死在 main.go，金图比对零容差。
帧门（presents400＋/单窗 fps57＋/p95 进 16ms/双金 0 差）见
`docs/RENDER_2_5D_GAP_PLAN.md` §5，中央正式包三遍最差时再判，
本窗只负责把数如实吐进 `ability_extra`。

## 文件

- `main.go`：双 case 窗＋无窗探针（逻辑/像素/金图）。
- `testdata/input_remap_golden.png`：remap 静帧（首遍 `-write-golden` 落盘）。
- `testdata/input_combo_golden.png`：combo 静帧（同上）。
- `README.md`：本文件。

冻结数来源（窗里不写死用户值）：动作表与向量来自
`engine/input/testdata/action_cases.json`（19 快照/4 向量），
缓存与触点来自 `engine/input/testdata/buffer_cases.json`
（限额 64/10/150ms）；窗内小写 wasd/j 方便键是示例层演示接线，
探针用文件原表。
