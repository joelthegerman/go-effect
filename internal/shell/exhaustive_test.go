package shell

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"
)

// TestRunHandlesEveryEffect fails if package core defines an Effect that
// shell.Run's type switch does not handle (or handles a case for an effect
// that no longer exists). This makes the executor's switch exhaustive by
// construction: you cannot add an effect and silently forget to wire it up.
func TestRunHandlesEveryEffect(t *testing.T) {
	effects := effectTypes(t)  // every type in core with an isEffect() method
	handled := handledInRun(t) // every `case core.X:` in shell.go

	for name := range effects {
		if !handled[name] {
			t.Errorf("core.%s has no case in shell.Run — add one", name)
		}
	}
	for name := range handled {
		if !effects[name] {
			t.Errorf("shell.Run handles core.%s but no such effect exists", name)
		}
	}
}

func effectTypes(t *testing.T) map[string]bool {
	t.Helper()
	out := map[string]bool{}
	files, _ := filepath.Glob("../core/*.go")
	fset := token.NewFileSet()
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, f, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range file.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if ok && fn.Name.Name == "isEffect" && fn.Recv != nil {
				out[receiverName(fn.Recv.List[0].Type)] = true
			}
		}
	}
	return out
}

func handledInRun(t *testing.T) map[string]bool {
	t.Helper()
	out := map[string]bool{}
	fset := token.NewFileSet()

	// Scan every non-test file in this package for a type switch on core.*
	// effects, so it doesn't matter which file Run lives in.
	files, _ := filepath.Glob("*.go")
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, f, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			sw, ok := n.(*ast.TypeSwitchStmt)
			if !ok {
				return true
			}
			for _, stmt := range sw.Body.List {
				cc, ok := stmt.(*ast.CaseClause)
				if !ok {
					continue
				}
				for _, expr := range cc.List {
					if sel, ok := expr.(*ast.SelectorExpr); ok {
						if id, ok := sel.X.(*ast.Ident); ok && id.Name == "core" {
							out[sel.Sel.Name] = true
						}
					}
				}
			}
			return true
		})
	}
	return out
}

func receiverName(expr ast.Expr) string {
	switch v := expr.(type) {
	case *ast.Ident:
		return v.Name
	case *ast.StarExpr:
		return receiverName(v.X)
	}
	return ""
}
