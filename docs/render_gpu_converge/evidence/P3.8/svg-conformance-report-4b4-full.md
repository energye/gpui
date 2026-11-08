# SVG conformance vs resvg v0.42 suite

- corpus: /tmp/resvg/crates/resvg/tests/tests (1685 cases)
- grading: perceptual 48x48 grid, identical ≤3 / close ≤24 / partial ≤128 / far

| group | total | identical | close | partial | far | blank | error | pass%(id+close) |
|---|---|---|---|---|---|---|---|---|
| filters | 397 | 184 | 160 | 53 | 0 | 0 | 0 | 86.6% |
| masking | 93 | 34 | 42 | 17 | 0 | 0 | 0 | 81.7% |
| paint-servers | 149 | 32 | 88 | 29 | 0 | 0 | 0 | 80.5% |
| painting | 305 | 175 | 104 | 26 | 0 | 0 | 0 | 91.5% |
| shapes | 131 | 119 | 6 | 6 | 0 | 0 | 0 | 95.4% |
| structure | 247 | 134 | 37 | 70 | 1 | 0 | 5 | 69.2% |
| text | 363 | 228 | 133 | 2 | 0 | 0 | 0 | 99.4% |
| **TOTAL** | 1685 | 906 | 570 | 203 | 1 | 0 | 5 | 87.6% |

## worst cases (far/blank/error, up to 8 per group)

### structure
no-width-and-height-on-svg.svg (far)
attribute-value-via-ENTITY-reference.svg (error)
elements-via-ENTITY-reference-1.svg (error)
elements-via-ENTITY-reference-2.svg (error)
elements-via-ENTITY-reference-3.svg (error)
not-UTF-8-encoding.svg (error)

