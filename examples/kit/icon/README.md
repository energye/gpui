# kit icon — F1 full-window icons (no chrome, metrics JSON only)

## 运行

```bash
export DISPLAY=:0 LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=8 go run ./examples/kit/icon
# 门禁自检（默认 8 秒：窗口缩放 excursion 恢复 + 2 秒稳态）：
go run ./examples/kit/icon -auto-only
```

窗口 1200×800 基线，可调大小。RUN_SECONDS>=5。
正确性窗：`slope_gate=off`（短窗，E 族只采不判）。
形态：整窗只摆被测图标，不设左侧栏与图形摆位壳，
指标只采 JSON 不上屏（见 `docs/antd/ACCEPTANCE.md` §7 窗形态）。
样子参考官网，功能与描述只认 Go 实现（无网页剪贴板/Copied）。

图标列表跟窗走：宽 = 窗宽-16，高从 492 铺到下边距；
窗口缩放时行盒撑满新约束，无重建，门禁带 `list_follows_window=true`。

## 窗内呈现（官网 demo 一一对应，共 20 个，默认 16/panda 32/间隔 8）

| 官网 demo | 内容 | 证明 |
|------|----------|----------|
| 基本用法 | HomeOutlined/SettingFilled/SmileOutlined/SyncOutlined-spin/SmileOutlined-rotate180/LoadingOutlined | ICO-01…10 |
| 多色图标 | SmileTwoTone默认#333/#E6E6E6/HeartTwoTone-#eb2f96/CheckCircleTwoTone-#52c41a | ICO-11 |
| 自定义图标 | HeartIcon-hotpink真形/PandaIcon-32px真形/Icon-component-HomeOutlined/HomeOutlined | ICO-12 |
| 离线iconfont | icon-tuichu/icon-facebook-#1877F2/icon-twitter（真形vendored） | ICO-13 |
| 多源 | icon-javascript/icon-java/icon-shoppingcart-后源赢/icon-python | ICO-14后源赢 |
| 全量目录 | 848全量减2个官网去重项，6分类6列卡片（图标36+名字实测居中） | bind<<total，tour>=3 |
| 卡片交互 | 悬停浅绿（字变绿-4+绿-1底+绿-4边）选中深绿（字变绿-8+绿-2底+绿-8边），双色图标主色同样走绿、次色按官方色板推，点击选中输出Go构造代码到日志；自检把选中和悬停留在同一排快照里 | hover_ok/select_logged |

目录分类与官网 `fields.ts` 同序：方向性/提示建议/编辑类/数据类/品牌和标识/网站通用；
`CopyrightCircle`/`DollarCircle` 按官网忽略。
转圈走组件 `Tick` 真转，每帧重绑，不管省动效开关（省动效下停转）。

## 调大小

脚本化走一次：2 秒时缩到 1040×700，4.5 秒时调回 1200×800。
门禁读数前要求 `resize_restored=true` 且回到 1200×800，
外加 `list_follows_window=true`（列表宽高 == 窗宽高推导值）。

## 门禁

| 门禁 | 阈值 | 类型 |
|------|------|------|
| policy | `present_policy == retained` 且 `damage_ratio <= 0.35`（稳态约 0.0003） | 硬 FAIL |
| presents | `present_count >= 1` | 硬 FAIL |
| 持续 tick FPS | `fps_interval >= 55` 且 `interval_p95_ms <= 22` | 硬 FAIL |
| 像素四探针 | 熊猫脸/对勾绿/热粉心/默认次色，全过 | 硬 FAIL |
| Golden | `testdata/showcase_icon_base.png` 零容差（仅静态区） | 硬 FAIL |
| 虚拟 | `bind << items`（约7/149）且 `tour>=3` | 硬 FAIL |
| 转圈 | spin 相位可推进 | 硬 FAIL |
| 悬停选中 | `hover_ok` 且 `select_logged`（Go构造代码落日志） | 硬 FAIL |
| 多源覆盖 | shoppingcart 走后源 f2 | 硬 FAIL |
| 调大小回基线 | `resize_restored=true` 且 `client_px=1200x800` | 硬 FAIL |
| vsync | `vsync_source` 非空 | 硬 FAIL |

巡馆用人速 3px/帧连续漂移；收尾 1.5 秒停巡，门禁读稳态损伤。
内容替换只走绘制（paint-only），不碰布局，保 p95。
