批 6 ⑥ · 原始跑数（2026-11-06，本机；日志 tmp 不入库，关键数字誊抄如下）
====================================================================================

探针迭代三轮（窗自身口径纠偏留痕）
--------------------------------
- 轮 1（首版）：探针 6 枚红 + 语义 1 枚红——
  `quad_payload_roundtrip`（float64 原值 vs encoding 的 float32 存储）、
  `replay_projection_equals_handwritten` diff=1622（手写臂把 SetProjection
  放到了蓝条之前）、`replay_stack_equals_handwritten` diff=7808（手写蓝条
  y=130 vs 场景 h−50=150）、`replay_material_equals_handwritten` diff=5120
  （漏了粘滞材质下 image 臂的 opts.Material——粘滞语义没照抄到手写腿）、
  `cpu_leg_stances` degrades=0（材质没清，image 被 material 门先拦——两态
  各开各的门）、`projected_panel_red` 落卡片底（Translate 下投影在设备域
  生效，P3.5 教训的正主：落点必须从画面同源反推，不能从公式外推）。
- 轮 2：15 枚探针全绿（仅剩缺基线判红，预期）。
- 一次性诊断（不入库件已删）：离屏程序 + 包内临时测试两件实测投影落点
  （bbox=(12,48)-(145,98) 无 translate / (18,72)-(136,111) 带 translate，
  回放与手写 SetProjection 两条独立路各自逐位一致）→ 语义锚改「与窗体同形
  的离屏全窗回放」红 mask bbox 中心。临时文件 `render/scene/zz_tmp_*.go`
  跑完即删、`git status` 核对无残留。
- 复核轮插曲：`live_only_skips==0` 门禁误判红一轮（239≈presents）——
  按 `ui_render_base_scene` 既有口径收回为只报数（scene 内容在呈现帧由
  语义锚扛，四锚全过）；README/主账/代码注释三处同步改写。

金图播种与复核（fail-closed 口径）
--------------------------------
- native 播种判红（rc=1「这一跑不算 PASS」）→ 复核 `OK presents=240
  probes=15/15 golden=0.0000% (921600 px) skips=239`。
- go 播种判红 → 复核 `OK backend=go presents=240 probes=15/15 golden=0.0000%`。
- 两基线 `cmp` 不同（字节 37 起）；逐像素工具：差异 171px 全在 y=10..18
  指标行（presents 尾数文案，金图比对区本就 mask 掉）；裁 y≥32 后像素差 =0。
- 链路计数两端全同：enc_bytes proj=205/stack=342/quad=130/material=175、
  enc_tags 17/26/2/19、a1_red_window_bbox=[18,72,137,112,4210]。

H15 定帧六轮（native，-auto-only）
--------------------------------
| 轮 | presents | probes | golden |
|---|---|---|---|
| 播种后复核 | 240 | 15/15 | 0.0000% |
| 复核轮（skips 口径修正后） | 240 | 15/15 | 0.0000% |
| 3 | 240 | 15/15 | 0.0000% |
| 4 | 241 | 15/15 | 0.0000% |
| 5 | 241 | 15/15 | 0.0000% |
| 6 | 240 | 15/15 | 0.0000% |

（另有轮 7 也恒 0；静态内容窗常驻相 -manual-seconds 6：`resident done
presents=358 probes_ok=true pixel_ok=true golden=0.0000% skips=357`——同 0 如实记。）

H16 两变异（窗体场景各删一行发射→跑→还原→终验复绿）
--------------------------------
| 变异 | 探针报警 | 像素报警 | golden diff |
|---|---|---|---|
| 1. 摘 sceneProj 的 SetProjection | 五枚红（counts/payload/回放≡手写 diff=14351/鉴别归零/Hash）；**首跑探针 2 got 串无守卫索引 panic 退 2，加安全访问后重跑取数** | 自洽跟随不报（锚从同形 bbox 反推，见主账四.3） | **2.0477%** |
| 2. 摘 sceneMaterial 的 SetMaterial | 四枚红（counts mat=0/材质回环空流/回放≡手写 diff=20000+鉴别归零/粘滞 shell） | `material_gradient_mid` (219,51,56)≠(172,86,37) | **3.3924%** |

还原核对：grep SetProjection(projM)×1 + SetMaterial(gradEffect)×2 原位；
gofmt/vet 净；终验 `OK backend=native presents=241 probes=15/15 golden=0.0000%`。
