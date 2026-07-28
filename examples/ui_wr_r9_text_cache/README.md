# ui_wr_r9_text_cache — R9 text measure cache

**Ability:** R9 (W1+)  
**Window:** **1200×800**  
**Close duration:** **`RUN_SECONDS=5`**

## Run

```bash
export LD_LIBRARY_PATH=$PWD/lib WGPU_NATIVE_PATH=$PWD/lib/libwgpu_native.so
RUN_SECONDS=5 go run ./examples/ui_wr_r9_text_cache
```

## Visible effect

A text panel with wrapped multi-line copy. Content is **stable**; each tick forces layout so `measureLine` hits the cache. Width/height must not jitter.

## Gates

- `measure_cache_hit ≥ 1` and **hits ≥ misses** after warm
- §2.2 schema + presents + fps
