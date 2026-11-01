# 批 3 逐文件唯一名单变化 = render/s6_9_heavy_budget_test.go（批 2 轮 PASS，本批轮 FAIL）

A/B 定性（2026-11-02）：
- 工作树连跑多轮：红名在 H03/H04/U05/H02 间漂移，1~3 条、贴线超出 2%~6%（如 U05 7.30 vs 上界 7.10）。
- 干净 HEAD（aae2d377，批 2 提交树，worktree+纯 same-lib 环境）同测同样 FAIL：红名 U05（7.30/7.10），H03/H04 反而 PASS——两树红名互不一致。
- 批 3 改动面（buildImageResources 的 persp 预扫+perspIdx 分配、triSampler 加两字段+一 bool 分支、新管线变体）不在 stroke/layer/text 帧时路径上；该文件是 P2 已登记的墙钟 p50 预算门，批 1 起就在基线红名单内（batch1.md §6 FAIL 名单），批 2 轮未复现、本批轮部分复现=环境/负载敏感老红。
结论：非批 3 回归，按「基线红名单逐名在案、名单只减不增（m1_noquadratic/m5_keystroke_g1 本轮转绿）」口径登记。原始复跑记录 s69-worktree-reruns.txt。
