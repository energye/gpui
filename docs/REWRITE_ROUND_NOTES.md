# 本轮重写记录（2026-09-08）：输入框闪烁/上屏慢/补丁清理

> 状态：本轮改动**未能解决用户问题**，用户决定整体回滚后重修。
> 本文档与 `docs/RETAINED_DEFAULT_AUDIT.md`（截至 09-08 的审计地图）配合使用，
> 是新会话重修的输入。行号以 2026-09-08 树为准，漂移后按函数名找。

## 0. 用户诉求原文要点

- 真窗（`examples/ui_text_edit_accept`，6 个输入框 A–F，retained 省帧默认）还闪，
  输入文字上屏慢。
- 怀疑引擎里补丁摞补丁，要求从根上按标准（Skia/Flutter）实现重写：
  原逻辑错就改原逻辑，错得多就重写，不要大量补丁。

## 1. 根因与改动对照（全部在引擎层，示例层零改动）

### R1 打字慢：A 框每帧自己弄脏自己（882×42 每帧重录）

- `ui/textinput/input_box.go`
  - 单行 `InputBox` 与多行 `MultiLineInputBox` 的 `OnPaint`（只画边框）尾巴各调一次
    `layoutCaret()`：画画着就挪光标、挪完标脏。已删（横滚 `ViewportInputBox` 本来就没有）。
  - 两个 `layoutCaret()` 里改光标条高度原来直接写字段不标脏，改成真变了才标脏。
- `ui/textinput/viewport_input.go`：同上，三个 `layoutCaret()` 分支的高度改动加标脏。
- `ui/rendering/box.go`：`RenderColorBox.MoveTo` 挪到相同坐标也标脏，加相同值早退
  （`SetAlpha` 早就有早退），另加空指针保护。

### R2 稳态从不空闲：包壳盒子的脏标记永远清不掉（本轮最深的根）

- `ui/rendering/node.go`
  - `baseOf` 只认具体类型，三个输入框（包一层 `*RenderBox` 的新类型）不在名单。
    标脏走提升方法能标上，清脏走 `baseOf` 认不出，`ConsumeNeedsPaint` 每次跳过，
    一次变脏脏一辈子，边界每帧重录。
  - 修法：`Base` 加 `base()` 方法，`baseOf` 先认接口、带空保护，认不出再走类型名单。
    以后包壳不用改名单。`EnsureCacheID` 只在 `Base` 实现了一份，包壳走哪条路拿的键相同。

### R3 present 的 Load 规则两兄弟写法相反

- `render/internal/gpu/render_session.go`（blit 贴图路）：`Load` 条件原来是
  `s.frameRendered || hasDamage`，新缓冲配损伤去 Load 读未定义显存。
  删掉 `|| hasDamage`，与分组路（R6 已修好的那条）统一：只有同缓冲已画满才 Load。
  帧缓冲（frameScratch，帧内先画后取）那处故意没动，是另一套语义。
  R6 注释同步改了一句。

### R4 鬼影：离屏录制空内容不提交，旧像素永久贴在那

- `render/internal/gpu/render_session.go` 新增 `ClearView`（只清屏不画画）。
- `render/internal/gpu/gpu_render_context.go`：`Flush` 空队列分支，
  仅当录制子通道（`offscreen.active`）且有 GPU 会话时走 `ClearView`。
  主上屏空队列还是“什么都不交”，纯 CPU 上下文走不到（要 `rc.session != nil`）。
- `ui/scene/textured.go`：透明标记整套删除（字段、函数、两个录制分支、查询分支）；
  `Has` 回到“有纹理才算命中”；损伤改成“这帧重录过就报”（空内容层也不跳过，
  否则删空文本那次留 ghost）；新 bounds 为空时保留老 bounds，保证过渡帧有损伤。
- `ui/scene/picture.go`：`VisibleContent` 预判删除（删完与主干一致）。
- 单测：`ui/scene/textured_transparent_internal_test.go` 重写成“空和非空走同一条
  分配路”；`ui/textinput/unfocus_caret_internal_test.go` 只改一行注释。

