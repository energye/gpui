# SVG conformance vs resvg v0.42 suite

- corpus: /tmp/resvg/crates/resvg/tests/tests (1685 cases)
- grading: perceptual 48x48 grid, identical ≤3 / close ≤24 / partial ≤128 / far

| group | total | identical | close | partial | far | blank | error | pass%(id+close) |
|---|---|---|---|---|---|---|---|---|
| filters | 397 | 178 | 150 | 57 | 12 | 0 | 0 | 82.6% |
| masking | 93 | 34 | 42 | 17 | 0 | 0 | 0 | 81.7% |
| paint-servers | 149 | 32 | 85 | 32 | 0 | 0 | 0 | 78.5% |
| painting | 305 | 174 | 103 | 28 | 0 | 0 | 0 | 90.8% |
| shapes | 131 | 113 | 6 | 6 | 0 | 0 | 6 | 90.8% |
| structure | 247 | 134 | 37 | 70 | 1 | 0 | 5 | 69.2% |
| text | 363 | 325 | 36 | 2 | 0 | 0 | 0 | 99.4% |
| **TOTAL** | 1685 | 990 | 459 | 212 | 13 | 0 | 11 | 86.0% |

## worst cases (far/blank/error, up to 8 per group)

### filters
default-values.svg (far)
empty-radius.svg (far)
negative-radius.svg (far)
no-radius.svg (far)
radius-with-too-many-values.svg (far)
zero-radius.svg (far)
hsla-color.svg (far)
inheritance-1.svg (far)

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

