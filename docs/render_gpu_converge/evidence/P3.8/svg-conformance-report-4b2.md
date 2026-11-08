# SVG conformance vs resvg v0.42 suite

- corpus: /tmp/resvg/crates/resvg/tests/tests (1685 cases)
- grading: perceptual 48x48 grid, identical ≤3 / close ≤24 / partial ≤128 / far

| group | total | identical | close | partial | far | blank | error | pass%(id+close) |
|---|---|---|---|---|---|---|---|---|
| filters | 397 | 125 | 137 | 106 | 29 | 0 | 0 | 66.0% |
| masking | 93 | 34 | 42 | 17 | 0 | 0 | 0 | 81.7% |
| paint-servers | 149 | 32 | 85 | 32 | 0 | 0 | 0 | 78.5% |
| painting | 305 | 170 | 107 | 28 | 0 | 0 | 0 | 90.8% |
| shapes | 131 | 113 | 6 | 6 | 0 | 0 | 6 | 90.8% |
| structure | 247 | 134 | 37 | 70 | 1 | 0 | 5 | 69.2% |
| text | 363 | 234 | 127 | 2 | 0 | 0 | 0 | 99.4% |
| **TOTAL** | 1685 | 842 | 541 | 261 | 30 | 0 | 11 | 82.1% |

## worst cases (far/blank/error, up to 8 per group)

### filters
bias=9999.svg (far)
diffuseConstant=5.svg (far)
single-light-source-with-invalid-child.svg (far)
lighting-color=inherit.svg (far)
linearRGB-color-interpolation.svg (far)
multiple-light-sources.svg (far)
single-light-source-with-comment.svg (far)
single-light-source-with-desc.svg (far)

### shapes
ignore-odd-points.svg (error)
stop-processing-on-invalid-data.svg (error)
not-enough-points.svg (error)
stop-processing-on-invalid-data.svg (error)
ignore-odd-points.svg (error)
not-enough-points.svg (error)

### structure
no-width-and-height-on-svg.svg (far)
elements-via-ENTITY-reference-1.svg (error)
attribute-value-via-ENTITY-reference.svg (error)
elements-via-ENTITY-reference-2.svg (error)
elements-via-ENTITY-reference-3.svg (error)
not-UTF-8-encoding.svg (error)

