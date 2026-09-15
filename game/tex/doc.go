// Package tex freezes the CPU-side compressed-texture intake every 2.5D
// draw feeds (capability 3.1, P2 long-chain head, S02/W1; streaming
// capability 3.3, P2, S29/W3).
//
// Frozen 2026-09-15: Format, FormatBC1RGBAUnorm, ParseKTX2, LoadKTX2,
// ParseBasis, LoadBasis, Image accessors. Additive changes only.
// Frozen 2026-09-15 (3.3): State, StateLoading/Ready/Missing/Failed,
// Stats, MaxStreams/MaxStreamBytes/MaxStreamAssetBytes/MaxStreamIDLen,
// PlaceholderImage, Stream{Request,RequestFile,Poll,Wait,Get,Placeholder}.
// Additive changes only.
//
// What is frozen: one container plus one block format. KTX2 (Khronos
// container v2, little-endian header) carrying VK_FORMAT_BC1_RGBA_UNORM_BLOCK
// (133), supercompression NONE, single 2D level, faceCount 1, layerCount 0,
// pixelDepth 0, width and height multiples of 4. The 8 block bytes upload
// to the GPU as-is; the CPU decodes the same blocks to RGBA8 for the
// offscreen comparison. One frozen kind first; a second kind freezes
// separately later.
//
// What is reserved: every other vkFormat, every non-zero
// supercompressionScheme (BasisLZ/Zstd/Zlib), multi-level (mipmaps go to
// 3.2), arrays/cubemaps/3D, uncompressed KTX2. They parse the header far
// enough to name the kind, then report core Unsupported, never a guessed
// image. The old PNG/JPG/WebP 8-bit path stays untouched.
//
// Errors use game/core codes: empty input is InvalidArg, a missing file is
// NotFound, a torn header or mismatched level bytes is BadData, a valid
// header naming an unfrozen kind is Unsupported, a picture beyond the
// MaxPixels budget is OutOfMemory. Only core numbers are used; decoded
// pixels cross to render as core.Color at the boundary.
//
// Streaming (3.3): Stream decodes KTX2+BC1 off the play path in the
// background (Request/RequestFile copy input, Poll never blocks, Wait caps
// at 5s). Unknown ids are StateEmpty plus NotFound; Loading duplicates are
// no-ops; missing files serve the shared 4x4 magenta placeholder
// (PlaceholderImage, never nil) with Missing/Failed replaying the stored
// cause. Budgets mirror game/asset (MaxStreams 1024, MaxStreamBytes 64MB,
// MaxStreamAssetBytes 8MB) but the ledger is standalone: game/asset reads
// tex, so tex cannot import asset (cycle); Stream reuses only core.Manager
// plus tex.ParseKTX2.
//
// Sampling switch (3.2 R3 bottom, S24/W3): Filter, Kind, MipMode, Table
// plus NumLevels/LevelForScale/LevelSize (mipmap.go). One picture carries
// one Filter: Near (magnify), Far (minify), Mip (between-level select),
// MaxAniso (1..16, 1 is off). SetFilter is the frozen near/far/aniso
// switch, SetMipmap the frozen level-blend switch; both validate loudly
// (InvalidArg) and leave the receiver untouched on error. SamplerParams
// is the single render-boundary mapping; a fully nearest picture masks
// mip/aniso exactly like the GPU nearest sampler. Level math mirrors the
// CPU box chain (render/internal/image/mipmap.go): NumLevels is
// 1+floor(log2(max)), LevelForScale is floor(-log2(scale)) clamped, bad
// scales fall back to 0. The render sampler bottom lives in
// render/internal/gpu/image_pipeline.go (ImageSamplerKey,
// SamplerDescriptorForImageFilter, SamplerForFilter, per-command
// SetPerImageFilter); the old Nearest/Bicubic path is untouched and the
// zero value keeps historic behaviour. Additive changes only.
package tex
