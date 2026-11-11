# B3：ui_text_edit_accept script 金图逐位比较（2026-11-11）

> 台账 §2.2 B3：script 金图自动模式不逐位比较（旧流程把像素交给窗外 xwd 脚本，
> 仓内 golden/f_script/{s1,s2,s3}.png 没有 in-repo 比对）。

## 0 结论

script 模式现在自带像素门禁：每个 SCRIPT_MARK 在状态上屏后，用 RetainedSnapshot
回放保留包（进程内，无 xwd），按 mark 矩形裁片与 golden/f_script/<tag>.png
零容差逐位比较；s1/s2/s3 三片必须 0 差分。两次跑 3/3 差分 0；1px 篡改负向红、还原绿。

## 1 实现（examples/ui_text_edit_accept/script_verify.go）

- mark()：记录 tag 的 mark 矩形与当次 present 计数。
- tick()：present 计数前进后启动 capture（goroutine 内调 RetainedSnapshot；该调用会
  spin，放 UI 线程会卡住承载回放的那一帧）。快照落地后由 UI 线程 drain 并按 mark 矩形
  裁片比较；capture 在飞期间步进机不推进，保证回放的是标记态而不是下一态。
- compareScriptCrop：RGB 逐通道零容差，diff>0 即 failf；缺基线 fail-closed。
- 快照目录默认 /tmp/accept_script_snap（每跑清空），可用 GPUI_ACCEPT_SCRIPT_SNAP_DIR 覆盖。
- 运行时长：finish 在 31s，脚本门禁命令改为 GPUI_ACCEPT_RUN_SECONDS=34（README 同步）。

## 2 金图重种（真源对齐）

- 旧 s1/s2/s3 是 xwd 实拍裁片；in-process 快照是引擎呈现树的重放，与 xwd 有约
  4-10% 像素差（文字 AA/合成路径差），内容目检一致、逻辑断言全过。
- 按 B2 同一口径，把三片基线重种为**引擎快照裁片**（880×90 / 880×40），旧片归档
  evidence/P2.6/b3-golden-reseed/；ref.json 保留为旧外部检查器的计数参考（不再参与门禁）。
- 重种前先验证快照可复现：两次独立跑的同 tag 裁片逐位 0 差。

## 3 验证（证据 evidence/P2.6/b3-windows/）

| 跑 | 结果 |
|---|---|
| 重种后 run1 | SCRIPT_VERIFY pass=true；pixel 3/3；s1/s2/s3 diff=0（79200/79200/35200 px） |
| run2 | pass=true；pixel 3/3 diff=0 |
| 负向（s1 基线 1px 改洋红） | script pixel s1: 1/79200 px differ (bitwise)，rc=1 |
| 还原后 | 基线与备份 sha256 一致；复跑 pass=true 3/3 |

## 4 备注

- 逻辑门禁口径未变（s1 滚底 ±1px、s2 全选/3 点击中、s3 B 横滚 >0）。
- 外部 xwd 跟拍/握手流程保留为可选旁证，自动门禁不再依赖 /tmp 脚本。
- 本项未改引擎/渲染行为，仅窗口门禁与基线。
