# game_asset — 12.3 热重载独立真窗

一句话：改一个文件只换一个资源，坏文件保旧图不崩，看门人与直装逐位一致，断链报得出哪条链。

## 跑法

```sh
go run ./examples/engine/asset --case=reload -auto-only
go run ./examples/engine/asset --case=reload -manual-seconds 30
go run ./examples/engine/asset --case=reload
RUN_SECONDS=8 go run ./examples/engine/asset --case=reload -auto-only
```

- `-auto-only`：先跑三证据自检（不过直接 exit 1），再开窗约 8 秒（`RUN_SECONDS` 覆盖，至少 5 秒），标准输出打 `wrgate` 门禁 JSON，`present>=1` 且探针全过且子门全过且耗时进帽，否则 exit 1。
- `-manual-seconds N`：常驻 N 秒收真事件，每事件打日志并用 `SetTitle` 显示累计数，结束出汇总 JSON；不带 N 则常驻到关窗。人工把 `testdata/live_slot.ktx2` 换成另一张图的内容保存回窗，下一帧即换色；改坏了旧图不花，修好自动回来。
- 无旗：自检后常驻；设了 `RUN_SECONDS` 则按该秒数定时跑后出汇总。

## 画面（1200x800，标题 game_asset）

- 顶栏 + 图例 + 身体 + HUD 照 `wrkit.NewShell`。
- 身体左侧 LIVE 看门槽：红青两块叠放，按装载字节数切显（红 112B / 青 136B），只看 `testdata/live_slot.ktx2` 这一个自家文件，从不看仓库源码；哈希/字节/看门态为 live 回放证据。
- 身体中间 DEP 链：`map/level1` 需两张贴图、`tex/red` 被两处引用的文字链，断链时 JSON 报链名。
- 身体右侧计数器：重载数 / 事件数 / 轮询与重载目标 / 帧率。

## 三证据

- 逻辑探针：冻表直读 `engine/asset/testdata/hotreload_cases.json`（哈希字节全从文件来，窗里不写死数），一改只脏一个、邻居逐位不动、看门人与直装逐位一致且拷贝隔离、删文件与坏字节都保旧、依赖正反两向对链、百次来回字节在两档间恒定、2000 次轮询与单次 136B 重载计时。
- 像素断言：红槽青槽底色链条色，容差写死 `probePixelTol=4`（0-255 阶）。
- Golden 静态掩码零容差：`testdata/asset_reload_golden.png`，首跑产生基线，后续逐位比对。

## 门禁 JSON

`wrgate.BuildReport` / `EvaluateGates`，`AbilityID=asset-reload`，`Scenario=game_asset--case=reload`，`Extra` 带 `case` / `probe_ok` / `swap_ok` / `identical_ok` / `bad_kept_ok` / `deps_ok` / `stable_ok` / `reloads` / `events` / `poll_us` / `reload_us` / `poll_target_us` / `reload_target_us` / `from_hash` / `to_hash` / `from_size` / `to_size` / `dep_chain` / `dependents` / `live_*`。

## 阈值

- `present>=1`，探针全过，子门（swap/identical/bad_kept/deps/stable）全过；`RUN_SECONDS>=5`。
- 百次来回字节恒定（112B/136B 两档振荡，终态回基线，计数器 100/100）。
- 看门人与直装逐位一致，坏文件保旧，依赖断报链（`map/level1->tex/red,tex/checker`）。
- 耗时帽：`poll_us<=100`（S59 目标 14 量级，实测约 18）、`reload_us<=5000`（S59 目标 40 量级，实测约 42）。
