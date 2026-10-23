package internal_test

import (
	"go/build"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"
)

const module = "github.com/get-phanuwat-api1/backend-challenge"

// The dependency rule of hexagonal architecture: dependencies point inward
// (infrastructure → application → domain). These tests fail the build if the
// core ever imports infrastructure, a framework or a driver.
func TestDependencyRule(t *testing.T) {
	rules := []struct {
		dir     string
		allowed func(imp string) bool
	}{
		{
			// Domain: standard library only.
			dir:     "domain",
			allowed: isStdlib,
		},
		{
			// Application: standard library and the domain only.
			dir: "application",
			allowed: func(imp string) bool {
				return isStdlib(imp) || imp == module+"/internal/domain"
			},
		},
	}

	for _, rule := range rules {
		for _, pkg := range packagesUnder(t, rule.dir) {
			for _, imp := range pkg.Imports {
				if !rule.allowed(imp) {
					t.Errorf("%s imports %q: violates the dependency rule for internal/%s", pkg.ImportPath, imp, rule.dir)
				}
			}
		}
	}
}

// TestAdaptersDoNotImportEachOther keeps every infrastructure adapter
// independent: HTTP never reaches into MongoDB, MongoDB never into HTTP, and so
// on. Adapters meet only through the application's ports.
func TestAdaptersDoNotImportEachOther(t *testing.T) {
	const infra = module + "/internal/infrastructure/"
	for _, pkg := range packagesUnder(t, "infrastructure") {
		for _, imp := range pkg.Imports {
			if strings.HasPrefix(imp, infra) && !strings.HasPrefix(pkg.ImportPath, imp) {
				t.Errorf("%s imports another adapter %q", pkg.ImportPath, imp)
			}
		}
	}
}

func isStdlib(imp string) bool {
	first, _, _ := strings.Cut(imp, "/")
	return !strings.Contains(first, ".")
}

// packagesUnder returns the non-test Go packages below internal/<dir>.
func packagesUnder(t *testing.T, dir string) []*build.Package {
	t.Helper()
	var pkgs []*build.Package
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || !d.IsDir() {
			return err
		}
		pkg, err := build.ImportDir(path, 0)
		if _, ok := err.(*build.NoGoError); ok {
			return nil
		}
		if err != nil {
			return err
		}
		pkg.ImportPath = module + "/internal/" + filepath.ToSlash(path)
		pkgs = append(pkgs, pkg)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(pkgs) == 0 {
		t.Fatalf("no packages found under internal/%s", dir)
	}
	return pkgs
}
