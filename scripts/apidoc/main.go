// Command apidoc 校验 docs/RENDER_API_CATALOG.md 与 render 主包（以及
// scene/recording/surface/svg 子包）公开 API 的一致性：任何顶层符号与
// 导出方法必须出现在目录文档文本中，否则报缺失并以非零码退出（可挂 CI）。
//
// 用途：AGENTS.md「API 文档同步纪律」的机器抓手。改 render 代码后跑：
//
//	go run ./scripts/apidoc
//
// 跳过规则：render/text 采用族级归纳（附录以 go doc 为准），不参与逐符号检查；
// render/filters、render/raster 是纯 init 副作用包（无顶层符号），跳过。
package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const docPath = "docs/RENDER_API_CATALOG.md"

// scanPkg 解析包顶层目录下的 .go 文件（非递归、排除 _test.go），返回：
// 顶层导出符号（func/type/const/var）与各导出类型的导出方法。
// 注意：render/ 下还有 render/internal、render/text、render/render 等子包，
// 包名同为 render，必须只读目录直属文件（子包自成 import 路径，不在目录内）。
func scanPkg(dir string) (top []string, methods map[string][]string) {
	methods = map[string][]string{}
	fset := token.NewFileSet()
	entries, err := os.ReadDir(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "readdir %s: %v\n", dir, err)
		return
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		path := filepath.Join(dir, e.Name())
		f, perr := parser.ParseFile(fset, path, nil, 0)
		if perr != nil {
			fmt.Fprintf(os.Stderr, "parse %s: %v\n", path, perr)
			continue
		}
		for _, d := range f.Decls {
			switch decl := d.(type) {
			case *ast.FuncDecl:
				if !decl.Name.IsExported() {
					continue
				}
				if decl.Recv == nil {
					top = append(top, decl.Name.Name)
					continue
				}
				recv := ""
				if len(decl.Recv.List) > 0 {
					switch t := decl.Recv.List[0].Type.(type) {
					case *ast.StarExpr:
						if id, ok := t.X.(*ast.Ident); ok {
							recv = "*" + id.Name
						}
					case *ast.Ident:
						recv = t.Name
					}
				}
				methods[recv] = append(methods[recv], decl.Name.Name)
			case *ast.GenDecl:
				for _, s := range decl.Specs {
					switch spec := s.(type) {
					case *ast.TypeSpec:
						if spec.Name.IsExported() {
							top = append(top, spec.Name.Name)
						}
					case *ast.ValueSpec:
						for _, n := range spec.Names {
							if n.IsExported() {
								top = append(top, n.Name)
							}
						}
					}
				}
			}
		}
	}
	sort.Strings(top)
	return top, methods
}

func main() {
	doc, err := os.ReadFile(docPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "read doc:", err)
		os.Exit(2)
	}
	text := string(doc)

	// 按 word boundary 匹配标识符（\bName\b）：避免 MoveTo 被 DrawString 里的
	// 子串误判、RGBA 匹配 RGBA2 等。
	has := func(name string) bool {
		re, err := regexp.Compile(`\b` + regexp.QuoteMeta(name) + `\b`)
		if err != nil {
			return strings.Contains(text, name)
		}
		return re.MatchString(text)
	}

	var missing []string
	check := func(kind string, names []string) {
		for _, n := range names {
			if !has(n) {
				missing = append(missing, kind+" "+n)
			}
		}
	}

	// 主包
	top, methods := scanPkg("render")
	check("mainpkg", top)
	for recv := range methods {
		sort.Strings(methods[recv])
		// 主包公开接收者：类型或 *类型（首字母大写）
		base := strings.TrimPrefix(recv, "*")
		if base == "" || !(base[0] >= 'A' && base[0] <= 'Z') {
			continue
		}
		check("method", methods[recv])
	}

	// 子包顶层（scene/recording/surface/svg；text 族级、filters/raster 无符号）
	for _, sub := range []string{"scene", "recording", "surface", "svg"} {
		t, _ := scanPkg("render/" + sub)
		check("subpkg", t)
	}

	if len(missing) > 0 {
		fmt.Printf("API 目录缺失 %d 个符号（docs/RENDER_API_CATALOG.md 未出现）：\n", len(missing))
		for _, m := range missing {
			fmt.Println("  " + m)
		}
		fmt.Println("→ 请在 docs/RENDER_API_CATALOG.md 对应分类表补齐（增/改/删 API 时同步更新）。")
		os.Exit(1)
	}
	fmt.Println("OK: docs/RENDER_API_CATALOG.md 覆盖 render 主包 + scene/recording/surface/svg 全部公开符号")
}

func mapToPrefixed(names []string, prefix string) []string {
	out := make([]string, len(names))
	for i, n := range names {
		out[i] = prefix + n
	}
	return out
}