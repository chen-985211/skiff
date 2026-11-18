package contract_test

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
)

const paletteOwner = "internal/contexts/transfer/presentation/theme.go"
const motionOwner = "internal/contexts/transfer/presentation/motion.go"

// Inspect syntax so comments, artwork strings and local functions named Hex
// are not mistaken for MyGo styling. The report points to the offending line.
func interfaceFindings(path string, src any) ([]string, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, src, 0)
	if err != nil {
		return nil, err
	}
	var findings []string
	report := func(pos token.Pos, message string) {
		findings = append(findings, fmt.Sprintf("%s:%d: %s", path, fset.Position(pos).Line, message))
	}
	var uiName string
	for _, imp := range file.Imports {
		name, _ := strconv.Unquote(imp.Path.Value)
		if name != "github.com/egoist/mygo/ui" {
			continue
		}
		uiName = "ui"
		if imp.Name != nil {
			uiName = imp.Name.Name
		}
		if uiName == "." {
			report(imp.Pos(), "use a named MyGo UI import so its design rules can be checked")
		}
	}
	if uiName == "" {
		return findings, nil
	}
	ast.Inspect(file, func(n ast.Node) bool {
		var expr ast.Expr
		switch node := n.(type) {
		case *ast.CallExpr:
			expr = node.Fun
		case *ast.CompositeLit:
			expr = node.Type
		default:
			return true
		}
		sel, ok := expr.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		owner, named := sel.X.(*ast.Ident)
		if named && owner.Name == uiName {
			switch sel.Sel.Name {
			case "RGB", "RGBA", "Hex", "Color":
				if path != paletteOwner {
					report(sel.Pos(), "use a semantic color from theme.go")
				}
			case "SpringOptions", "Animation", "ElementTransition", "ContentTransition":
				if path != motionOwner {
					report(sel.Pos(), "select a component curve from motion.go")
				}
			case "Switch", "Button":
				report(sel.Pos(), "use the themed control built on SwitchBase or ButtonBase")
			}
		} else if path != motionOwner && (sel.Sel.Name == "Animate" || sel.Sel.Name == "AnimateWith" || sel.Sel.Name == "Spring") {
			report(sel.Sel.Pos(), "use AnimateTo with the component curve from motion.go")
		}
		return true
	})
	return findings, nil
}

func TestInterfacePolicy(t *testing.T) {
	_, source, _, _ := runtime.Caller(0)
	root := filepath.Clean(filepath.Join(filepath.Dir(source), "../.."))
	err := filepath.WalkDir(filepath.Join(root, "internal/contexts"), func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if d.IsDir() || !strings.Contains(rel, "/presentation/") || !strings.HasSuffix(rel, ".go") || strings.HasSuffix(rel, "_test.go") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		findings, err := interfaceFindings(rel, data)
		if err != nil {
			return err
		}
		for _, finding := range findings {
			t.Error(finding)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestInterfacePolicyRecognizesAliasesAndExceptions(t *testing.T) {
	source := "package sample\nimport kit \"github.com/egoist/mygo/ui\"\nfunc draw(){\n_ = kit.Hex(\"#fff\"); _ = kit.SpringOptions{}; _ = kit.Switch(c,on); node.Animate(1,2,3); node.AnimateTo(1,2,sheetSpring); _ = kit.ButtonBase(c)\n}"
	for path, want := range map[string]int{"view.go": 4, paletteOwner: 3, motionOwner: 2} {
		found, err := interfaceFindings(path, source)
		if err != nil || len(found) != want {
			t.Fatalf("%s: %v, %v", path, found, err)
		}
	}
}
