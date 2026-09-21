# F0 中央门禁四查 + API 总账覆盖检查（F0-5 钉死）

地位：F1 开工的前置条件。无命令 F1 不开。

## 中央门禁四查（每窗必过）

实现：`ui/kit/internal/gate/{props,state,wiring,token}.go`，公开门面 `ui/kit/gate.go`。
单测：`ui/kit/internal/gate/gate_test.go`（四查全绿才算过）。

| 查 | 断什么 | 红线 |
| --- | --- | --- |
| Props | 文档参数全覆盖（Covered>=Total 且 Missing 为空） | 少一个参数即 FAIL |
| State | 每个声称的状态都走 StateResolver，且含 Disabled 位 | 有状态没 Resolve 即 FAIL |
| 接线 | 产品禁直调 render/gpu，只许调 prim/behavior/scope+theme | 直调即 FAIL |
| Token | 颜色全走主题，Render 层零用户值硬编码 | 硬编码即 FAIL |

## API 总账覆盖检查命令（合入前必跑）

```bash
# 1. 单测逐文件（禁止一次全量），F0-5 新增：
go test ./ui/kit/internal/behavior/ -run 'TestMotion_' -count=1
go test ./ui/kit/internal/behavior/ -run 'TestSemantics_' -count=1
go test ./ui/kit/internal/behavior/ -run 'TestPerformance_' -count=1
go test ./ui/kit/internal/behavior/ -run 'TestDirection_' -count=1
go test ./ui/kit/internal/gate/ -run 'TestGate_' -count=1

# 2. 产品禁直调 render/gpu（编译期卡，F0-5 起每窗必查）：
go list -f '{{ join .Imports "\n" }}' ./ui/kit/ | grep -E '/(render|gpu)(/|$)' && echo FAIL || echo wiring-ok

# 3. 引擎层用户值硬编码回归（合入前必跑，白名单仅主题种子默认值）：
grep -rn '1677ff\|No data\|请输入' ui/ --include='*.go' | grep -v '_test.go' | grep -v 'ui/theme/' | grep -v 'DefaultRenderEmpty\|DefaultHolderRender' || echo hardcode-ok

# 4. 构建：
CGO_ENABLED=0 go build ./ui/kit/...
```

任一条红即打回，不许标绿。
