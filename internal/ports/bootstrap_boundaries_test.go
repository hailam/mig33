package ports_test

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// Bootstrap supervises arbitrary components. Application construction, transport
// selection, argument parsing and process exit belong outside this package.
func TestBootstrapOnlyOwnsLifecycle(t *testing.T) {
	inspectProductionPackage(t, "../bootstrap", func(file string, source *ast.File) {
		for _, imp := range source.Imports {
			path, _ := strconv.Unquote(imp.Path.Value)
			if path == "flag" || path == "os/signal" || path == "encoding/json" || path == "net" || strings.HasPrefix(path, "net/") || strings.HasPrefix(path, "github.com/hailam/mig33/") {
				t.Errorf("%s imports application, transport or command dependency %s", file, path)
			}
		}
		for _, declaration := range source.Decls {
			switch declaration := declaration.(type) {
			case *ast.FuncDecl:
				if declaration.Name.Name == "Main" || declaration.Name.Name == "Execute" || declaration.Name.Name == "validateMode" {
					t.Errorf("%s declares command dispatcher %s", file, declaration.Name.Name)
				}
			case *ast.GenDecl:
				for _, spec := range declaration.Specs {
					if typ, ok := spec.(*ast.TypeSpec); ok && strings.EqualFold(typ.Name.Name, "Mode") {
						t.Errorf("%s declares application modes", file)
					}
				}
			}
		}
	})
}

func TestSharedCommandHelperHasNoApplicationDispatch(t *testing.T) {
	inspectProductionPackage(t, "../../cmd/internal/command", func(file string, source *ast.File) {
		for _, imp := range source.Imports {
			path, _ := strconv.Unquote(imp.Path.Value)
			if strings.HasPrefix(path, "github.com/hailam/mig33/") && path != "github.com/hailam/mig33/internal/config" {
				t.Errorf("%s depends on application capability %s", file, path)
			}
			if path == "database/sql" {
				t.Errorf("%s depends on a database driver", file)
			}
			if path == "encoding/json" {
				t.Errorf("%s encodes application responses; that belongs to an entrypoint", file)
			}
		}
		ast.Inspect(source, func(node ast.Node) bool {
			// Flag names such as "http" are expected here. Runtime mode/action
			// selectors and named dispatchers are not shared configuration work.
			identifier, ok := node.(*ast.Ident)
			if !ok {
				return true
			}
			switch strings.ToLower(identifier.Name) {
			case "mode", "action", "execute", "dispatch", "validatemode":
				t.Errorf("%s contains command dispatch symbol %s", file, identifier.Name)
			}
			return true
		})
	})
}

func TestExplicitExecutableEntrypoints(t *testing.T) {
	for _, name := range []string{"http", "socket", "omni", "cli"} {
		path := filepath.Join("../../cmd", name, "main.go")
		source, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			t.Fatalf("entrypoint %s: %v", name, err)
		}
		if source.Name.Name != "main" {
			t.Errorf("%s is not an executable", path)
		}
		found := map[string]bool{}
		for _, declaration := range source.Decls {
			if function, ok := declaration.(*ast.FuncDecl); ok && function.Recv == nil {
				found[function.Name.Name] = true
			}
		}
		if !found["main"] || !found["run"] {
			t.Errorf("%s must own main and testable run functions", path)
		}
	}
	if _, err := os.Stat("../../cmd/mig33d"); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("obsolete cmd/mig33d must not remain: %v", err)
	}
}

func inspectProductionPackage(t *testing.T, directory string, inspect func(string, *ast.File)) {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(directory, "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		source, err := parser.ParseFile(token.NewFileSet(), file, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		count++
		inspect(file, source)
	}
	if count == 0 {
		t.Fatalf("missing production package %s", directory)
	}
}
