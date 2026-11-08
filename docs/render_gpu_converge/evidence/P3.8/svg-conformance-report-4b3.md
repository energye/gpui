# SVG conformance vs resvg v0.42 suite

- corpus: /tmp/resvg/crates/resvg/tests/tests (397 cases)
- grading: perceptual 48x48 grid, identical ≤3 / close ≤24 / partial ≤128 / far

| group | total | identical | close | partial | far | blank | error | pass%(id+close) |
|---|---|---|---|---|---|---|---|---|
| filters | 397 | 178 | 150 | 57 | 12 | 0 | 0 | 82.6% |
| **TOTAL** | 397 | 178 | 150 | 57 | 12 | 0 | 0 | 82.6% |

## worst cases (far/blank/error, up to 8 per group)

### filters
inheritance-1.svg (far)
inheritance-2.svg (far)
inheritance-3.svg (far)
inheritance-4.svg (far)
inheritance-5.svg (far)
default-values.svg (far)
empty-radius.svg (far)
negative-radius.svg (far)

