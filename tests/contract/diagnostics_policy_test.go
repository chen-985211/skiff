package contract_test

import (
	"fmt"
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

func TestDiagnosticsPolicyRecognizesBoundaryViolations(t *testing.T) {
	const fixture = `package sample
import stdlog "log/slog"
import errs "errors"
import format "fmt"
func report(err error) {
	stdlog.Error("bad")
	_ = errs.New("A user-facing error")
	_ = format.Errorf("bare error %d", 2)
	_ = format.Errorf("retain cause: %w", err)
	_ = err.Error()
}
`
	issues, err := diagnosticsViolations("internal/contexts/sample/application/presentation/copy.go", []byte(fixture))
	if err != nil {
		t.Fatal(err)
	}
	for _, rule := range []string{"structured diagnostics", "stable context error code", "localized error message"} {
		if !strings.Contains(strings.Join(issues, "\n"), rule) {
			t.Errorf("missing %q in %v", rule, issues)
		}
	}
	if len(issues) != 4 {
		t.Errorf("got %d violations, want 4: %v", len(issues), issues)
	}
}

func TestDiagnosticsPolicyProduction(t *testing.T) {
	root := i18nRepositoryRoot()
	for _, dir := range []string{"internal", "cmd"} {
		err := filepath.WalkDir(filepath.Join(root, dir), func(path string, entry fs.DirEntry, err error) error {
			if err != nil || entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return err
			}
			relative, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			source, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			issues, err := diagnosticsViolations(filepath.ToSlash(relative), source)
			for _, issue := range issues {
				t.Error(issue)
			}
			return err
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}

func diagnosticsViolations(path string, source []byte) ([]string, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, source, 0)
	if err != nil {
		return nil, err
	}
	layerOwnsErrors := strings.Contains(path, "/contexts/") &&
		(strings.Contains(path, "/domain/") || strings.Contains(path, "/application/"))
	isPresentation := strings.Contains(path, "/presentation/")
	isAdapter := strings.Contains(path, "/contexts/") && strings.Contains(path, "/infrastructure/")
	var issues []string
	add := func(pos token.Pos, rule string) {
		issues = append(issues, fmt.Sprintf("%s: %s", fset.Position(pos), rule))
	}
	imports := map[string]string{}
	for _, spec := range file.Imports {
		dependency, _ := strconv.Unquote(spec.Path.Value)
		name := filepath.Base(dependency)
		if spec.Name != nil {
			name = spec.Name.Name
		}
		imports[name] = dependency
		if (dependency == "log" || dependency == "log/slog") && path != "internal/platform/diagnostics/logger.go" {
			add(spec.Pos(), "use platform structured diagnostics")
		}
	}
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		dependency, name := packageOf(sel, imports), sel.Sel.Name
		switch {
		case dependency == "fmt" && strings.HasPrefix(name, "Print"):
			add(call.Pos(), "use platform structured diagnostics")
		case layerOwnsErrors && dependency == "errors" && name == "New":
			add(call.Pos(), "use a stable context error code")
		case layerOwnsErrors && dependency == "fmt" && name == "Errorf":
			literal, ok := call.Args[0].(*ast.BasicLit)
			if !ok || !wrapsCause(literal.Value) {
				add(call.Pos(), "use a stable context error code or wrap an existing cause")
			}
		case isAdapter && (dependency == "errors" && name == "New" || dependency == "fmt" && name == "Errorf") && len(call.Args) > 0:
			if value, ok := stringLiteral(call.Args[0]); ok && hasChineseCopy(value) {
				add(call.Pos(), "represent adapter failures with a stable context error code")
			}
		case isPresentation && name == "Error" && len(call.Args) == 0:
			add(call.Pos(), "use a localized error message from the presentation error mapping")
		}
		return true
	})
	return issues, nil
}

func TestDiagnosticsPolicyErrorCatalog(t *testing.T) {
	root := filepath.Join(i18nRepositoryRoot(), "internal", "contexts", "transfer")
	parse := func(path string) *ast.File {
		t.Helper()
		file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		return file
	}
	names, codes := map[string]bool{}, map[string]string{}
	codePattern := regexp.MustCompile(`^[A-Z][A-Z0-9]*(_[A-Z0-9]+)+$`)
	for _, layer := range []string{"domain", "application"} {
		file := parse(filepath.Join(root, layer, "errors.go"))
		ast.Inspect(file, func(node ast.Node) bool {
			spec, ok := node.(*ast.ValueSpec)
			if !ok {
				return true
			}
			for i, name := range spec.Names {
				if !strings.HasPrefix(name.Name, "Err") || i >= len(spec.Values) {
					continue
				}
				value, ok := spec.Values[i].(*ast.BasicLit)
				if !ok {
					continue // Application aliases preserve the underlying domain code.
				}
				code, _ := strconv.Unquote(value.Value)
				if !codePattern.MatchString(code) {
					t.Errorf("%s has invalid code %q", name.Name, code)
				}
				if first, duplicate := codes[code]; duplicate {
					t.Errorf("%s and %s share code %q", name.Name, first, code)
				}
				codes[code] = name.Name
				names[name.Name] = true
			}
			return true
		})
	}

	presentation := filepath.Join(root, "presentation")
	base := readMessages(t, filepath.Join(presentation, "i18n", "catalogs", "zh-CN.json"))
	file := parse(filepath.Join(presentation, "error_messages.go"))
	mapped := map[string]bool{}
	ast.Inspect(file, func(node ast.Node) bool {
		pair, ok := node.(*ast.KeyValueExpr)
		if !ok {
			return true
		}
		name, ok := pair.Key.(*ast.SelectorExpr)
		if !ok || !strings.HasPrefix(name.Sel.Name, "Err") {
			return true
		}
		value, ok := pair.Value.(*ast.BasicLit)
		if !ok {
			t.Errorf("%s needs a static message key", name.Sel.Name)
			return true
		}
		key, _ := strconv.Unquote(value.Value)
		if _, ok := base[key]; !ok {
			t.Errorf("%s maps to missing key %s", name.Sel.Name, key)
		}
		mapped[name.Sel.Name] = true
		return true
	})
	for name := range names {
		if !mapped[name] {
			t.Errorf("%s is missing a user-facing message", name)
		}
	}
}

func wrapsCause(format string) bool {
	for _, verb := range formatVerb.FindAllString(format, -1) {
		if strings.HasSuffix(verb, "w") {
			return true
		}
	}
	return false
}

func TestDiagnosticsPolicyRecognizesAdapterCopy(t *testing.T) {
	const source = `package adapter
import "fmt"
func fail() error { return fmt.Errorf("无法读取文件") }
func httpStatus(n int) error { return fmt.Errorf("HTTP %d", n) }
`
	issues, err := diagnosticsViolations("internal/contexts/transfer/infrastructure/adapter.go", []byte(source))
	if err != nil || len(issues) != 1 || !strings.Contains(issues[0], "stable context error code") {
		t.Fatalf("adapter diagnostics = %v (%v)", issues, err)
	}
}
