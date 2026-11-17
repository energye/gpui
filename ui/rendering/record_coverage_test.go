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
					// Record(rec, ox, oy, scale): four parameters, no results.
					n.record = paramCount(d.Type.Params) == 4 && d.Type.Results == nil
				case "Volatile":
					n.volatile = paramCount(d.Type.Params) == 0
				case "structuralNode":
					n.structural = paramCount(d.Type.Params) == 0
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

// paramCount counts declared parameters (grouped names count individually).
func paramCount(fl *ast.FieldList) int {
	if fl == nil {
		return 0
	}
	n := 0
	for _, f := range fl.List {
		if len(f.Names) == 0 {
			n++
			continue
		}
		n += len(f.Names)
	}
	return n
}
