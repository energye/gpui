// hbshape — 原生 HarfBuzz（libharfbuzz.so）shaping 对照度量衡。
//
// S1 用途：验证生产 shaping 路径（HbShaper = go-text/typesetting/harfbuzz
// 移植）在复杂脚本上与原生 libharfbuzz 输出一致（skeleton：
// gid/cluster/xoffset/yoffset 必须完全一致；advance 记录不判，沿用 M2 判据）。
//
// 只存源码，不提交二进制；测试经 hbshapeLocal(t) 解析（$HBSHAPE_BIN →
// 已构建产物 → 拷贝源码到 TempDir go build 重建，需 go + purego）。
//
// 用法: hbshape <ttf|ttc> <text> [script] [lang] [direction] [sizePx] [faceIdx]
//   script    = 三字母 OpenType 标签（默认 auto 按首字符探测）
//   lang      = BCP47（默认 en）
//   direction = ltr|rtl|ttb|btt（默认 ltr）
//   sizePx    = 字号（默认 16，仅影响 advance 不影响 gid/cluster/offset）
//   faceIdx   = TTC face 序号（默认 0）
// 输出: 每 glyph 一行 "gid cluster xoff yoff xadv yadv"（26.6 定点，
//   harfbuzz 位置单位 = font units，未设 scale 时）。
//
// 注意: 不设 hb_font_set_scale（保持 font units），所以 xadv/yadv 是
//   font units 整数；gid/cluster/offset 与 scale 无关，是对照骨架的
//   核心。go-text 侧对照时用相同 upem 缩放即可。
package main

import (
	"fmt"
	"os"
	"unicode/utf8"
	"unsafe"

	"github.com/ebitengine/purego"
)

var (
	libhb uintptr

	hbVersionString   func() *byte
	hbBlobCreate      func(data unsafe.Pointer, len uint, mode int, userData uintptr, destroy uintptr) uintptr
	hbBlobDestroy     func(blob uintptr)
	hbFaceCreate      func(blob uintptr, index uint) uintptr
	hbFaceDestroy     func(face uintptr)
	hbFaceGetUpem     func(face uintptr) uint
	hbFontCreate      func(face uintptr) uintptr
	hbFontDestroy     func(font uintptr)
	hbOTFontSetFuncs  func(font uintptr)
	hbBufferCreate    func() uintptr
	hbBufferDestroy   func(buf uintptr)
	hbBufferAddUtf8   func(buf uintptr, text *byte, textLen int, itemOffset uint, itemLen int)
	hbBufferSetDir    func(buf uintptr, dir int)
	hbBufferSetScript func(buf uintptr, script uint)
	hbBufferSetLang   func(buf uintptr, lang uintptr)
	hbBufferSetClust  func(buf uintptr, level int)
	hbShape           func(font uintptr, buf uintptr, features *uintptr, numFeatures uint)
	hbBufferGetInfos  func(buf uintptr) uintptr
	hbBufferGetPosns  func(buf uintptr) uintptr
	hbBufferGetLength func(buf uintptr) uint
	hbLangFromString  func(s *byte, len int) uintptr
	hbTagFromString   func(s *byte) uint
	hbDirectionToStr  func(dir int) *byte
)

const (
	hbDirLTR = 4
	hbDirRTL = 5
	hbDirTTB = 6
	hbDirBTT = 7
)

type hbGlyphInfo struct {
	Codepoint   uint32
	Mask        uint32
	Cluster     uint32
	Var1        uint32
	Var2        uint32
}

type hbGlyphPosition struct {
	XAdvance   int32
	YAdvance   int32
	XOffset    int32
	YOffset    int32
	Var        int32
}

