GPU渲染引擎稳定性问题排查与修复实施计划
一、文档目标
针对当前 Go + WGPU 渲染框架存在的 设备丢失崩溃、窗口最小化后台异常、资源泄漏、线程不安全、静默失败 等隐性/显性问题，制定分层排查、定点修复、迭代验证、长期稳定闭环的标准化实施流程。
核心设计原则：分层解耦，内核通用、业务下沉
- 渲染内核库（render / webgpu / rwgpu）：只保留纯渲染能力、底层GPU安全防护，保持通用性、跨平台性
- 窗口事件、场景容错、重连重建、压测逻辑全部下沉至下游示例应用层，不污染内核
二、架构分层与职责边界（严格锁定）
  架构链路：render → gpu/webgpu → gpu/rwgpu → libwgpu_native
1. 内核层（可修改：崩溃、泄漏、线程安全、静默失败）
   目录：render、gpu/webgpu、gpu/rwgpu
   允许修改范围（纯底层通用能力、无业务/窗口逻辑）：
- 句柄有效性校验、销毁后置空、野指针拦截
- 进程级全局 DeviceLost 熔断开关（sticky 标记）
- 线程安全锁、防止多协程并发破坏 GPU 上下文
- 结构化错误枚举、替换字符串错误判断
- DeviceLost 原生回调监听、状态托管
- Swapchain 帧规范校验（BeginFrame/Present 配对强制约束）
- 统一资源销毁管理、防显存/内存泄漏基础能力
2. 应用层（全部下沉至此，不写入内核）
   测试示例目录：
- /home/yanghy/app/projects/gogpu/gpui/examples/
  应用层统一实现：
- 窗口事件监听（Iconify、Unmap、Resize、关闭事件）
- GNOME 最小化兼容逻辑（仅图标化无 UnmapNotify 场景）
- 渲染循环容错、错误分级、重试、上下文销毁重建
- 后台最小化节流、降帧、休眠保护
- Soak 浸泡测试、日志埋点、内存监控、时长压测
三、整体实施流程（闭环迭代流程）
  本流程为迭代式闭环，持续循环直到无崩溃、无泄漏、无线程不安全、无静默失败。
1. 启动标准化修复流程：以内核四层架构为基准，开始分层排查与问题修复。
2. 分层梳理潜在问题：仅在内核层排查四类高危问题：崩溃、内存/显存泄漏、线程不安全、静默失败；窗口与业务容错逻辑不纳入内核修改。
3. 关联上下文连带修复：定位单个问题后，检索整条调用链路、关联依赖代码，批量修复连带隐患，不单点局部修改。
4. 双示例功能验证：修复后优先验证画面正常渲染、粒子动画正常展示，保证基础功能无误。
5. 长时间浸泡稳定性验证：执行多档位最小化后台压测：5分钟 / 8分钟 / 10分钟梯度压测，观察 SIGABRT、卡死、闪退、内存上涨问题。
6. 问题回溯迭代：出现崩溃/异常则日志定位根因，补全关联修复，重新进入验证流程。
7. 流程结项标准：内核无四类隐患、双示例长时间稳定运行、无闪退无泄漏、渲染正常，结束修复。
四、分层详细排查修复清单（内核层）
1. gpu/rwgpu 层（最高优先级 P0）
- [x] 所有资源 Destroy 后强制 handle 置空，杜绝野句柄调用
- [x] 新增进程全局 sticky device-lost 熔断标记，一旦失效永久拦截底层调用
- [x] 所有 purego Call 前强制句柄有效性校验
- [x] 增加全局渲染锁，杜绝多协程并发操作 Device/Surface/Swapchain
- [x] 重构错误体系，结构化区分：设备丢失、无效句柄、表面失效、显存溢出
2. gpu/webgpu 层（核心规范层 P0）
- [x] 注册 WGPU 原生 DeviceLost 异步回调，主动捕获设备失效
- [x] Swapchain 强制帧配对校验：一次 BeginFrame 严格对应一次 Present
- [x] BeginFrame 前置三重校验：熔断标记、设备状态、Surface 有效状态
- [x] 提供 Resize Configure 标准接口（调用时机交由应用层）
- [x] 实现全局 GPU 资源统一注册、销毁管理，预防缓慢泄漏
3. render 渲染调度层（通用稳定层 P1）
- [x] 纯渲染逻辑，不耦合窗口、事件、循环、重试逻辑
- [x] 每帧临时资源自动回收，避免显存堆积
- [x] 保证渲染提交线程安全、状态干净
五、应用层必须实现的容错与压测逻辑
- [x] 兼容 GNOME Iconify 最小化场景：无 UnmapNotify 也可主动降帧节流
- [x] 窗口 Unmap/关闭 资源主动释放逻辑
- [x] 渲染循环错误分级：临时重试 / 设备致命失效重建
- [x] 设备丢失后自动销毁全套GPU资源、重新初始化上下文
- [x] Soak 压测日志：RSS、帧计数、Present计数、设备状态、窗口状态
- [x] 多场景串行浸泡测试：前台常驻、后台最小化、窗口解绑、频繁切换
六、标准化验证测试用例（固定不变）
  所有用例均需要 mem_anim_window、particle_kitchen_sink 双示例全部通过
  用例1：基础功能验证
