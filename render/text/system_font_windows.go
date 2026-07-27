//go:build windows

package text

import (
	"os"
	"path/filepath"
)

// platformSystemFontCandidates uses %WINDIR%\Fonts (and common installs).
func platformSystemFontCandidates(b FontRole) []string {
	root := windowsFontsDir()
	join := func(names ...string) []string {
		out := make([]string, 0, len(names))
		for _, n := range names {
			out = append(out, filepath.Join(root, n))
		}
		return out
	}
	switch b {
	case FontRoleLatin:
		return join(
			"segoeui.ttf",
			"arial.ttf",
			"calibri.ttf",
			"tahoma.ttf",
			"verdana.ttf",
		)
	case FontRoleCJK:
		// YaHei / JhengHei / Malgun / Yu Gothic / Meiryo — cover zh/ja/ko on typical installs.
		return join(
			"msyh.ttc", // Microsoft YaHei
			"msyh.ttf",
			"msyhbd.ttc",
			"msjh.ttc", // Microsoft JhengHei (zh-TW)
			"msjh.ttf",
			"malgun.ttf", // Korean
			"YuGothR.ttc",
			"yugothic.ttf",
			"meiryo.ttc",
			"msgothic.ttc",
			"simsun.ttc",
			"simhei.ttf",
		)
	case FontRoleThai:
		return join(
			"LeelawadeeUI.ttf",
			"leelawad.ttf",
			"Leelawadee.ttf",
			"cordia.ttf",
			"NotoSansThai-Regular.ttf",
		)
	case FontRoleDevanagari:
		return join(
			"Nirmala.ttf",
			"NirmalaUI.ttf",
			"mangal.ttf",
			"NotoSansDevanagari-Regular.ttf",
		)
	case FontRoleArabic:
		return join(
			"segoeui.ttf",
			"tahoma.ttf",
			"arial.ttf",
			"Traditional Arabic.ttf",
			"NotoSansArabic-Regular.ttf",
		)
	default:
		return nil
	}
}

func windowsFontsDir() string {
	if windir := os.Getenv("WINDIR"); windir != "" {
		return filepath.Join(windir, "Fonts")
	}
	if systemroot := os.Getenv("SystemRoot"); systemroot != "" {
		return filepath.Join(systemroot, "Fonts")
	}
	return `C:\Windows\Fonts`
}