### 刻意保留、没动的部分（当时判为 load-bearing 或方向正确）

- `scrollRecordWindow`（巨行纹理内存必需）、`postResizeFull=3`（审计 §四认定的双预算）、
  C2/C3/C4 的全部 `sched()`、H-B/H-C、C7 回绕、B3/B5/B6、O2、A5。

### 诊断探针：全删光，树里一条不剩

`FRM`/`BLIT`/`SURF`/`CLR`/`PRS*`/`GPUI_NOCLR`，`grep` 确认过。
`render/context.go` 与 `render/present_target.go` 里上轮残留的诊断打印顺手清掉。

## 2. 验证数据（修完当时实测）

- 稳态 1717 帧里 1380 帧零脏（之前 0%）；882×42 从每帧重录降到真实改动那 60 次。
- 6 秒快照：exit 0，六框与 B/C/F 预填字全对，fps 59，`paint_count=3`。
- 打字：`h` 按下 150ms 内上屏；切 A 到 B 后 A 区零残留（连看多帧）。
- `go build ./...` 过；`gofmt` 干净；`go run ./scripts/apidoc` 绿。
- `go test ./ui/...` 除 `TestKeystrokeRatio_M5` 全绿（M5 是微秒计时门，
  干净头上也连红，机器忙就抖，见审计 §七）。
- `go test ./render/` 另外 6 个红：4 个干净头同红（环境）；`BlendHue`/`S69` 用
  “干净头只加 blit 那行改动”对照过，照过不误，是当时工作区另一批
  （同问题线、早于本轮的）未提交改动挂的——回滚时注意区分，见 §4。

## 3. 未解：变暗闪（停手点，原样交接）

- 现象：背景压黑、亮字不动，几十毫秒一次，爱跟在光标翻转附近，
  120–200 连抓里 2%–17% 不等（抓得越密单次越少、慢抓 200ms 间隔 7/40）。
- 已排除到哪：毫秒级日志对过，出事帧引擎决策（清/留、损伤、录了几个纹理）
  与前后正常帧一字一样。`ClearView` 的 A/B 开关思想实验没来得及做。
- 第一嫌疑（未定罪）：录制和主上屏共用会话里的纹理绑定槽，背景大纹理被踩；
  字走字形管线所以没事。第二嫌疑：合成器/抓屏竞态。
- 抓屏证据在 `/tmp/rwwatch`（f/g/j/k/m/s 六批，`k/m/s_manifest.txt` 带毫秒时间戳），
  二进制 `/tmp/edit_rewrite`。/tmp 重启会丢，要用先拷走。
- 建议的下一步（当时想问没问）：B 先做 render 层内录制与主上屏帧状态彻底分离
  （标准做法，风险可控，还能让 Load 真正吃上）；不行再进 `gpu` 查绑定缓存。

## 4. 回滚注意

- 本轮 13 个文件（见 §1）与更早同问题线的未提交改动混在同一文件里，
  `BlendHue`/`S69` 的挂就是早于本轮的改动，不是本轮 hunks。
- 回滚前建议整树先备份分支提交一次；`docs/RETAINED_DEFAULT_AUDIT.md` 与本文件是
  未跟踪/审计文档，别一起洗掉。
- `docs/ENGINE_UI_WIDGET_RENDER.md` §10 修订行本轮没加，重修合入时记得补。

## 5. 新会话开工顺序（建议第一句话就照着念）

1. 读 `AGENTS.md`，再读 `docs/ENGINE_UI_WIDGET_RENDER.md` §2/§3/§5/§10。
2. 读 `docs/RETAINED_DEFAULT_AUDIT.md`（老地图），再读本文件（本轮的新旧账）。
3. 先跑真窗复现基线（闪还在、慢还在），确认后再动手；`M5` 与 `render` 那几个红
   先当环境问题，别被带偏；修完跑长连抓（≥120 帧）再谈闪。
