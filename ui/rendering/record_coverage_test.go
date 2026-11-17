//----------------------------------------
//
// Copyright © yanghy. All Rights Reserved.
//
// Licensed under Apache License Version 2.0, January 2004
//
// https://www.apache.org/licenses/LICENSE-2.0
//
//----------------------------------------

package rendering

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

// TestRecordCoverage_AllNodeTypesDeclareCacheContract is the machine gate for
// ENGINE_NODE_RECORD_COVERAGE_PLAN §5.4: every node type that paints in this
// package must declare its retained-cache contract ON THE SOURCE, one of:
//
//   - Record(rec, ox, oy, scale)  — faithful display-list capture (Recordable);
//   - Volatile() bool            — per-frame pixels, live paint only;
//   - structuralNode()           — never its own cache unit by design
//     (Viewport: content boundary is the unit; VirtualList: bind window).
//
// Enumeration is source-level (every type with a Paint method), not a
// hand-maintained whitelist, so adding a node type without the contract turns
// this red — the silent-slow-path regression (4/14) cannot recur.
func TestRecordCoverage_AllNodeTypesDeclareCacheContract(t *testing.T) {
	type nodeContract struct {
		file       string
		paint      bool
		record     bool
		volatile   bool
		structural bool
	}
	nodes := map[string]*nodeContract{}

	files, err := packageGoFiles()
	if err != nil {
		t.Fatalf("list package files: %v", err)
	}
	fset := token.NewFileSet()
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		af, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		for _, decl := range af.Decls {
			switch d := decl.(type) {
			case *ast.GenDecl:
				if d.Tok != token.TYPE {
					continue
				}
				for _, spec := range d.Specs {
					ts, ok := spec.(*ast.TypeSpec)
					if !ok {
						continue
					}
					if _, ok := ts.Type.(*ast.StructType); !ok {
						continue
					}
					nodes[ts.Name.Name] = &nodeContract{file: name}
				}
			case *ast.FuncDecl:
				if d.Recv == nil || len(d.Recv.List) == 0 {
					continue
				}
				n := nodes[recvTypeName(d.Recv.List[0].Type)]
				if n == nil {
					continue
				}
				switch d.Name.Name {
				case "Paint":
					n.paint = true
				case "Record":
					n.record = recordSignatureOK(d.Type)
				case "Volatile":
					n.volatile = volatileSignatureOK(d.Type)
				case "structuralNode":
					n.structural = structuralSignatureOK(d.Type)
				}
			}
		}
	}

	checked := 0
	var missing []string
	for name, n := range nodes {
		if !n.paint {
			continue
		}
		checked++
		if n.record || n.volatile || n.structural {
			continue
		}
		missing = append(missing, name+" ("+n.file+")")
	}
	if len(missing) > 0 {
		t.Fatalf("node types without a retained-cache contract: %s\n"+
			"implement Record(rec, ox, oy, scale) (faithful Paint parity), declare Volatile() bool, "+
			"or declare structuralNode() with a reason — see record.go",
			strings.Join(missing, ", "))
	}
	// 14 Render* nodes + AbsoluteBox + VirtualList. A lower count means the
	// source scan itself broke (renamed helper, moved files), not coverage.
	if checked < 16 {
		t.Fatalf("enumerated %d painting node types, want >=16 (scan broken?)", checked)
	}
}

// packageGoFiles lists the package's Go sources (same dir as this test).
func packageGoFiles() ([]string, error) {
	entries, err := os.ReadDir(".")
	if err != nil {
		return nil, err
	}
	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".go") {
			files = append(files, e.Name())
		}
	}
	return files, nil
}

func recvTypeName(expr ast.Expr) string {
	if star, ok := expr.(*ast.StarExpr); ok {
		expr = star.X
	}
	if id, ok := expr.(*ast.Ident); ok {
		return id.Name
	}
	return ""
}

// paramTypes expands a field list into one type per declared name (grouped
// names repeat their type), so signature checks count the real arity.
func paramTypes(fl *ast.FieldList) []ast.Expr {
	if fl == nil {
		return nil
	}
	var out []ast.Expr
	for _, f := range fl.List {
		n := len(f.Names)
		if n == 0 {
			n = 1
		}
		for i := 0; i < n; i++ {
			out = append(out, f.Type)
		}
	}
	return out
}

// recordSignatureOK checks Record(render.Canvas, float64, float64, float64).
// Without the type check a same-name method with the wrong signature would
// satisfy the gate while never fulfilling Recordable — the silent-slow-path
// hole again, only one level deeper.
func recordSignatureOK(ft *ast.FuncType) bool {
	if ft == nil || len(paramTypes(ft.Results)) != 0 {
		return false
	}
	ps := paramTypes(ft.Params)
	return len(ps) == 4 && isRenderCanvas(ps[0]) &&
		isFloat64(ps[1]) && isFloat64(ps[2]) && isFloat64(ps[3])
}

// volatileSignatureOK checks Volatile() bool.
func volatileSignatureOK(ft *ast.FuncType) bool {
	if ft == nil || len(paramTypes(ft.Params)) != 0 {
		return false
	}
	rs := paramTypes(ft.Results)
	if len(rs) != 1 {
		return false
	}
	id, ok := rs[0].(*ast.Ident)
	return ok && id.Name == "bool"
}

// structuralSignatureOK checks the marker method structuralNode().
func structuralSignatureOK(ft *ast.FuncType) bool {
	return ft != nil && len(paramTypes(ft.Params)) == 0 && len(paramTypes(ft.Results)) == 0
}

func isRenderCanvas(e ast.Expr) bool {
	se, ok := e.(*ast.SelectorExpr)
	if !ok || se.Sel == nil || se.Sel.Name != "Canvas" {
		return false
	}
	id, ok := se.X.(*ast.Ident)
	return ok && id.Name == "render"
}

func isFloat64(e ast.Expr) bool {
	id, ok := e.(*ast.Ident)
	return ok && id.Name == "float64"
}
