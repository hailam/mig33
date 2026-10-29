package ports_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

var businessPackages = []string{
	"accounts", "contacts", "chat", "rooms", "community", "commerce", "catalog",
	"media", "review", "activity", "realtime", "session", "commands", "events", "faults",
}

// Capability models, rules and live coordination must remain independent of
// transports, presentation, concrete persistence and process composition.
func TestApplicationDependencyDirection(t *testing.T) {
	for _, name := range businessPackages {
		inspectProductionPackage(t, "../"+name, func(file string, source *ast.File) {
			for _, imp := range source.Imports {
				path, _ := strconv.Unquote(imp.Path.Value)
				for _, forbidden := range []string{
					"database/sql", "/internal/chatstore", "/internal/adapters/", "/internal/bootstrap", "/internal/config",
					"/internal/protocol", "/internal/pages", "/internal/controllers", "/internal/httpapi",
					"/internal/domain", "/internal/services", "github.com/a-h/templ", "github.com/mattn/go-sqlite3",
				} {
					if strings.Contains(path, forbidden) {
						t.Errorf("%s imports outer dependency %s", file, path)
					}
				}
				if path == "net" || path == "net/http" || strings.HasPrefix(path, "net/http/") {
					t.Errorf("%s imports transport %s", file, path)
				}
			}
		})
	}
}

// Temporary migration aliases and the all-feature constructor must not become
// permanent dependencies hidden behind a renamed package.
func TestNoLegacyApplicationOrAggregatePorts(t *testing.T) {
	for _, directory := range []string{"../services", "../domain"} {
		files, err := filepath.Glob(filepath.Join(directory, "*.go"))
		if err != nil {
			t.Fatal(err)
		}
		for _, file := range files {
			if !strings.HasSuffix(file, "_test.go") {
				t.Errorf("obsolete application package remains: %s", file)
			}
		}
	}
	err := filepath.WalkDir("..", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		source, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			return err
		}
		portNames := map[string]bool{}
		for _, imp := range source.Imports {
			imported, _ := strconv.Unquote(imp.Path.Value)
			if strings.HasSuffix(imported, "/internal/domain") || strings.HasSuffix(imported, "/internal/services") {
				t.Errorf("%s depends on obsolete application package %s", path, imported)
			}
			if strings.HasSuffix(imported, "/internal/ports") {
				alias := "ports"
				if imp.Name != nil {
					alias = imp.Name.Name
				}
				portNames[alias] = true
			}
		}
		ast.Inspect(source, func(node ast.Node) bool {
			switch node := node.(type) {
			case *ast.SelectorExpr:
				pkg, ok := node.X.(*ast.Ident)
				if ok && portNames[pkg.Name] && (node.Sel.Name == "Repositories" || node.Sel.Name == "Services") {
					t.Errorf("%s depends on all-feature port %s.%s", path, pkg.Name, node.Sel.Name)
				}
			case *ast.TypeSpec:
				if filepath.Dir(path) == "../ports" && (node.Name.Name == "Repositories" || node.Name.Name == "Services") {
					t.Errorf("%s declares obsolete all-feature port %s", path, node.Name.Name)
				}
			}
			return true
		})
		return nil
	})
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
}

// Feature repositories own transactions and domain mappings, but SQL lives in
// query/migration files. Keep handwritten driver calls out of those features.
func TestRepositorySQLUsesGeneratedMethods(t *testing.T) {
	files, err := filepath.Glob("../adapters/sqlite/repositories/*.go")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("missing SQLite repository adapter")
	}
	sqlLiteral := regexp.MustCompile(`(?i)^\s*(SELECT\s|INSERT\s+INTO\s|UPDATE\s+\w+\s+SET\s|DELETE\s+FROM\s|CREATE\s+(TABLE|INDEX|TRIGGER)\s|ALTER\s+TABLE\s|DROP\s+TABLE\s|PRAGMA\s)`)
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		positions := token.NewFileSet()
		source, err := parser.ParseFile(positions, file, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(source, func(node ast.Node) bool {
			if literal, ok := node.(*ast.BasicLit); ok && literal.Kind == token.STRING {
				value, _ := strconv.Unquote(literal.Value)
				if sqlLiteral.MatchString(value) {
					t.Errorf("%s: SQL belongs in db/queries or db/migrations", positions.Position(literal.Pos()))
				}
			}
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			switch selector.Sel.Name {
			case "Exec", "ExecContext", "Query", "QueryContext", "QueryRow", "QueryRowContext", "Prepare", "PrepareContext":
				t.Errorf("%s: feature repository bypasses generated queries with %s", positions.Position(call.Pos()), selector.Sel.Name)
			}
			return true
		})
	}
}

