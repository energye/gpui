# Golden frames (P6)

Locked reference PNGs for capability matrix window scenarios C01–C32.

## Capture (regenerate)

```bash
export PATH=/home/yanghy/app/energy/go/bin:$PATH
export WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so LD_LIBRARY_PATH=$PWD/lib
export DISPLAY=:1 XAUTHORITY=/run/user/1000/gdm/Xauthority
go build -o /tmp/cap_l0_bin ./examples/capability_matrix

# deterministic, no HUD, frame 90
for id in C01 C02 ... C32; do
  GPUI_SCENARIO=$id GPUI_ANIM_SECONDS=3 GPUI_DETERMINISTIC=1 GPUI_GOLDEN_NO_HUD=1 \
    GPUI_CAPTURE_DIR=examples/capability_matrix/golden GPUI_CAPTURE_FRAME=90 \
    /tmp/cap_l0_bin
done
```

Or: `GPUI_P6_MODE=capture-golden scripts/run_capability_matrix_p6.sh`

## Compare

```bash
python3 scripts/cap_compare_golden.py \
  --got /tmp/cap_p6_run/capture \
  --golden examples/capability_matrix/golden \
  --diff-dir /tmp/cap_p6_run/diff \
  --report /tmp/cap_p6_run/golden_report.json
```

Gates (default): RMSE ≤ 0.08, SSIM ≥ 0.90 (downsampled luminance SSIM).

## Notes

- Frames are **repo-locked references** of the real X11 present path (not Skia offline export).
- Capture uses `GPUI_DETERMINISTIC=1` (`t = frame/60`) and `GPUI_GOLDEN_NO_HUD=1`.
- Re-lock golden only after intentional visual changes; then re-run compare in CI/local.
