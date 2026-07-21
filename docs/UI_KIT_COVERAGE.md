# UI Kit ↔ Ant Design 覆盖率（M6 快照）

> 源：`ui/kit/coverage.go` · 对照 `UI_FRAMEWORK_MAP` §5.7

运行：

```bash
go test ./ui/kit -run TestAntCoverageTable -v
```

| 状态 | 含义 |
|------|------|
| ready | kit API + Headless 测过 |
| partial | 有基线，高级 props 后置 |
| primitive | 用 primitive 可组，尚无 kit 产品名 |
| later | 未开工 |

主路径已 ready/partial 的含：Button、Input、Checkbox/Radio/Switch、Form、Select、Menu/Tabs、Modal/Drawer/Message、Table/List/Tree、Pagination/Dropdown、Transfer/Cascader、Progress/Skeleton/Spin/Tour、Tooltip/Popover 等。

长尾（DatePicker、Upload、Table 固定列/虚拟列完整、嵌套 Menu…）标 later。
