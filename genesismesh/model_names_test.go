package genesismesh

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// v1.7.1: a model name the embedded registry does not define fails closed at
// run time (strict.go). This test makes such a name fail CI first: every model
// name the source passes to a function taking a `model string` first, or
// looks up in the registry, must be a model of the registry. A core rename or
// a typo ("BoundaryDecison") is caught here, not by a verifier that refuses
// every record.
func TestEveryModelNameInTheSourceIsInTheRegistry(t *testing.T) {
	fset := token.NewFileSet()
	paths, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	var files []*ast.File
	for _, p := range paths {
		if strings.HasSuffix(p, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, p, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, f)
	}

	// Package-level string constants, which the parser resolves only within their own file.
	consts := map[string]string{}
	// The functions whose first parameter is `model string`.
	takesModel := map[string]bool{}
	for _, f := range files {
		for _, decl := range f.Decls {
			switch d := decl.(type) {
			case *ast.GenDecl:
				if d.Tok != token.CONST {
					continue
				}
				for _, spec := range d.Specs {
					vs := spec.(*ast.ValueSpec)
					for i, name := range vs.Names {
						if i < len(vs.Values) {
							if s, ok := stringLiteral(vs.Values[i]); ok {
								consts[name.Name] = s
							}
						}
					}
				}
			case *ast.FuncDecl:
				if firstParamIsModel(d.Type) {
					takesModel[d.Name.Name] = true
				}
			}
		}
	}
	for _, name := range []string{"UnknownFields", "unknownFieldsIn", "hasUnknownFields", "nonCanonicalFields",
		"nonCanonicalTimestamps", "receivedCanonical", "signedAsReceived", "strictRefusal"} {
		if !takesModel[name] {
			t.Errorf("%s no longer takes `model string` first: update this test so it still checks its callers", name)
		}
	}

	checked := map[string]bool{}
	check := func(f *ast.File, e ast.Expr, where string) {
		pos := fset.Position(e.Pos())
		model, ok := modelName(e, consts)
		if !ok {
			if registryDerived(f, e, fset) {
				return
			}
			t.Errorf("%s:%d: the model passed to %s is not a string literal or constant, so this test cannot check it", pos.Filename, pos.Line, where)
			return
		}
		if _, known := canonicalRegistry.Models[model]; !known {
			t.Errorf("%s:%d: %s: model %q is not in the embedded registry", pos.Filename, pos.Line, where, model)
		}
		checked[model] = true
	}
	for _, f := range files {
		ast.Inspect(f, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.CallExpr:
				if id, ok := x.Fun.(*ast.Ident); ok && takesModel[id.Name] && len(x.Args) > 0 {
					check(f, x.Args[0], id.Name)
				}
			case *ast.IndexExpr:
				// canonicalRegistry.Models[model] and NARecordTimeFields[model].
				if isRegistryModels(x.X) || isIdent(x.X, "NARecordTimeFields") {
					check(f, x.Index, "a registry lookup")
				}
			case *ast.CompositeLit:
				// The keys of a table by model name, such as NARecordTimeFields.
				if mt, ok := x.Type.(*ast.MapType); ok && isIdent(mt.Key, "string") {
					for _, elt := range x.Elts {
						if kv, ok := elt.(*ast.KeyValueExpr); ok {
							if s, ok := stringLiteral(kv.Key); ok && looksLikeModel(s) {
								check(f, kv.Key, "a table by model")
							}
						}
					}
				}
			}
			return true
		})
	}
	t.Logf("checked %d model names", len(checked))
	// The test must not pass by checking nothing.
	if len(checked) < 10 {
		names := make([]string, 0, len(checked))
		for m := range checked {
			names = append(names, m)
		}
		sort.Strings(names)
		t.Fatalf("checked only %d model names (%v): the scan no longer finds the verifiers' models", len(checked), names)
	}
}

// Every model the registry names as nested (object, list or map of a model)
// is itself in the registry, so the strict checks never meet one they lack.
func TestEveryNestedModelIsInTheRegistry(t *testing.T) {
	for name, spec := range canonicalRegistry.Models {
		for field, kind := range spec.Fields {
			nested, ok := kind.(map[string]interface{})
			if !ok {
				continue
			}
			for _, k := range []string{"object", "list", "map"} {
				if m, ok := nested[k].(string); ok {
					if _, known := canonicalRegistry.Models[m]; !known {
						t.Errorf("%s.%s names model %q, which the registry lacks", name, field, m)
					}
				}
			}
		}
	}
}

func firstParamIsModel(ft *ast.FuncType) bool {
	if ft.Params == nil || len(ft.Params.List) == 0 {
		return false
	}
	p := ft.Params.List[0]
	return len(p.Names) > 0 && p.Names[0].Name == "model" && isIdent(p.Type, "string")
}

func modelName(e ast.Expr, consts map[string]string) (string, bool) {
	if s, ok := stringLiteral(e); ok {
		return s, true
	}
	id, ok := e.(*ast.Ident)
	if !ok {
		return "", false
	}
	if id.Obj != nil {
		if vs, ok := id.Obj.Decl.(*ast.ValueSpec); ok && id.Obj.Kind == ast.Con {
			for i, n := range vs.Names {
				if n.Name == id.Name && i < len(vs.Values) {
					return stringLiteral(vs.Values[i])
				}
			}
		}
		return "", false
	}
	s, ok := consts[id.Name]
	return s, ok
}

// registryDerived reports whether e is a parameter (its callers are checked)
// or, in strict.go, a model name read from the registry itself (checked by
// TestEveryNestedModelIsInTheRegistry).
func registryDerived(f *ast.File, e ast.Expr, fset *token.FileSet) bool {
	id, ok := e.(*ast.Ident)
	if !ok || id.Obj == nil {
		return false
	}
	switch id.Obj.Decl.(type) {
	case *ast.Field:
		return true
	case *ast.AssignStmt:
		return filepath.Base(fset.Position(f.Pos()).Filename) == "strict.go"
	}
	return false
}

func stringLiteral(e ast.Expr) (string, bool) {
	lit, ok := e.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return "", false
	}
	s, err := strconv.Unquote(lit.Value)
	return s, err == nil
}

func isIdent(e ast.Expr, name string) bool {
	id, ok := e.(*ast.Ident)
	return ok && id.Name == name
}

func isRegistryModels(e ast.Expr) bool {
	sel, ok := e.(*ast.SelectorExpr)
	return ok && sel.Sel.Name == "Models" && isIdent(sel.X, "canonicalRegistry")
}

// looksLikeModel reports whether a map key reads as a model name (UpperCamelCase).
func looksLikeModel(s string) bool {
	if s == "" || s[0] < 'A' || s[0] > 'Z' {
		return false
	}
	for _, r := range s {
		if !(r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9') {
			return false
		}
	}
	return true
}
