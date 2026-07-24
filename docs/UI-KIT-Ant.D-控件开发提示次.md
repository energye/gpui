可直接复制。把 <控件名> / <slug> 换成实际值（如 Switch / switch）。

  ---
  附带文档（所有提示词共用）

  最少必带：
  需求：docs/antd/<slug>.md §6
  全库 DoD/gallery：docs/antd/README.md

  新做 / 推翻重写再加：
  样板：docs/antd/button.md §6 + ui/kit/button.go + button_prd_test.go
  工程：docs/UI_KIT_DEV_GUIDE.md
  对齐：docs/UI_KIT_ANT_V5_SPEC.md（L1–L4）
  antd 只读：/home/yanghy/app/projects/ant-design/components/<slug>/

  ---
  1. 新开发控件

  按 docs/antd/<slug>.md 的 §6 实现 ui/kit 控件 <控件名>。

  【附带文档】
  - 需求：docs/antd/<slug>.md §6（范围 §6.8 P0）
  - 全库：docs/antd/README.md（ui_polish_gallery / 完成定义）
  - 样板：docs/antd/button.md §6 + ui/kit/button.go + button_prd_test.go
  - 工程：docs/UI_KIT_DEV_GUIDE.md
  - 对齐：docs/UI_KIT_ANT_V5_SPEC.md
  - antd 只读：/home/yanghy/app/projects/ant-design/components/<slug>/

  【硬性要求】
  1. 以 §6 为唯一 DoD；§1–§3 仅参考；只做 §6.8 P0。
  2. API 对齐 §6.10；默认值对齐 §6.10 表。
  3. 行为对齐 §6.4；度量对齐 §6.2。
  4. 测试：Test<Name>_PRD_* 覆盖 §6.9 全部 L1/L2 P0。
  5. gallery：examples/ui_polish_gallery 对应页覆盖 §6.8 P0。
  6. coverage.go Notes：P0 已对齐 docs/antd/<slug>.md §6；P1 列出。
  7. 组合 ui/primitive；hit==layout==paint；Theme Token；无第二套帧循环。
  8. 跑 go test ./ui/kit/ -run 'Test<Name>' 到绿并报告。

  最短：
  按 docs/antd/<slug>.md §6 做齐 P0：实现 + PRD 测试 + ui_polish_gallery + coverage Notes。对照 button 样板。测到绿。

  ---
  2. 修改已有控件

  修改 ui/kit 的 <控件名>，符合 docs/antd/<slug>.md §6。

  【附带文档】
  - docs/antd/<slug>.md §6.2 / §6.4 / §6.8 / §6.9 / §6.10（与本次相关）
  - docs/antd/README.md（若动 gallery）

  【范围】
  - 要做：§6.8 中的 [列出 P0 项]
  - 不做：未点名的 P1

  【步骤】
  1. 改实现；breaking 须说明影响。
  2. 补/改 Test<Name>_PRD_*。
  3. 更新 examples/ui_polish_gallery 相关 section。
  4. 必要时更新 coverage.go Notes。
  5. 跑 go test ./ui/kit/ -run 'Test<Name>' 并报告。

  禁止：扩大未要求的 P1；为过测试放宽 §6。

  ---
  3. 修 bug

  Bug：<现象>
  控件：ui/kit/<控件> + docs/antd/<slug>.md

  【附带文档】
  - docs/antd/<slug>.md §6.4 / §6.9（对应规则 ID / 用例 ID）
  - 涉及度量时加 §6.2

  【要求】
  1. 定位规则 ID（如 B-S2、BTN-04）。
  2. 修到与规则一致。
  3. 加/强回归测试（优先 PRD 用例）。
  4. 跑相关测试；说明根因与修复点（文件:行号）。

  ---
  4. 推翻重写（整控件）

  推翻重写 ui/kit 的 <控件名>。breaking OK。

  【附带文档】
  - 需求：docs/antd/<slug>.md §6（P0=§6.8；API=§6.10；行为=§6.4；用例=§6.9；DoD=§6.12）
  - 全库：docs/antd/README.md
  - 样板：docs/antd/button.md §6 + button.go + button_prd_test.go
  - 工程：docs/UI_KIT_DEV_GUIDE.md
  - 对齐：docs/UI_KIT_ANT_V5_SPEC.md
  - antd 只读：/home/yanghy/app/projects/ant-design/components/<slug>/

  【实现】
  - 只做 §6.8 P0；P1 进 coverage Notes。
  - 以 §6.10 重建公开 API；可删旧实现。
  - primitive 组合；Token；hit==layout==paint；Ticker 做 loading。

  【连带交付】
  1. §6 有误则先改 md 再写码，并说明改了哪几节。
  2. Test<Name>_PRD_* 覆盖 §6.9 全部 L1/L2 P0。
  3. examples/ui_polish_gallery 按 §6.8 P0 重铺该页。
  4. coverage.go Notes 更新。
  5. 列出 breaking API 与已修引用。
  6. go test ./ui/kit/ -run 'Test<Name>' 到绿。

  不要：迁就旧 API 牺牲 §6；不要做 P1；不要浏览器逐像素。

  最短：
  按 docs/antd/<slug>.md §6 推翻重写 ui/kit.<控件>：只做 P0，breaking OK。
  实现 + PRD 测试 + ui_polish_gallery + coverage Notes。对照 button。测到绿。

  ---
  5. 半推翻（壳可留、内核重做）

  半推翻 ui/kit/<控件名>：
  - 公开 API 以 docs/antd/<slug>.md §6.10 为准，可 breaking。
  - 内部 chrome/状态机按 §6.4 / §6.5 重做。
  - 能复用的 primitive 组合保留。

  【附带文档】同「推翻重写」最少集（该控件 §6 + README + button 样板）。

  补齐：PRD 测试 + gallery P0 + coverage Notes。测到绿。

  ---
  6. 只补测试 / 只补 gallery

  测试：
  按 docs/antd/<slug>.md §6.9 为 ui/kit/<控件> 补全 P0 自动测试 Test<Name>_PRD_*。
  断言要可测（次数、高度±0.5、状态）。参考 button_prd_test.go。跑测试并报告。

  gallery：
  在 examples/ui_polish_gallery 为 <控件> 增加/更新示例，覆盖 docs/antd/<slug>.md §6.8 P0。
  细则见 docs/antd/README.md「ui_polish_gallery」。一控件一页，section 分块。不做 P1。

  ---
  7. 交付对照（写进提示词时可缩成一句）

  ┌──────────┬─────────────────────────────────────┐
  │   产物   │              文档锚点               │
  ├──────────┼─────────────────────────────────────┤
  │ 实现     │ §6.8 P0 + §6.10                     │
  ├──────────┼─────────────────────────────────────┤
  │ 测试     │ §6.4 + §6.9                         │
  ├──────────┼─────────────────────────────────────┤
  │ gallery  │ §6.12 + README「ui_polish_gallery」 │
  ├──────────┼─────────────────────────────────────┤
  │ 完成登记 │ coverage.go Notes                   │
  ├──────────┼─────────────────────────────────────┤
  │ 勾选完成 │ §6.12 全条                          │
  └──────────┴─────────────────────────────────────┘

  交付：代码(P0) + Test*_PRD_* + ui_polish_gallery + coverage Notes；验收=§6.12。

  ---
  8. 选用速查

  ┌─────────────────────────┬────────────┐
  │          场景           │   用哪套   │
  ├─────────────────────────┼────────────┤
  │ 从 0 做控件             │ 1 新开发   │
  ├─────────────────────────┼────────────┤
  │ 改几个能力              │ 2 修改     │
  ├─────────────────────────┼────────────┤
  │ 行为不对                │ 3 修 bug   │
  ├─────────────────────────┼────────────┤
  │ 整控件推倒              │ 4 推翻重写 │
  ├─────────────────────────┼────────────┤
  │ 大部能留、皮/状态机重做 │ 5 半推翻   │
  ├─────────────────────────┼────────────┤
  │ 只欠测试或示例          │ 6          │
  └─────────────────────────┴────────────┘

