package genesismesh

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// The runners of the shared suites, by suite: each writes a receipt.
var conformanceRunners = map[string][]string{
	"admin_auth":     {"TestAdminSignatureConformanceVectors"},
	"audit":          {"TestConformanceAuditVectors"},
	"canonical":      {"TestConformanceCanonicalVectors"},
	"consensus":      {"TestConsensusTypesRoundTripPythonProof"},
	"field_registry": {"TestConformanceFieldRegistryVectors"},
	"grants":         {"TestConformanceGrantsVectors"},
	"vouching":       {"TestConformanceVouchingVectors"},
	"interop":        {"TestConformanceInteropVectors"},
}

// Runs the conformance runners again in this test binary with receipts on,
// then checks that the receipts account for every vector of every suite file:
// each kind of a file appears in run or not_run, and the two add up to the
// number of vectors of that kind.
func TestConformanceReceiptsAccountForEveryVector(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, runners := range conformanceRunners {
		names = append(names, runners...)
	}
	sort.Strings(names)
	dir := t.TempDir()
	cmd := exec.Command(exe, "-test.run=^("+strings.Join(names, "|")+")$", "-test.count=1")
	cmd.Env = append(withoutEnv(os.Environ(), receiptsEnv), receiptsEnv+"="+dir)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("the conformance runners failed: %v\n%s", err, out)
	}

	suites, err := filepath.Glob(filepath.Join("testdata", "conformance", "*.json"))
	if err != nil || len(suites) == 0 {
		t.Fatalf("no suite files: %v", err)
	}
	seen := map[string]bool{}
	for _, path := range suites {
		suite := strings.TrimSuffix(filepath.Base(path), ".json")
		runners, ok := conformanceRunners[suite]
		if !ok {
			t.Errorf("%s: no conformance runner is listed for this suite", path)
			continue
		}
		want := suiteKinds(t, path)
		run, notRun := map[string]int{}, map[string]int{}
		for _, runner := range runners {
			name := suite + "." + runner + ".json"
			seen[name] = true
			r := readReceipt(t, filepath.Join(dir, name))
			if r == nil {
				continue
			}
			if r.Schema != "gm.contract.receipt/1" || r.Suite != suite ||
				r.File != "genesismesh/testdata/conformance/"+suite+".json" ||
				!strings.HasPrefix(r.Runner, "genesismesh/") || !strings.HasSuffix(r.Runner, ".go:"+runner) {
				t.Errorf("%s: header %q %q %q %q", name, r.Schema, r.Suite, r.File, r.Runner)
			}
			for k, n := range r.Run {
				run[k] += n
			}
			for k, n := range r.NotRun {
				notRun[k] += n
			}
		}
		got := map[string]int{}
		for k, n := range run {
			got[k] += n
		}
		for k, n := range notRun {
			got[k] += n
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s: run %v + not_run %v, want per kind %v", suite, run, notRun, want)
		}
		if len(run) == 0 {
			t.Errorf("%s: no vector ran", suite)
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if !seen[e.Name()] {
			t.Errorf("unexpected receipt %s", e.Name())
		}
	}
}

func TestNoReceiptWithoutTheVariable(t *testing.T) {
	t.Setenv(receiptsEnv, "")
	if r := newReceipt(t, "canonical", canonicalSuiteFile); r != nil {
		t.Fatalf("receipt started with %s unset", receiptsEnv)
	}
}

// suiteKinds counts the vectors of a suite file per kind ("*" without one).
func suiteKinds(t *testing.T, path string) map[string]int {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var suite struct {
		Vectors []struct {
			Kind string `json:"kind"`
		} `json:"vectors"`
	}
	if err := json.Unmarshal(raw, &suite); err != nil {
		t.Fatal(err)
	}
	out := map[string]int{}
	for _, v := range suite.Vectors {
		out[receiptKind(v.Kind)]++
	}
	return out
}

func readReceipt(t *testing.T, path string) *receipt {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Errorf("missing receipt: %v", err)
		return nil
	}
	var r receipt
	if err := json.Unmarshal(raw, &r); err != nil {
		t.Errorf("%s: %v", path, err)
		return nil
	}
	return &r
}

func withoutEnv(env []string, name string) []string {
	out := env[:0:0]
	for _, kv := range env {
		if !strings.HasPrefix(kv, name+"=") {
			out = append(out, kv)
		}
	}
	return out
}
