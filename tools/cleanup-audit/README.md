# cleanup-audit — 框架整理基线审计脚本

T0 产物，随 `docs/RENDER_CLEANUP_PLAN.md` 入库，重启不丢。

| 脚本 | 用法 | 输出 |
|---|---|---|
| `gpui_audit.py` | `python3 tools/cleanup-audit/gpui_audit.py`（仓库根） | 各域文件数、导出数、`ui` 直调 `gpu` 名单、`render` 后端初扫、临时标记计数 |
| `gpui_audit2.py` | `python3 tools/cleanup-audit/gpui_audit2.py`（约 2 分钟） | 后端精确命中、大文件榜、`render` 根 148 导出三态抽查 |

基线快照：2026-10-07，数字见 `docs/RENDER_CLEANUP_PLAN.md` §7。T2 起调用三态以 `go doc` 权威清单重出，本目录脚本为 T0 快照口径。
