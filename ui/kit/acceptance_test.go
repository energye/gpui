package kit_test

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/energye/gpui/ui/kit"
)

// 中央门禁：状态与文件对不上就挂。
//
// 规则：只要 coverage.go 里某控件不是“未开工”，
// 以下七件必须齐。想标“首批完”，先把测试和文件补齐，
// 否则这个测试直接挂掉，合不进去。
// 出处见 docs/antd/ACCEPTANCE.md 第七节。

// specIDs 读规格 §6.9 表里的用例号（如 ICO-01）。
var specIDRe = regexp.MustCompile(`\| ([A-Z]+-[0-9A-Z]+) \|`)

// testPRD 读测试名里的用例号（如 TestIcon_PRD_ICO01 中的 ICO01）。
var testPRDRe = regexp.MustCompile(`PRD_([A-Z]+0*([0-9]+))`)

func normID(prefix, num string) string {
	n, _ := strconv.Atoi(num)
	return prefix + "-" + strconv.Itoa(n)
}

func specP0IDs(t *testing.T, doc string) map[string]bool {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", doc))
	if err != nil {
		t.Fatalf("read spec %s: %v", doc, err)
	}
	text := string(b)
	// 只取 §6.9 表：从 "### 6.9" 到下一个 "### 6.10"。
	start := strings.Index(text, "### 6.9")
	end := strings.Index(text, "### 6.10")
	if start < 0 || end < 0 || end <= start {
		t.Fatalf("%s 缺少 §6.9/§6.10（验收用例表是必须的）", doc)
	}
	ids := map[string]bool{}
	for _, m := range specIDRe.FindAllStringSubmatch(text[start:end], -1) {
		raw := m[1]
		// 跳过 P1/N/A/L3/L4 行：整行含这些标记的不算 P0。
		line := ""
		for _, l := range strings.Split(text[start:end], "\n") {
			if strings.Contains(l, raw) {
				line = l
				break
			}
		}
		if strings.Contains(line, "P1") || strings.Contains(line, "N/A") ||
			strings.Contains(line, "L3") || strings.Contains(line, "L4") {
			continue
		}
		ids[raw] = true
	}
	if len(ids) == 0 {
		t.Fatalf("%s §6.9 里一个 P0 用例都没读到", doc)
	}
	return ids
}

func kitTestFiles(t *testing.T, name string) []string {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(name, "*_test.go"))
	if err != nil || len(matches) == 0 {
		t.Fatalf("%s 缺测试文件（七件套第1件：行为测试必须有）", name)
	}
	return matches
}

func fileHas(t *testing.T, path, sub string) bool {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return strings.Contains(string(b), sub)
}

func TestAcceptance_StartedControlsMustBeComplete(t *testing.T) {
	for _, c := range kit.Controls {
		if c.Name == "util" || c.Status == kit.NotStarted {
			continue
		}
		t.Run(c.Name, func(t *testing.T) {
			// 1. 行为：有测试文件，且每个 P0 用例号在测试名里出现。
			files := kitTestFiles(t, c.Name)
			all := ""
			for _, f := range files {
				b, err := os.ReadFile(f)
				if err != nil {
					t.Fatalf("read %s: %v", f, err)
				}
				all += string(b)
			}
			if !strings.Contains(all, "PRD_") {
				t.Fatalf("%s 测试名缺 PRD_ 前缀（用例与 §6.9 对不上）", c.Name)
			}
			want := specP0IDs(t, c.Doc)
			for id := range want {
				parts := strings.Split(id, "-")
				flat := parts[0] + strings.TrimLeft(parts[1], "0")
				// 兼容 ICO01 / ICO1 两种写法。
				found := strings.Contains(all, parts[0]+"_"+parts[1]) ||
					strings.Contains(all, parts[0]+parts[1]) ||
					strings.Contains(all, flat) ||
					strings.Contains(all, id)
				if !found {
					t.Fatalf("%s 缺 P0 用例 %s 的测试（§6.9 要求全过）", c.Name, id)
				}
			}

			// 2. 布局矩阵：测试里必须调过 Layout（Exact/Min/Max 约束见单测内）。
			hasLayout := false
			for _, f := range files {
				if fileHas(t, f, "Layout(") {
					hasLayout = true
				}
			}
			if !hasLayout {
				t.Fatalf("%s 缺布局断言（七件套第2件）", c.Name)
			}

			// 3. 录制断言：必须有 PaintVisits 脏局部用例
			// （全量画一遍记个数，脏一个再画，第二次远小于第一次；
			// 钩子见 ui/rendering/paint_context.go，用法抄 render_matrix_test.go）。
			// 光“画过”不算，必须证明“只脏局部”，否则就是假绿。
			if !strings.Contains(all, "PaintVisits") {
				t.Fatalf("%s 缺脏局部用例（七件套第3件：PaintVisits 全量/局部两次计数）", c.Name)
			}

			// 4. 数据文件：testdata 必须有东西（标准数不许现编）。
			entries, err := os.ReadDir(filepath.Join(c.Name, "testdata"))
			if err != nil || len(entries) == 0 {
				t.Fatalf("%s 缺 testdata（七件套第4件：标准数必须落文件）", c.Name)
			}

			// 5. 无障碍：必须断过焦点或角色。
			if !strings.Contains(all, "Focus") && !strings.Contains(all, "Role(") && !strings.Contains(all, "Aria") {
				t.Fatalf("%s 缺无障碍断言（七件套第6件：焦点/角色）", c.Name)
			}

			// 6. 主题：必须读过主题不断言写死色。
			if !strings.Contains(all, "theme.") && !strings.Contains(all, "Theme") && !strings.Contains(all, "Token") {
				t.Fatalf("%s 缺主题断言（七件套第7件：颜色走主题）", c.Name)
			}

			// 7. 纪律：kit 里不许碰显卡包，不许用 cgo，不过测试数据目录。
			fset := token.NewFileSet()
			walk, err := filepath.Glob(filepath.Join(c.Name, "*.go"))
			if err != nil {
				t.Fatal(err)
			}
			for _, f := range walk {
				if strings.HasSuffix(f, "_test.go") {
					continue
				}
				node, err := parser.ParseFile(fset, f, nil, parser.ImportsOnly)
				if err != nil {
					t.Fatalf("parse %s: %v", f, err)
				}
				for _, imp := range node.Imports {
					p, _ := strconv.Unquote(imp.Path.Value)
					// render 抽象层允许，直连显卡后端才拦：
					// 顶层 gpu 包与 render/gpu 后端。注意 gpui 前缀含 gpu
					// 三个字母，子串匹配会误杀，必须按路径段判。
					segs := strings.Split(p, "/")
					forbidden := p == "C"
					for _, s := range segs {
						if s == "gpu" {
							forbidden = true
						}
					}
					if p == "github.com/energye/gpui/render" {
						forbidden = false
					}
					if forbidden {
						t.Fatalf("%s/%s 引用了 %s（引擎纪律：ui 不直连显卡）", c.Name, filepath.Base(f), p)
					}
				}
				b, _ := os.ReadFile(f)
				if strings.Contains(string(b), "/home/") {
					t.Fatalf("%s 有本机绝对路径（测试数据必须进 testdata）", f)
				}
			}

			// 8. 文档：P0Done 必须有 P0 范围说明，P1 必须指到规格。
			if c.Status == kit.P0Done && c.P0 == "" {
				t.Fatalf("%s 标首批完却没写 P0 范围", c.Name)
			}
			if c.P1 == "" {
				t.Fatalf("%s 缺 P1 去向（欠什么必须写清）", c.Name)
			}
		})
	}
}