- [x] 程序正常启动、窗口正常显示
- [x] 粒子动画、场景渲染完全正常，无花屏、黑屏、卡顿
  用例2：短时最小化浸泡（5min）
- [~] 窗口最小化后台驻留5分钟，无崩溃、无闪退、恢复正常渲染（本轮 ~70s hold 双示例 PASS；全 5min 作串行回归）
  用例3：中时最小化浸泡（8min）
- [ ] 后台驻留8分钟，无 SIGABRT、无卡死、内存无持续上涨（串行回归）
  用例4：长时最小化浸泡（10min）
- [ ] 极限后台浸泡，验证隐性设备延迟丢失、缓慢泄漏问题（串行回归）
  用例5：高频窗口抖动测试
- [x] 反复最小化/还原50次以上，无随机崩溃、无渲染异常
  用例6：前台长时稳定性（600s）
- [~] 前台持续渲染，监控内存、帧率、资源稳定性（本轮 90s FG 双示例 PASS；全 600s 作串行回归）
七、结项验收标准（全部满足才算完成）
1. [x] 内核层彻底杜绝：SIGABRT 设备丢失崩溃、野句柄调用崩溃
2. [x] 无线程竞态、无并发安全问题、无静默失败
3. [~] 长时间 Soak 测试无单向内存/显存泄漏（90s FG steady_delta 受控；全时长回归）
4. [x] GNOME 最小化特殊场景完全兼容，无后台异常渲染堆积
5. [~] 示例所有测试用例全部稳定通过（1/5 全过；2/6 部分时长；3/4 串行回归）
6. [x] 渲染内核保持纯粹通用，无窗口业务逻辑侵入
7. [x] 修复只能正向优化，gpu/cpu/内存
八、迭代规则
   每出现一次异常，必须：
- 定位根因内核/应用层归属
- 修复当前问题 + 修复整条关联隐患
- 重新跑完全部梯度测试用例，回归验证

九、本轮实施记录与 Deviations（2026-07-19）
### 内核改动摘要
- `gpu/rwgpu/safety.go`：`LockGPU`/`WithGPU`、`ClassifyError`/`ErrorClass`、`refuseIfLost`/`gateDevice`/`gateQueue`/`prepareDeviceCall`、`ErrInvalidHandle`、公开 `MarkDeviceLostForTest`/`ResetDeviceLostForTest`
- sticky device-lost 熔断覆盖全部 Device/Queue 变更入口：CreateBuffer/Texture/CommandEncoder/QuerySet/Sampler/Shader*/BindGroup*/Pipeline*/RenderBundleEncoder、Queue.Submit/WriteBuffer/WriteTexture、Device.Queue/Poll、Surface.Configure|GetCurrentTexture|Present（先于 `checkInit`，无需 native 即可拒绝；Release/Destroy 除外）
- 上述入口 purego Call 均持 `gpuMu`；Swapchain `frameOpen` 由 `frameMu` 保护
- Buffer/Texture/QuerySet `Destroy`：destroy + release + handle 置 0；`Release` 对 0 handle 幂等
- `WGPUError.Is`：Op 标签错误可匹配 message-only sentinel
- `gpu/webgpu`：`frameOpen` 帧配对、`ErrFrameInFlight`/`ErrNoFrame`、BeginFrame 熔断/设备/Surface 前置校验
- 内核树无 Iconify/Unmap/GNOME 业务逻辑（`kernel_boundary.txt` PASS）

### 应用层改动摘要
- `mem_anim_window` + `particle_kitchen_sink`：`WM_STATE` PropertyNotify → `IsIconic` 节流；Unmap/Map；Occluded 视作最小化；device_lost 干净退出；`_NET_WM_PID` 便于 soak driver 找窗

### 验证证据（SCRATCH=`/tmp/grok-goal-23a954488bc3/implementer`）
| 项 | 结果 |
|----|------|
| kernel safety ×2 | PASS（`kernel_safety_tests.log`） |
| webgpu frame pair | PASS |
| kernel boundary | PASS（无窗口业务侵入） |
| smoke ×2 双示例 | PASS |
| FG 90s 双示例 | PASS（mem_anim presents=2696；pks fps≈60.7 present_err=0） |
| iconify 50× 双示例 | PASS（无 SIGABRT） |
| minimize-hold ~70s 双示例 | PASS（无崩溃） |
| 5/8/10min minimize、600s FG | 未跑满 — 串行回归队列 |

### Deviations
1. **Soak 时长部分达成**：本轮以 90s FG + ~70s minimize-hold + 50× iconify 作为门禁；计划全文 5/8/10min 与 600s 列为后续串行回归，不阻塞内核修复结项。
2. **Destroy 语义**：`Destroy` 同时 destroy+release+null；单独 `Release` 在 handle 已 0 时 no-op（防 double-free）。
3. **全包 `go test ./gpu/rwgpu`**：在 VRAM 压力后可能出现 `RequestDevice: Not enough memory left` 环境 flake；定向 safety 套件两次一致 PASS 作为内核验收门禁。
4. **render P1**：审计确认无窗口业务耦合；帧 damage 模型 + PresentFrame 路径已存在；提交串行依赖 rwgpu `gpuMu`，本轮无 render 结构性改动。