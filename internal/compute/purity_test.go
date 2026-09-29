package compute

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// TestComputeIsPure enforces the rule in CLAUDE.md: the financial model is
// pure functions. Non-test files under internal/compute may import the
// standard library (minus I/O: network, files, processes, databases), uuid,
// decimal, and Ascenda's model and pkg types — nothing else, so no GORM, no
// net/http, no repo, service or handler.
func TestComputeIsPure(t *testing.T) {
	allowedModules := []string{
		"github.com/google/uuid",
		"github.com/shopspring/decimal",
		"ascenda/internal/model",
		"ascenda/pkg/",
	}
	forbiddenStd := []string{"net", "os", "database", "syscall", "plugin", "log/syslog"}

	fset := token.NewFileSet()
	checked := 0
	err := filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return err
		}
		f, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		checked++
		for _, spec := range f.Imports {
			imp, _ := strconv.Unquote(spec.Path.Value)
			if isStdlib(imp) {
				if hasPathPrefix(imp, forbiddenStd) {
					t.Errorf("%s imports %q: compute must not do I/O", path, imp)
				}
				continue
			}
			if !hasPathPrefix(imp, allowedModules) {
				t.Errorf("%s imports %q: compute may only use the standard library, uuid, decimal and model/pkg types", path, imp)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if checked == 0 {
		t.Fatal("no Go files checked")
	}
}

// isStdlib reports whether an import path belongs to the standard library,
// whose first element never contains a dot (and is not the module name).
func isStdlib(imp string) bool {
	first, _, _ := strings.Cut(imp, "/")
	return !strings.Contains(first, ".") && first != "ascenda"
}

// hasPathPrefix matches whole path elements: "net" matches "net" and
// "net/http" but not "netip"; an entry ending in "/" matches its subtree only.
func hasPathPrefix(imp string, prefixes []string) bool {
	for _, p := range prefixes {
		if strings.HasSuffix(p, "/") {
			if strings.HasPrefix(imp, p) {
				return true
			}
			continue
		}
		if imp == p || strings.HasPrefix(imp, p+"/") {
			return true
		}
	}
	return false
}
