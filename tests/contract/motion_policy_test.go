package contract_test

import (
	"go/ast"
	"go/token"
	"testing"
)

func motionField(lit *ast.CompositeLit, key string) ast.Expr {
	for _, elem := range lit.Elts {
		kv, ok := elem.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		field, ok := kv.Key.(*ast.Ident)
		if ok && field.Name == key {
			if id, ok := kv.Value.(*ast.Ident); ok && id.Name == "nil" {
				return nil
			}
			return kv.Value
		}
	}
	return nil
}

func checkMotionDefinition(kind string, lit *ast.CompositeLit, report func(token.Pos, string)) {
	switch kind {
	case "Animation":
		if motionField(lit, "Spring") == nil && (motionField(lit, "Duration") == nil || motionField(lit, "Ease") == nil) {
			report(lit.Pos(), "define an explicit spring or a duration and easing curve")
		}
	case "SpringOptions":
		if motionField(lit, "Stiffness") == nil || motionField(lit, "Damping") == nil {
			report(lit.Pos(), "specify spring stiffness and damping")
		}
	case "ElementTransition", "ContentTransition":
		if motionField(lit, "Animation") == nil {
			report(lit.Pos(), "connect the transition to a component curve")
		}
	}
}

// Follow only the receiver chain. A transition on a sibling or inside a
// Children closure cannot animate the control being constructed.
func checkControlMotion(call *ast.CallExpr, parents []ast.Node, uiName string, report func(token.Pos, string)) {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return
	}
	pkg, ok := sel.X.(*ast.Ident)
	if !ok || pkg.Name != uiName {
		return
	}
	control := false
	switch sel.Sel.Name {
	case "ButtonBase", "TextInputBase", "TextAreaBase", "SwitchBase":
		control = true
	case "Row", "Column", "Box", "Text", "Textf":
	default:
		return
	}
	var receiver ast.Expr = call
	i := len(parents) - 1
	hasChildren := false
	for i >= 1 {
		method, ok := parents[i].(*ast.SelectorExpr)
		if !ok || method.X != receiver {
			break
		}
		chained, ok := parents[i-1].(*ast.CallExpr)
		if !ok || chained.Fun != method {
			break
		}
		if method.Sel.Name == "Transition" && len(chained.Args) == 1 {
			return
		}
		if method.Sel.Name == "Children" {
			hasChildren = true
		}
		receiver = chained
		i -= 2
	}
	if control {
		report(call.Pos(), sel.Sel.Name+" must attach a transition from motion.go on its construction chain")
		return
	}
	if !hasChildren && sel.Sel.Name != "Text" && sel.Sel.Name != "Textf" {
		return
	}
	for ; i >= 0; i-- {
		switch parents[i].(type) {
		case *ast.FuncLit, *ast.FuncDecl:
			return
		case *ast.IfStmt, *ast.CaseClause:
			report(call.Pos(), "conditional content needs an enter/exit transition or a permanently mounted animated container")
			return
		}
	}
}

func TestInterfacePolicyRequiresCurvesAtTheControl(t *testing.T) {
	for _, tc := range []struct {
		body string
		bad  bool
	}{
		{"kit.ButtonBase(c).Label(name)", true},
		{"kit.TextInputBase(c, value).Width(80)", true},
		{"kit.ButtonBase(c).Label(name); other.Transition(controlMotion)", true},
		{"kit.ButtonBase(c).Children(func(){ child.Transition(controlMotion) })", true},
		{"kit.ButtonBase(c).Label(name).Transition(controlMotion)", false},
		{"if open {kit.Column(c).FillWidth().Children(render)}", true},
		{"if open {kit.Column(c).Transition(panelMotion).Children(render)}", false},
		{"kit.Text(c, title).FontSize(14)", false},
		{"if failed {kit.Text(c, reason)}", true},
		{"if expanded {kit.Box(c).Children(render)}", true},
		{"if failed {kit.Text(c, reason).Transition(pageFade)}", false},
		{"var blank kit.ElementTransition; kit.ButtonBase(c).Transition(blank)", true},
	} {
		source := "package sample\nimport kit \"github.com/egoist/mygo/ui\"\nfunc draw(){" + tc.body + "}"
		found, err := interfaceFindings("view.go", source)
		if err != nil || (len(found) > 0) != tc.bad {
			t.Errorf("%s: %v, %v", tc.body, found, err)
		}
	}
}

func TestInterfacePolicyMotionDefinitionsNeedACurve(t *testing.T) {
	for _, test := range []struct {
		declaration string
		bad         bool
	}{
		{"kit.Animation{Duration: time.Second}", true},
		{"kit.Animation{Duration: time.Second, Ease: nil}", true},
		{"kit.Animation{Spring: kit.SpringOptions{}}", true},
		{"kit.Animation{Duration: time.Second, Ease: kit.EaseOut}", false},
		{"kit.ElementTransition{Colors:true}", true},
		{"kit.ContentTransition{Numeric:true}", true},
		{"kit.ElementTransition{Colors:true, Animation:&pressSpring}", false},
	} {
		src := `package sample
import kit "github.com/egoist/mygo/ui"
var motion = ` + test.declaration
		found, err := interfaceFindings(motionOwner, src)
		if err != nil || (len(found) > 0) != test.bad {
			t.Errorf("%s: %v, %v", test.declaration, found, err)
		}
	}
}