func TestControllersDependOnServicePorts(t *testing.T) {
	files, err := filepath.Glob("../controllers/*.go")
	if err != nil {
		t.Fatal(err)
	}
	if len(files) == 0 {
		t.Fatal("missing controllers")
	}
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		source, err := parser.ParseFile(token.NewFileSet(), file, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, imp := range source.Imports {
			path, _ := strconv.Unquote(imp.Path.Value)
			for _, forbidden := range []string{"database/sql", "/internal/adapters/sqlite", "/internal/services", "/db/migrations"} {
				if strings.Contains(path, forbidden) {
					t.Errorf("controller %s bypasses service ports: %s", file, path)
				}
			}
		}
		ast.Inspect(source, func(node ast.Node) bool {
			if s, ok := node.(*ast.SelectorExpr); ok && (s.Sel.Name == "Repositories" || strings.HasSuffix(s.Sel.Name, "Repository")) {
				t.Errorf("controller %s refers to repository capability %s", file, s.Sel.Name)
			}
			return true
		})
	}
}

// Repository capabilities describe request operations, not resource ownership.
// Composition closes adapters; every feature operation receives its caller's
// cancellation and deadline rather than inventing a detached context.
func TestRepositoryPortsRequireOperationContext(t *testing.T) {
	total := 0
	for _, name := range []string{"accounts", "contacts", "chat", "rooms", "community", "commerce", "media", "review", "activity"} {
		path := "../" + name + "/repository.go"
		source, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		methods := 0
		ast.Inspect(source, func(node ast.Node) bool {
			contract, ok := node.(*ast.InterfaceType)
			if !ok {
				return true
			}
			for _, field := range contract.Methods.List {
				method, ok := field.Type.(*ast.FuncType)
				if !ok {
					continue
				}
				methods++
				methodName := field.Names[0].Name
				if methodName == "Close" {
					t.Errorf("%s: repository feature ports must not own adapter Close", path)
				}
				if method.Params.NumFields() == 0 || !isContextType(method.Params.List[0].Type) {
					t.Errorf("%s: operation %s must accept context.Context first", path, methodName)
				}
				ast.Inspect(method, func(node ast.Node) bool {
					if selector, ok := node.(*ast.SelectorExpr); ok {
						if pkg, ok := selector.X.(*ast.Ident); ok && (pkg.Name == "sql" || pkg.Name == "sqlc") {
							t.Errorf("%s exposes infrastructure in %s", path, methodName)
						}
					}
					return true
				})
			}
			return false
		})
		if methods == 0 {
			t.Errorf("no repository operations inspected in %s", path)
		}
		total += methods
	}
	if total < 124 {
		t.Errorf("inspected only %d repository operations; expected all 124 established operations", total)
	}
}

func TestApplicationAndRepositoriesDoNotDetachOperationContexts(t *testing.T) {
	directories := []string{"../adapters/sqlite/repositories", "../adapters/sqlite/migrations"}
	for _, name := range businessPackages {
		directories = append(directories, "../"+name)
	}
	for _, directory := range directories {
		inspectProductionPackage(t, directory, func(file string, source *ast.File) {
			ast.Inspect(source, func(node ast.Node) bool {
				switch node := node.(type) {
				case *ast.CallExpr:
					selector, ok := node.Fun.(*ast.SelectorExpr)
					if !ok {
						break
					}
					pkg, ok := selector.X.(*ast.Ident)
					if ok && pkg.Name == "context" && (selector.Sel.Name == "Background" || selector.Sel.Name == "TODO" || selector.Sel.Name == "WithoutCancel") {
						t.Errorf("%s detaches an operation with context.%s; accept the owner's context", file, selector.Sel.Name)
					}
				case *ast.StructType:
					for _, field := range node.Fields.List {
						if isContextType(field.Type) {
							t.Errorf("%s stores context.Context; pass it to the operation", file)
						}
					}
				}
				return true
			})
		})
	}
}

func isContextType(expression ast.Expr) bool {
	selector, ok := expression.(*ast.SelectorExpr)
	if !ok || selector.Sel.Name != "Context" {
		return false
	}
	pkg, ok := selector.X.(*ast.Ident)
	return ok && pkg.Name == "context"
}
