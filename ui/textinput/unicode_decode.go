package textinput

import (
	"strconv"
	"strings"
	"unicode/utf8"
)

// decodeUnicodeEscapes 把 "\u4f60\u597d" 这类字面转义解回 "你好"。
// 只处理形如 \uXXXX（4位16进制）和 \UXXXXXXXX（8位），其它保持原样。
// 若串里没有 "\u" / "\U" 则原样返回，避免正常文本被误改。
func decodeUnicodeEscapes(s string) string {
	if !strings.Contains(s, "\\u") && !strings.Contains(s, "\\U") {
		return s
	}
	// 先让 strconv 帮我们解最常见的 \uXXXX / \n 等：把串包成引号再 Unquote
	// 外部复制来的往往就是 JSON 片段，去掉首尾引号后刚好能解
	// 为避免误伤正常带反斜杠的文本，只有确实含 \u 时才尝试
	if q, err := strconv.Unquote(`"` + strings.ReplaceAll(s, `"`, `\"`) + `"`); err == nil {
		// Unquote 会把 \uXXXX 解掉，若结果含有效 UTF-8 且与原串不同则用它
		if utf8.ValidString(q) && q != s {
			// 再确认原串里确实是 \u 转义而不是正常反斜杠：q 里不应再有 "\u"
			// 若 q 仍含 "\u" 说明外层还有一层转义，继续解一轮
			if strings.Contains(q, "\\u") || strings.Contains(q, "\\U") {
				if q2, err2 := strconv.Unquote(`"` + strings.ReplaceAll(q, `"`, `\"`) + `"`); err2 == nil && utf8.ValidString(q2) {
					return q2
				}
			}
			return q
		}
	}
	// 回退：手工只解 \uXXXX / \UXXXXXXXX，避免 Unquote 失败时整体不解
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); {
		if s[i] == '\\' && i+1 < len(s) && (s[i+1] == 'u' || s[i+1] == 'U') {
			hexLen := 4
			if s[i+1] == 'U' {
				hexLen = 8
			}
			if i+2+hexLen <= len(s) {
				hex := s[i+2 : i+2+hexLen]
				if v, err := strconv.ParseUint(hex, 16, 32); err == nil {
					b.WriteRune(rune(v))
					i += 2 + hexLen
					continue
				}
			}
		}
		b.WriteByte(s[i])
		i++
	}
	out := b.String()
	if out == s {
		return s
	}
	return out
}
