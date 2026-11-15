// tokenscan 对 Go 源码做词法级标识符提取（只输出真实代码里的 IDENT，
// 跳过注释与字符串字面量——避免「示例注释提到 API 名」被误算作使用）。
// 用法: go run tokenscan.go <文件|目录>...   输出 TSV: IDENT\tfile
package main

import (
	"go/scanner"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	for _, arg := range os.Args[1:] {
		st, err := os.Stat(arg)
		if err != nil {
			continue
		}
		var files []string
		if st.IsDir() {
			filepath.Walk(arg, func(p string, fi os.FileInfo, err error) error {
				if err == nil && !fi.IsDir() && strings.HasSuffix(p, ".go") && !strings.HasSuffix(p, "_test.go") {
					files = append(files, p)
				}
				return nil
			})
		} else if strings.HasSuffix(arg, ".go") && !strings.HasSuffix(arg, "_test.go") {
			files = append(files, arg)
		}
		for _, f := range files {
			src, err := os.ReadFile(f)
			if err != nil {
				continue
			}
			fset := token.NewFileSet()
			var s scanner.Scanner
			s.Init(fset.AddFile(f, fset.Base(), len(src)), src, nil, 0)
			for {
				_, tok, lit := s.Scan()
				if tok == token.EOF {
					break
				}
				if tok == token.IDENT {
					os.Stdout.WriteString(lit + "\t" + f + "\n")
				}
			}
		}
	}
}
