// apiscan 提取 Go 包目录的全部公开符号（与 scripts/apidoc 同口径的独立实现）：
//   - 顶层导出 FuncDecl（无接收者）          → kind=func
//   - 导出类型的导出方法（FuncDecl 有接收者）→ kind=method
//   - 顶层导出 type                          → kind=type
//   - 接口类型声明的方法集合                  → kind=ifacemethod
//   - 顶层导出 const/var（含块内逐名）        → kind=const / var
// 结构体字段不单列（对齐 RENDER_API_CATALOG 口径：成员随类型行）。
// 用法: go run apiscan.go <pkg目录>...   输出 TSV: pkg kind recv name file line
package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func exported(name string) bool {
	return name != "" && token.IsExported(name)
}

func main() {
	var rows []string
	for _, dir := range os.Args[1:] {
		pkg := filepath.Base(dir)
		entries, err := os.ReadDir(dir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "skip %s: %v\n", dir, err)
			continue
		}
		var files []string
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
				continue
			}
			files = append(files, filepath.Join(dir, e.Name()))
		}
		sort.Strings(files)
		fset := token.NewFileSet()
		for _, f := range files {
			af, err := parser.ParseFile(fset, f, nil, 0)
			if err != nil {
				fmt.Fprintf(os.Stderr, "parse %s: %v\n", f, err)
				continue
			}
			pos := func(n ast.Node) (string, int) {
				p := fset.Position(n.Pos())
				return p.Filename, p.Line
			}
			for _, decl := range af.Decls {
				switch d := decl.(type) {
				case *ast.FuncDecl:
					name := d.Name.Name
					if !exported(name) {
						continue
					}
					fi, ln := pos(d)
					if d.Recv != nil && len(d.Recv.List) > 0 {
						recv := ast.Expr(d.Recv.List[0].Type)
						if st, ok := recv.(*ast.StarExpr); ok {
							recv = st.X
						}
						if id, ok := recv.(*ast.Ident); ok && exported(id.Name) {
							rows = fmtAppend(rows, pkg, "method", id.Name, name, fi, ln)
						}
						continue
					}
					rows = fmtAppend(rows, pkg, "func", "", name, fi, ln)
				case *ast.GenDecl:
					for _, spec := range d.Specs {
						switch s := spec.(type) {
						case *ast.TypeSpec:
							if !exported(s.Name.Name) {
								continue
							}
							fi, ln := pos(s)
							rows = fmtAppend(rows, pkg, "type", "", s.Name.Name, fi, ln)
							if it, ok := s.Type.(*ast.InterfaceType); ok {
								for _, m := range it.Methods.List {
									for _, n := range m.Names {
										if exported(n.Name) {
											mi, ml := pos(m)
											rows = fmtAppend(rows, pkg, "ifacemethod", s.Name.Name, n.Name, mi, ml)
										}
									}
								}
							}
						case *ast.ValueSpec:
							kind := "var"
							if d.Tok == token.CONST {
								kind = "const"
							}
							for _, n := range s.Names {
								if !exported(n.Name) {
									continue
								}
								fi, ln := pos(s)
								rows = fmtAppend(rows, pkg, kind, "", n.Name, fi, ln)
							}
						}
					}
				}
			}
		}
	}
	for _, r := range rows {
		fmt.Println(r)
	}
}

func fmtAppend(rows []string, pkg, kind, recv, name, file string, line int) []string {
	return append(rows, fmt.Sprintf("%s\t%s\t%s\t%s\t%s\t%d", pkg, kind, recv, name, file, line))
}