func mustLoad() {
	// $HBSHAPE_HB_LIB 可指定具体库路径（如对照新版 libharfbuzz 8.3.0），
	// 缺省用系统 libharfbuzz.so.0（2.7.4）。
	if p := os.Getenv("HBSHAPE_HB_LIB"); p != "" {
		libhb, _ = purego.Dlopen(p, purego.RTLD_NOW|purego.RTLD_GLOBAL)
		if libhb == 0 {
			fmt.Fprintf(os.Stderr, "FAIL dlopen %s\n", p)
			os.Exit(1)
		}
		registerFuncs()
		return
	}
	libhb, _ = purego.Dlopen("libharfbuzz.so.0", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	if libhb == 0 {
		// fallback: plain name (some systems)
		libhb, _ = purego.Dlopen("libharfbuzz.so", purego.RTLD_NOW|purego.RTLD_GLOBAL)
	}
	if libhb == 0 {
		fmt.Fprintln(os.Stderr, "FAIL dlopen libharfbuzz.so.0")
		os.Exit(1)
	}
	registerFuncs()
}

func registerFuncs() {
	purego.RegisterLibFunc(&hbVersionString, libhb, "hb_version_string")
	purego.RegisterLibFunc(&hbBlobCreate, libhb, "hb_blob_create")
	purego.RegisterLibFunc(&hbBlobDestroy, libhb, "hb_blob_destroy")
	purego.RegisterLibFunc(&hbFaceCreate, libhb, "hb_face_create")
	purego.RegisterLibFunc(&hbFaceDestroy, libhb, "hb_face_destroy")
	purego.RegisterLibFunc(&hbFaceGetUpem, libhb, "hb_face_get_upem")
	purego.RegisterLibFunc(&hbFontCreate, libhb, "hb_font_create")
	purego.RegisterLibFunc(&hbFontDestroy, libhb, "hb_font_destroy")
	purego.RegisterLibFunc(&hbOTFontSetFuncs, libhb, "hb_ot_font_set_funcs")
	purego.RegisterLibFunc(&hbBufferCreate, libhb, "hb_buffer_create")
	purego.RegisterLibFunc(&hbBufferDestroy, libhb, "hb_buffer_destroy")
	purego.RegisterLibFunc(&hbBufferAddUtf8, libhb, "hb_buffer_add_utf8")
	purego.RegisterLibFunc(&hbBufferSetDir, libhb, "hb_buffer_set_direction")
	purego.RegisterLibFunc(&hbBufferSetScript, libhb, "hb_buffer_set_script")
	purego.RegisterLibFunc(&hbBufferSetLang, libhb, "hb_buffer_set_language")
	purego.RegisterLibFunc(&hbBufferSetClust, libhb, "hb_buffer_set_cluster_level")
	purego.RegisterLibFunc(&hbShape, libhb, "hb_shape")
	purego.RegisterLibFunc(&hbBufferGetInfos, libhb, "hb_buffer_get_glyph_infos")
	purego.RegisterLibFunc(&hbBufferGetPosns, libhb, "hb_buffer_get_glyph_positions")
	purego.RegisterLibFunc(&hbBufferGetLength, libhb, "hb_buffer_get_length")
	purego.RegisterLibFunc(&hbLangFromString, libhb, "hb_language_from_string")
	purego.RegisterLibFunc(&hbTagFromString, libhb, "hb_tag_from_string")
	purego.RegisterLibFunc(&hbDirectionToStr, libhb, "hb_direction_to_string")
}

// cStr returns a NUL-terminated pointer to s (kept alive by caller).
func cStr(s string) *byte {
	b := append([]byte(s), 0)
	return &b[0]
}

// goStr converts a NUL-terminated C string to Go string.
func goStr(p *byte) string {
	if p == nil {
		return ""
	}
	end := p
	for *end != 0 {
		end = (*byte)(unsafe.Add(unsafe.Pointer(end), 1))
	}
	return string(unsafe.Slice(p, uintptr(unsafe.Pointer(end))-uintptr(unsafe.Pointer(p))))
}

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "usage: hbshape <ttf|ttc> <text> [script] [lang] [direction] [sizePx] [faceIdx]")
		os.Exit(2)
	}
	mustLoad()
	ver := hbVersionString()
	fmt.Fprintf(os.Stderr, "libharfbuzz %s\n", goStr(ver))

	path := os.Args[1]
	text := os.Args[2]
	script := "auto"
	lang := "en"
	direction := hbDirLTR
	if len(os.Args) > 3 {
		script = os.Args[3]
	}
	if len(os.Args) > 4 {
		lang = os.Args[4]
	}
	if len(os.Args) > 5 {
		switch os.Args[5] {
		case "rtl":
			direction = hbDirRTL
		case "ttb":
			direction = hbDirTTB
		case "btt":
			direction = hbDirBTT
		}
	}
	faceIdx := uint(0)
	if len(os.Args) > 7 {
		fmt.Sscanf(os.Args[7], "%d", &faceIdx)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "FAIL read:", err)
		os.Exit(1)
	}

	// hb_blob_create with MEMORY_MODE_READONLY, no destroy callback (we
	// hb_blob_destroy explicitly at the end; the Go byte slice stays alive
	// until then thanks to `_ = data`).
	blob := hbBlobCreate(unsafe.Pointer(&data[0]), uint(len(data)), 1 /* MEMORY_MODE_READONLY */, 0, 0)
	if blob == 0 {
		fmt.Fprintln(os.Stderr, "FAIL blob")
		os.Exit(1)
	}
	face := hbFaceCreate(blob, faceIdx)
	if face == 0 {
		fmt.Fprintln(os.Stderr, "FAIL face")
		os.Exit(1)
	}
	upem := hbFaceGetUpem(face)
	font := hbFontCreate(face)
	hbOTFontSetFuncs(font)

	buf := hbBufferCreate()
	// 对齐 go-text HbShaper：MonotoneGraphemes cluster level（=0），
	// 保证 cluster 归并语义与生产路径一致。
	hbBufferSetClust(buf, 0)
	hbBufferSetDir(buf, direction)
	if script != "auto" && script != "" {
		hbBufferSetScript(buf, hbTagFromString(cStr(script)))
	}
	if lang != "" {
		hbBufferSetLang(buf, hbLangFromString(cStr(lang), -1))
	}
	hbBufferAddUtf8(buf, cStr(text), -1, 0, -1)
	hbShape(font, buf, nil, 0)

	n := int(hbBufferGetLength(buf))
	infos := (*[1 << 24]hbGlyphInfo)(unsafe.Pointer(hbBufferGetInfos(buf)))[:n]
	posns := (*[1 << 24]hbGlyphPosition)(unsafe.Pointer(hbBufferGetPosns(buf)))[:n]
	// HarfBuzz cluster = UTF-8 字节偏移；go-text HbShaper 用 rune 索引。
	// 为对齐对照，输出 rune 索引 cluster（源文本逐 rune 的字节偏移表）。
	runeOffsets := make([]int, 0, len([]rune(text))+1)
	for _, r := range []rune(text) {
		runeOffsets = append(runeOffsets, utf8.RuneLen(r))
	}
	byteToRune := make([]int, 0, len(text)+1)
	byteToRune = append(byteToRune, 0)
	for _, l := range runeOffsets {
		byteToRune = append(byteToRune, byteToRune[len(byteToRune)-1]+l)
	}
	clusterOfByte := func(b int) int {
		if b <= 0 {
			return 0
		}
		for i, off := range byteToRune {
			if b == off {
				return i
			}
			if b < off {
				if i == 0 {
					return 0
				}
				return i - 1
			}
		}
		return len(byteToRune) - 1
	}
	fmt.Printf("# upem=%d len=%d\n", upem, n)
	for i := 0; i < n; i++ {
		fmt.Printf("%d %d %d %d %d %d\n",
			infos[i].Codepoint, clusterOfByte(int(infos[i].Cluster)),
			posns[i].XOffset, posns[i].YOffset,
			posns[i].XAdvance, posns[i].YAdvance)
	}

	hbBufferDestroy(buf)
	hbFontDestroy(font)
	hbFaceDestroy(face)
	hbBlobDestroy(blob)
	_ = data
}
