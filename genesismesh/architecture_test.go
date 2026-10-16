package genesismesh

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"path"
	"path/filepath"
	"strings"
	"testing"
)

// The package's layers and the direction they may depend in. docs/architecture.md
// shows the same table; change both together.
//
// The offline verifier (json, registry, verifiers) must work without the HTTP
// client: none of its files may use anything declared in the client half
// (protocol, transport, sub-clients, entry). A file may use what its own layer
// declares and what the layers in its "may use" list declare, nothing else.
var architectureLayers = []struct {
	name    string
	files   []string
	mayUse  []string
	purpose string
}{
	{"json", []string{"strict_json.go", "canonical.go"}, nil,
		"strict JSON and the canonical form"},
	{"registry", []string{"strict.go"}, []string{"json"},
		"the embedded field registry and the strict checks"},
	{"verifiers", []string{"verify.go", "key_history.go", "grants.go"}, []string{"json", "registry"},
		"offline verification of NA-signed records"},
	{"signing", []string{"auth.go"}, []string{"json"},
		"Ed25519 keys and admin request headers"},
	{"protocol", []string{"types.go", "errors.go"}, nil,
		"wire structs and typed API errors"},
	{"transport", []string{"transport.go"}, []string{"json", "signing", "protocol"},
		"HTTP, strict response decoding, error mapping"},
	{"sub-clients", []string{"agreement.go", "attestation.go", "boundary.go", "consensus.go",
		"data_usage.go", "disclosure.go", "evidence.go"}, []string{"protocol", "transport"},
		"one thin wrapper per NA route domain"},
	{"entry", []string{"client.go"}, []string{"transport", "sub-clients"},
		"Client and NewClient"},
}

// Today's uses that break the table, as "file -> file: identifier". The list
// only shrinks: a new violation fails, and so does an entry that no longer
// occurs (remove it here when the code stops needing it).
var architectureAllowlist = map[string]string{
	"strict.go -> verify.go: object": "the record type alias lives with the verifiers; move it down to strict.go",
}

// The offline layers make no network calls: their files import no net package.
var offlineLayers = map[string]bool{"json": true, "registry": true, "verifiers": true, "signing": true}

// TestArchitectureLayers type-checks the package's source files and fails on
// any use of a package-level declaration, method or field across files that
// the layer table does not allow.
func TestArchitectureLayers(t *testing.T) {
	layerOf := map[string]string{}
	allowed := map[[2]string]bool{}
	for _, l := range architectureLayers {
		allowed[[2]string{l.name, l.name}] = true
		for _, u := range l.mayUse {
			allowed[[2]string{l.name, u}] = true
		}
		for _, f := range l.files {
			layerOf[f] = l.name
		}
	}

	edges := packageEdges(t)

	// Every source file has a layer, so a new file is placed deliberately.
	paths, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range paths {
		if !strings.HasSuffix(p, "_test.go") && layerOf[p] == "" {
			t.Errorf("%s has no layer: add it to architectureLayers and docs/architecture.md", p)
		}
	}
	for f := range layerOf {
		if _, ok := edges.files[f]; !ok {
			t.Errorf("architectureLayers lists %s, which is not a source file of the package", f)
		}
	}

	for f, imports := range edges.imports {
		if !offlineLayers[layerOf[f]] {
			continue
		}
		for _, imp := range imports {
			if imp == "net" || strings.HasPrefix(imp, "net/") {
				t.Errorf("%s (layer %s) imports %s: the offline verifier makes no network calls", f, layerOf[f], imp)
			}
		}
	}

	// The check is not vacuous: known edges inside the client half are found.
	for _, e := range []string{"boundary.go -> transport.go: adminPost", "client.go -> transport.go: newTransport",
		"verify.go -> strict.go: hasUnknownFields"} {
		if !edges.uses[e] {
			t.Errorf("the dependency walk no longer finds %q: the architecture test is broken", e)
		}
	}

	violations := map[string]bool{}
	for _, use := range sortedKeys(edges.uses) {
		from, rest, _ := strings.Cut(use, " -> ")
		to, _, _ := strings.Cut(rest, ":")
		if allowed[[2]string{layerOf[from], layerOf[to]}] {
			continue
		}
		violations[use] = true
		if _, ok := architectureAllowlist[use]; !ok {
			t.Errorf("layer violation %q: a %s file may not use a %s declaration (see docs/architecture.md)",
				use, layerOf[from], layerOf[to])
		}
	}
	allowlisted := map[string]bool{}
	for a := range architectureAllowlist {
		allowlisted[a] = true
	}
	for _, a := range sortedKeys(allowlisted) {
		if !violations[a] {
			t.Errorf("allowlisted violation %q no longer occurs: remove it from architectureAllowlist", a)
		}
	}
}

type dependencyEdges struct {
	files map[string]bool
	// The import paths of each file.
	imports map[string][]string
	// "from.go -> to.go: name" for every use in from.go of an object declared in to.go.
	uses map[string]bool
}

// packageEdges type-checks the non-test files with every import stubbed out:
// only the package's own objects matter, so errors about imported names are
// expected and ignored.
func packageEdges(t *testing.T) dependencyEdges {
	t.Helper()
	fset := token.NewFileSet()
	paths, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	var files []*ast.File
	edges := dependencyEdges{files: map[string]bool{}, imports: map[string][]string{}, uses: map[string]bool{}}
	for _, p := range paths {
		if strings.HasSuffix(p, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, p, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, f)
		edges.files[p] = true
		for _, imp := range f.Imports {
			edges.imports[p] = append(edges.imports[p], strings.Trim(imp.Path.Value, `"`))
		}
	}
	info := &types.Info{Uses: map[*ast.Ident]types.Object{}}
	conf := types.Config{Importer: stubImporter{}, Error: func(error) {}}
	pkg, _ := conf.Check("github.com/GenesisMeshLabs/sdk-go/genesismesh", fset, files, info)
	for id, obj := range info.Uses {
		if obj == nil || obj.Pkg() != pkg || !obj.Pos().IsValid() {
			continue
		}
		from := filepath.Base(fset.Position(id.Pos()).Filename)
		to := filepath.Base(fset.Position(obj.Pos()).Filename)
		if from != to {
			edges.uses[from+" -> "+to+": "+obj.Name()] = true
		}
	}
	return edges
}

// stubImporter returns an empty package for every import.
type stubImporter struct{}

func (stubImporter) Import(p string) (*types.Package, error) {
	pkg := types.NewPackage(p, path.Base(p))
	pkg.MarkComplete()
	return pkg, nil
}
