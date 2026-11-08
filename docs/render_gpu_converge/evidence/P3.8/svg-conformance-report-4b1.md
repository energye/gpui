# SVG conformance vs resvg v0.42 suite

- corpus: /tmp/resvg/crates/resvg/tests/tests (397 cases)
- grading: perceptual 48x48 grid, identical ≤3 / close ≤24 / partial ≤128 / far

| group | total | identical | close | partial | far | blank | error | pass%(id+close) |
|---|---|---|---|---|---|---|---|---|
| filters | 397 | 119 | 141 | 106 | 31 | 0 | 0 | 65.5% |
| **TOTAL** | 397 | 119 | 141 | 106 | 31 | 0 | 0 | 65.5% |

## worst cases (far/blank/error, up to 8 per group)

### filters
operator=arithmetic-and-invalid-k1-4.svg (far)
operator=arithmetic-with-some-k1-4.svg (far)
bias=9999.svg (far)
diffuseConstant=5.svg (far)
lighting-color=inherit.svg (far)
single-light-source-with-title.svg (far)
linearRGB-color-interpolation.svg (far)
multiple-light-sources.svg (far)

