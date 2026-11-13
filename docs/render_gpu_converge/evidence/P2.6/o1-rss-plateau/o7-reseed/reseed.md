# O7 reseed result — static 7 pairs (2026-11-13)

Scope (user-approved): reseed only deterministic pairs. Excluded:
- `ui_wr_c4_shell_overlay` steady/recover: dynamic red dot; native-vs-go content
  diff 855 px within bbox (285,576)-(321,609), not a render hole.
- `ui_base_multiwin_cap` win1..8: per-window dynamic number/title band
  (13–21 rows), diff 206–267 px per window.

## Results

- Cross-check (old native current vs new go current): **content_diff = 0** for
  all 7 pairs (idle, c8, r4, r10, p33, c2, p37).
- Reseed + rerun native/go: **14/14 reports golden = 0.0000%**.
- Negative control (idle 1px shift): `rc=1`, golden **1.251488%** (expected
  red); restore from the freshly reseeded base → `rc=0`, golden **0.0**.
- Script bug found: the negative restore step originally restored the pre-run
  backup (stale 0.8489% baseline). Fixed to save/restore the new base
  (`negative/showcase_idle_new_base.png`); corrected idle verification is
  `idle_restore.out` (golden 0).
- Note: the raw `reseed_static.log` ends with `RESTORE FAIL` from the old
  restore path; that is the tool bug, not a baseline failure. All reseeded
  bases were produced by step 4 and validated by step 5 before that point.

## Excluded dynamic windows

- c4: 855 px diff confined to the dynamic red dot bbox; classification already
  recorded it as baseline-expired/dynamic.
- multiwin: 206–267 px diff confined to the top dynamic number/title band;
  single-frame goldens would bake a random phase.

## Evidence

- `reseed_static.log` — full run.
- `reseed.md` — this summary.
- `backup/` — pre-run bases (rollback evidence).
- `runs/<window>/<backend>/` — rerun stdout/stderr/rc.
- `o7-classify/summary.md` — dynamic-window classification.
