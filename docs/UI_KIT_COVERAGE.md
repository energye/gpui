# UI Kit ↔ Ant Design 覆盖率

> **权威源码**：[`ui/kit/coverage.go`](../ui/kit/coverage.go) · 对照 [`UI_FRAMEWORK_MAP.md`](./UI_FRAMEWORK_MAP.md) §5.7  
> **更新**：2026-07-23 · 与 `AntCoverage()` 表一致（**勿以本文件过期叙述为准，以 `go test` 为准**）  
> **底层支撑**：[`UI_FOUNDATION_P0.md`](./UI_FOUNDATION_P0.md) · **Kit 开发纪律**：[`UI_KIT_DEV_GUIDE.md`](./UI_KIT_DEV_GUIDE.md) · **Ant 对齐规格**：[`UI_KIT_ANT_V5_SPEC.md`](./UI_KIT_ANT_V5_SPEC.md)

运行：

```bash
go test ./ui/kit -run TestAntCoverageTable -v
```

| 状态 | 含义 |
|------|------|
| ready | kit API 存在且 Headless 测过（CoverageStatus=`CovReady`） |
| partial | 有基线，高级 props 后置 |
| primitive | 用 primitive 可组，尚无 kit 产品名 |
| later | 未开工 |

---

## 当前快照（源码表）

| 项 | 数值 |
|----|------|
| 表项总数 | **70** |
| `CovReady` | **70** |
| `CovPartial` / `CovPrimitive` / `CovLater` | **0** |

> 说明：状态列全为 ready **不表示**与 ant.design 网页 100% props 等价。残差写在表项 **Notes** 字段（见下）。

### Notes 残差（`coverage.go` 非空 Notes）

| Ant | Status | Notes |
|-----|--------|--------|
| Button | ready | ghost/color-variant/icon-end later |
| Flex | ready | wrap ✅ |
| Space | ready | wrap ✅ |
| Anchor | ready | ScrollTarget+SyncFromScroll spy |
| Menu | ready | flat; **nested later** |
| Tabs | ready | bar/body ScrollViewport |
| ColorPicker | ready | swatches |
| DatePicker | ready | SelectDay+Value; **range later** |
| Upload | ready | Picker/CapFile inject; **host dialog later** |
| Image | ready | Src+SetPixels sample; **GPU texture later** |
| QRCode | ready | deterministic modules; **codec later** |
| Table | ready | virtual rows; fixed header (Column, not in-scroll sticky) |

主路径 ready 含：Button、Input、Checkbox/Radio/Switch、Form、Select、Menu/Tabs、Modal/Drawer/Message、Table/List/Tree、Pagination/Dropdown、Transfer/Cascader、Progress/Skeleton/Spin/Tour、Tooltip/Popover 等（完整列表见 `AntCoverage()`）。

---

## 与文档历史叙述的差异

| 旧 markdown 说法 | 现源码 |
|------------------|--------|
| 长尾 DatePicker/Upload/Table… 标 **later** | 表内均为 **ready**；能力缺口在 **Notes** |
| 覆盖率仅「主路径 partial」 | 表 **70/70 ready** |

更新覆盖表时请只改 `ui/kit/coverage.go`，再同步本文件快照数字与 Notes。
