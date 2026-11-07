# SVG conformance vs resvg v0.42 suite

- corpus: /tmp/resvg/crates/resvg/tests/tests (1685 cases)
- grading: perceptual 48x48 grid, identical ≤3 / close ≤24 / partial ≤128 / far

| group | total | identical | close | partial | far | blank | error | pass%(id+close) |
|---|---|---|---|---|---|---|---|---|
| filters | 397 | 40 | 110 | 226 | 21 | 0 | 0 | 37.8% |
| masking | 93 | 34 | 43 | 16 | 0 | 0 | 0 | 82.8% |
| paint-servers | 149 | 28 | 63 | 58 | 0 | 0 | 0 | 61.1% |
| painting | 305 | 175 | 102 | 28 | 0 | 0 | 0 | 90.8% |
| shapes | 131 | 113 | 6 | 6 | 0 | 0 | 6 | 90.8% |
| structure | 247 | 134 | 37 | 70 | 1 | 0 | 5 | 69.2% |
| text | 363 | 325 | 36 | 2 | 0 | 0 | 0 | 99.4% |
| **TOTAL** | 1685 | 849 | 397 | 406 | 22 | 0 | 11 | 73.9% |

## worst cases (far/blank/error, up to 8 per group)

### filters
operator=arithmetic-and-invalid-k1-4.svg (far)
operator=arithmetic-with-some-k1-4.svg (far)
operator=arithmetic.svg (far)
bias=9999.svg (far)
diffuseConstant=5.svg (far)
lighting-color=inherit.svg (far)
linearRGB-color-interpolation.svg (far)
multiple-light-sources.svg (far)

### shapes
ignore-odd-points.svg (error)
not-enough-points.svg (error)
stop-processing-on-invalid-data.svg (error)
ignore-odd-points.svg (error)
not-enough-points.svg (error)
stop-processing-on-invalid-data.svg (error)

### structure
no-width-and-height-on-svg.svg (far)
attribute-value-via-ENTITY-reference.svg (error)
elements-via-ENTITY-reference-1.svg (error)
elements-via-ENTITY-reference-2.svg (error)
elements-via-ENTITY-reference-3.svg (error)
not-UTF-8-encoding.svg (error)

