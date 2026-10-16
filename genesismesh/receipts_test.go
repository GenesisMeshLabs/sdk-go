package genesismesh

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// Conformance receipts (schema gm.contract.receipt/1). When
// GM_CONFORMANCE_RECEIPTS names a directory, every runner of a shared suite
// writes <dir>/<suite>.<runner>.json once its vectors ran: the vectors it ran
// and the ones it deliberately did not, counted per kind. The contract tool
// checks them against the pinned bundle, so a vector no runner selected is
// noticed. With the variable unset, newReceipt returns nil and nothing is
// recorded or written.

const receiptsEnv = "GM_CONFORMANCE_RECEIPTS"

type receipt struct {
	Schema string         `json:"schema"`
	Suite  string         `json:"suite"`
	File   string         `json:"file"`
	Runner string         `json:"runner"`
	Run    map[string]int `json:"run"`
	NotRun map[string]int `json:"not_run"`
}

// newReceipt starts the receipt of the calling runner for suite, read from
// file (relative to this package, as passed to os.ReadFile). It is written
// when the test ends, unless the test failed.
func newReceipt(t *testing.T, suite, file string) *receipt {
	t.Helper()
	dir := os.Getenv(receiptsEnv)
	if dir == "" {
		return nil
	}
	_, source, _, _ := runtime.Caller(1)
	r := &receipt{
		Schema: "gm.contract.receipt/1",
		Suite:  suite,
		File:   "genesismesh/" + filepath.ToSlash(file),
		Runner: "genesismesh/" + filepath.Base(source) + ":" + t.Name(),
		Run:    map[string]int{},
		NotRun: map[string]int{},
	}
	t.Cleanup(func() {
		if t.Failed() {
			return
		}
		raw, err := json.MarshalIndent(r, "", "  ")
		if err == nil {
			err = os.MkdirAll(dir, 0o755)
		}
		if err == nil {
			err = os.WriteFile(filepath.Join(dir, suite+"."+t.Name()+".json"), append(raw, '\n'), 0o644)
		}
		if err != nil {
			t.Errorf("conformance receipt: %v", err)
		}
	})
	return r
}

// receiptKind is the kind a vector counts under: "*" for one without a kind.
func receiptKind(kind string) string {
	if kind == "" {
		return "*"
	}
	return kind
}

// ran records a vector of kind that this runner checked.
func (r *receipt) ran(kind string) {
	if r != nil {
		r.Run[receiptKind(kind)]++
	}
}

// notRun records a vector of kind that this runner deliberately skips.
func (r *receipt) notRun(kind string) {
	if r != nil {
		r.NotRun[receiptKind(kind)]++
	}
}
