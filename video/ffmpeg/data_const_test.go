package ffmpeg

import (
	"strings"
	"testing"
)

// 大白话：前面函数都能调了，这里验剩下的数据也能拿到，别出现空读。
func TestDataConst(t *testing.T) {
	if !Available() {
		t.Skipf("lib missing: %s", LibPath())
	}
	var c Crypto
	var u Util
	var l Library
	sizes := map[string]int{
		"aes":      c.AesSize(),
		"camellia": c.CamelliaSize(),
		"cast5":    c.Cast5Size(),
		"tea":      c.TeaSize(),
		"twofish":  c.TwofishSize(),
		"md5":      c.Md5Size(),
		"ripemd":   c.RipemdSize(),
		"sha":      c.ShaSize(),
		"sha512":   c.Sha512Size(),
		"tree":     u.TreeNodeSize(),
	}
	for k, v := range sizes {
		if v <= 0 {
			t.Fatalf("%s size = %d, want >0", k, v)
		}
	}
	strs := map[string]string{
		"codec":  l.CodecFfversion(),
		"device": l.DeviceFfversion(),
		"filter": l.FilterFfversion(),
		"format": l.FormatFfversion(),
		"util":   l.UtilFfversion(),
		"swr":    l.SwrFfversion(),
	}
	for k, v := range strs {
		if !strings.Contains(v, "FFmpeg") {
			t.Fatalf("%s ffversion = %q, want FFmpeg version", k, v)
		}
	}
}
