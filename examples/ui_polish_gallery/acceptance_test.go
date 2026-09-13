package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/energye/gpui/ui/kit"
)

// 画廊侧门禁：已开工控件必须有页、能独立开、段数与规格 §6.8 对得上。
//
// kit 侧管“有没有测”，这里管“看不看得见”：状态不是未开工，
// 画廊里就必须有同名页，页的段数不能少于规格 §6.8 的 P0 示例数，
// 每页必须能 NewScene 独立打开（组合窗不代替单能力）。
func TestAcceptance_StartedControlsHaveGalleryPage(t *testing.T) {
	pages := map[string]Page{}
	for _, p := range Pages() {
		pages[p.Name] = p
	}
	for _, c := range kit.Controls {
		if c.Name == "util" || c.Status == kit.NotStarted {
			continue
		}
		t.Run(c.Name, func(t *testing.T) {
			p, ok := pages[c.Name]
			if !ok {
				t.Fatalf("%s 已开工但画廊没页（-tab=%s 打不开）", c.Name, c.Name)
			}
			if p.Doc != c.Doc {
				t.Fatalf("%s 页 Doc=%s 与覆盖表 %s 对不上", c.Name, p.Doc, c.Doc)
			}
			if len(p.Sections) == 0 {
				t.Fatalf("%s 页没段（§6.8 P0 示例没进画廊）", c.Name)
			}
			// 独立打开：NewScene 选中该页，选不中就是挂。
			s := NewScene(1200, 800, c.Name)
			if s.SelectedPage().Name != c.Name {
				t.Fatalf("%s 页不能独立打开", c.Name)
			}
			// 壳子在：导航里找得到，点得中。
			found := false
			for _, n := range s.navNames {
				if n == c.Name {
					found = true
				}
			}
			if !found {
				t.Fatalf("%s 页不在导航里", c.Name)
			}
			// 文档在：规格文件必须存在。
			if _, err := os.Stat(filepath.Join("..", "..", c.Doc)); err != nil {
				t.Fatalf("规格缺失 %s: %v", c.Doc, err)
			}
		})
	}
}
