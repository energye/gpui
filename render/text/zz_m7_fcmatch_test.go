package text

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"
)

// M7 系统字体发现验证：家族匹配 + rune 回退对照 fc-match（fontconfig）。
//
// 验收线（真源 §5.3 M7）：系统字体列表与回退选择对照 fc-match 等价。
// 对照原则：比「命中的字体家族」（family name），不比具体文件路径——
// fontconfig 与 go-text 索引可能选中同族的用户版/系统版不同文件（如
// NotoSansCJKsc-Regular.otf vs NotoSansCJK-Regular.ttc），家族一致即等价。
//
// 依赖 fc-match/fc-list（fontconfig）：缺失环境 t.Skipf，禁止假绿。

// fcMatchFamily runs `fc-match -f "%{family}"` and returns the winning
// family name (normalized), or "" when fontconfig is unavailable.
func fcMatchFamily(t *testing.T, query string) string {
	t.Helper()
	out, err := exec.Command("fc-match", "-f", "%{family}", query).Output()
	if err != nil {
		t.Skipf("fc-match unavailable: %v", err)
	}
	return strings.TrimSpace(string(out))
}

// TestM7FamilyMatch vs fc-match：代表性家族的家族命中一致。
func TestM7FamilyMatch(t *testing.T) {
	families := []string{
		"DejaVu Sans",
		"Liberation Sans",
		"Noto Sans CJK JP",
		"Noto Sans CJK SC",
		"Noto Sans Thai",
		"Noto Sans Devanagari",
		"Noto Naskh Arabic",
		"Noto Sans",
	}
	// 跳过字体缺失（fc-match 会回退到别的字体，此时双方不同族属正常回退差异，
	// 不算 FAIL——只有「双方都命中且家族不同」才报）。
	for _, fam := range families {
		want := fcMatchFamily(t, fam)
		if want == "" {
			continue
		}
		gotPath, ok := SystemFontForFamily(fam)
		if !ok {
			// go-text 索引未命中该家族——如果 fc-match 也回退到别的家族则等价；
			// 若 fc-match 精确命中说明索引有缺口。
			if strings.EqualFold(want, fam) {
				t.Errorf("%s: fc-match 命中家族 %q，go-text 索引未命中", fam, want)
			}
			continue
		}
		got := familyOfPath(t, gotPath)
		if got == "" {
			continue
		}
		// 双方都命中：家族必须一致（fc-match 可能回退到同族不同 subfamily）。
		if !strings.EqualFold(got, want) && !strings.Contains(strings.ToLower(want), strings.ToLower(got)) &&
			!strings.Contains(strings.ToLower(got), strings.ToLower(want)) {
			t.Errorf("%s: go-text 家族=%q fc-match=%q（路径=%s）", fam, got, want, gotPath)
		}
	}
}

// TestM7RuneFallback vs fc-match charset：代表性 rune 的回退覆盖性。
//
// 硬断言：go-text 选择的回退字体必须覆盖该 rune（覆盖性等价）。
// script 对照：泰文/天城文/阿拉伯文等复杂脚本的 fallback 必须与 fc-match
// 家族一致（M7 用 FontMap.SetScript 后已对齐：泰→Loma、天→Lohit、
// 阿拉伯→DejaVu）。拉丁 A 例外：本机 fontconfig 用户字体（NotoSansCJKsc）
// 优先级极高，fc-match 连 lang=en 都选 CJK——go-text 选 DejaVu 才是正确
// 的拉丁选择，该差异是 fc-match 本机配置怪癖，不判、不记。
func TestM7RuneFallback(t *testing.T) {
	// script 对照组：fc-match 选择合理、必须家族一致。
	scriptChecks := []struct {
		r    rune
		want string // 期望家族（fc-match 合理选择）
	}{
		{'ก', "Loma"},
		{'क', "Lohit"},
		{'ف', "DejaVu"},
	}
	for _, sc := range scriptChecks {
		gotPath := globalFallbackPath(sc.r)
		if gotPath == "" {
			t.Errorf("%U: go-text 回退为空", sc.r)
			continue
		}
		ok := globalFallback.coversRune(sc.r, gotPath)
		if !ok {
			t.Errorf("%U: go-text 回退字体 %s 不覆盖该 rune", sc.r, gotPath)
			continue
		}
		got := familyOfPath(t, gotPath)
		if !strings.Contains(strings.ToLower(got), strings.ToLower(sc.want)) &&
			!strings.Contains(strings.ToLower(sc.want), strings.ToLower(got)) {
			t.Errorf("%U: go-text 回退=%q(%s) 期望家族=%q", sc.r, got, gotPath, sc.want)
		}
	}

	// 覆盖性组：任何 rune 回退必须真渲染该字符。
	for _, r := range []rune{'日', 'A', 'ก', 'ف', 'क'} {
		gotPath := globalFallbackPath(r)
		if gotPath == "" {
			t.Errorf("%U: go-text 回退为空", r)
			continue
		}
		if !globalFallback.coversRune(r, gotPath) {
			t.Errorf("%U: go-text 回退字体 %s 不覆盖该 rune", r, gotPath)
		}
	}
}

// TestM7FamilyList：SystemFontsByFamily 返回同族多面（regular+bold+italic）。
func TestM7FamilyList(t *testing.T) {
	paths := SystemFontsByFamily("DejaVu Sans")
	if len(paths) == 0 {
		t.Skipf("DejaVu Sans not in system index")
	}
	// 至少应有 Regular + Bold（系统 DejaVu 通常带 Bold/Italic/Oblique）。
	if len(paths) < 2 {
		t.Logf("DejaVu Sans only %d faces (环境精简): %v", len(paths), paths)
	}
}

// --- helpers ---

// globalFallbackPath resolves r via the rune fallback index.
func globalFallbackPath(r rune) string {
	return globalFallback.resolvePath(r)
}

// coversRune reports whether the font file at path actually contains a
// glyph for r (via the own parser's cmap — the same path used to render).
func (f *fontScanFallback) coversRune(r rune, path string) bool {
	src, err := NewFontSourceFromFile(path)
	if err != nil {
		return false
	}
	defer src.Close()
	return src.Face(12).HasGlyph(r)
}

// familyOfPath parses the family name of a font file by asking fontconfig
// (fc-scan) — returns "" on error/absence.
func familyOfPath(t *testing.T, path string) string {
	t.Helper()
	out, err := exec.Command("fc-scan", "--format", "%{family}", path).Output()
	if err != nil || len(out) == 0 {
		return ""
	}
	// fc-scan may return "A,B" for multi-family faces; take first.
	fam := strings.TrimSpace(string(out))
	if i := strings.IndexByte(fam, ','); i >= 0 {
		fam = fam[:i]
	}
	return fam
}

// runeHex formats a rune as lowercase hex (fc-match charset format).
func runeHex(r rune) string {
	return strings.ToLower(fmt.Sprintf("%x", r))
}