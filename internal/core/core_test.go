package core

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestCoreIsPure fails the build if any core file imports a side-effecting
// package. This is what makes the split REAL and not just a naming convention.
func TestCoreIsPure(t *testing.T) {
	forbidden := []string{"os", "net", "net/http", "database/sql", "os/exec", "syscall", "log"}
	files, _ := filepath.Glob("*.go")
	fset := token.NewFileSet()

	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue // tests may touch the real world
		}
		src, _ := os.ReadFile(f)
		parsed, err := parser.ParseFile(fset, f, src, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, imp := range parsed.Imports {
			pkg := strings.Trim(imp.Path.Value, `"`)
			for _, bad := range forbidden {
				if pkg == bad {
					t.Errorf("%s imports %q: core must be pure", f, pkg)
				}
			}
		}
	}
}
