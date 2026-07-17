# oraclejs — CanvasKit 标准图生成

输入：`../scenes/*.json`  
输出：`../refs/standard/*.png` + 更新 `../refs/standard.json`

```bash
npm ci
node gen.mjs --from-scenes
node gen.mjs --check
```

禁止 import gpui。与 `stdgate/scene` 使用同一套 op 名。
