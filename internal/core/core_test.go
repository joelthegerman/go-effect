package core

import (
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// allowedImports is the ALLOWLIST of packages a core file may import directly.
// Core must be pure, so this is deliberately tiny: standard-library packages
// that only transform data, plus core's own sub-packages. Anything else — os,
// net, log, math/rand, database/sql, or a new internal helper — is rejected.
//
// An allowlist, not a denylist, is the point: a denylist silently permits every
// impure package nobody remembered to list (the old check missed math/rand,
// crypto/rand, and any wrapper package). To add a genuinely-pure dependency,
// add it here on purpose — that edit is the review checkpoint.
var allowedImports = map[string]bool{
	"errors":  true,
	"fmt":     true,
	"strconv": true,
	"strings": true,
	"time":    true, // a data type; core carries timestamps but never calls now()
	"unicode": true,
}

// TestCoreIsPure enforces the purity boundary two ways that together close the
// hole a plain import denylist leaves open:
//
//  1. Direct imports of every core file must be on allowedImports (or a core
//     sub-package). Catches an impure package imported straight into core.
//  2. Core's full TRANSITIVE first-party dependency set must be core only.
//     Catches the subtler case the review flagged: a "pure-looking" helper
//     package that itself does I/O and gets imported (perhaps indirectly) by
//     core — invisible to a direct-import check.
//
// Neither check can prove a function performs no I/O through an ALLOWED package
// (e.g. calling time.Now); that is a fundamental limit of import analysis.
// Keeping the allowlist tiny is what keeps that residual risk small.
func TestCoreIsPure(t *testing.T) {
	assertDirectImportsAllowlisted(t)
	assertNoImpureFirstPartyDeps(t)
}

func assertDirectImportsAllowlisted(t *testing.T) {
	fset := token.NewFileSet()
	// Walk every core package (core and its feature subpackages like core/todos).
	err := filepath.WalkDir(".", func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil // dirs, non-Go, and tests (tests may touch the real world) are skipped
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		parsed, err := parser.ParseFile(fset, path, src, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, imp := range parsed.Imports {
			pkg := strings.Trim(imp.Path.Value, `"`)
			if allowedImports[pkg] || strings.HasPrefix(pkg, "agentic-sandbox/internal/core") {
				continue
			}
			t.Errorf("%s imports %q, which is not on the core allowlist — core "+
				"must be pure; return an Effect instead, or add the package to "+
				"allowedImports if it is genuinely pure", path, pkg)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func assertNoImpureFirstPartyDeps(t *testing.T) {
	// `go list -deps ./...` walks the FULL transitive dependency graph of core
	// and every feature subpackage under it. Stdlib entries are ignored (fmt and
	// time legitimately pull in os and syscall further down); only first-party
	// packages are policed, and the only ones core may reach are under core.
	out, err := exec.Command("go", "list", "-deps", "./...").Output()
	if err != nil {
		t.Fatalf("go list -deps: %v", err)
	}
	for _, dep := range strings.Fields(string(out)) {
		if !strings.HasPrefix(dep, "agentic-sandbox/") {
			continue
		}
		if strings.HasPrefix(dep, "agentic-sandbox/internal/core") {
			continue
		}
		t.Errorf("core transitively depends on first-party package %q — core must "+
			"not reach the shell or any package that performs I/O", dep)
	}
}
