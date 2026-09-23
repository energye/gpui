//go:build !(amd64 || arm64) || purego

package h264

// itrans8x8Arch stays scalar off amd64 (scalar留守 runs). arm64 has
// its own file; this covers 386/32-bit arm/wasm and purego builds.
func itrans8x8Arch(c, t1, t2, out *[64]int32) {
	*out = itrans8x8Core(*c)
}

// itrans4x4Arch stays scalar off amd64 (same coverage as above;
// output is identical via itrans4x4CoreScalar, only speed differs).
func itrans4x4Arch(c, out *[16]int32) {
	*out = itrans4x4CoreScalar(*c)
}
