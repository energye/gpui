# FFI 迁移检查报告

本报告由 `cmd/migrate` 在将 `goffi` 导入重写为 `gpui/ffi` 后生成。

- 使用的符号数: 22
- 已覆盖的符号数: 22
- 缺失的符号数: 0

## 已覆盖符号

- `ffi.CallFunction`（759 处使用）
- `ffi.FreeLibrary`（1 处使用）
- `ffi.GetSymbol`（59 处使用）
- `ffi.LoadLibrary`（20 处使用）
- `ffi.NewCallback`（12 处使用）
- `ffi.PrepareCallInterface`（152 处使用）
- `ffi.PrepareVariadicCallInterface`（7 处使用）
- `types.CallInterface`（165 处使用）
- `types.DefaultCall`（158 处使用）
- `types.DoubleType`（2 处使用）
- `types.DoubleTypeDescriptor`（17 处使用）
- `types.FloatType`（2 处使用）
- `types.FloatTypeDescriptor`（7 处使用）
- `types.PointerTypeDescriptor`（186 处使用）
- `types.SInt32TypeDescriptor`（65 处使用）
- `types.StructType`（20 处使用）
- `types.TypeDescriptor`（204 处使用）
- `types.UInt32TypeDescriptor`（90 处使用）
- `types.UInt64TypeDescriptor`（39 处使用）
- `types.UInt8TypeDescriptor`（5 处使用）
- `types.UnixCallingConvention`（1 处使用）
- `types.VoidTypeDescriptor`（38 处使用）
