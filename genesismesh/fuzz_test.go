package genesismesh

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"
	"unicode/utf16"
	"unicode/utf8"
)

// Fuzz targets for the parsers every verification starts from. `go test` runs
// each on its seed corpus (the conformance inputs below); CI also fuzzes each
// for a few seconds (go test -fuzz). A failing input is written to
// testdata/fuzz/<Target>/ by the fuzzer: commit it with the fix, and it stays
// a regression test.

var strictReasons = map[string]bool{
	"invalid_json": true, "duplicate_key": true, "non_finite_number": true,
	"integer_out_of_range": true, "negative_zero": true, "lone_surrogate": true,
}

// conformanceSeeds returns the JSON text inputs of the canonical suite and
// numbers, escapes and nesting the suite does not spell out.
func conformanceSeeds(tb testing.TB) []string {
	tb.Helper()
	seeds := []string{
		`0`, `-1`, `18446744073709551615`, `18446744073709551616`, `-9223372036854775808`,
		`-9223372036854775809`, `-0`, `-0.0`, `0e0`, `1E400`, `1e-400`, `1.5e-7`, `100000000000000000000.0`,
		`0.1`, `5e-324`, `1.7976931348623157e308`, `[1,1.0,1e2,1E+2,-1.25e-2]`,
		`"\u00e9\u20ac\ud83d\ude00"`, `"\ud800"`, `"\udc00x"`, `"\/\b\f\n\r\t\"\\"`, `"é€😀"`,
		"\"\x7f\"", `"\u007f\u001f"`,
		`{"a":1,"a":2}`, `{"a":1,"\u0061":2}`, `{"b":{"a":[]},"a":{}}`, `[[[[[[[[[[1]]]]]]]]]]`,
		`{"x":NaN}`, `[Infinity]`, `[01]`, `[1.]`, `[.5]`, `"a` + "\x00" + `"`, " \t\n\r[]\n", `[]x`, ``,
	}
	for _, v := range loadCanonicalSeedVectors(tb) {
		if v.Kind != "json" {
			continue
		}
		var text string
		if err := json.Unmarshal(v.Input, &text); err != nil {
			tb.Fatal(err)
		}
		seeds = append(seeds, text)
	}
	return seeds
}

func loadCanonicalSeedVectors(tb testing.TB) []canonicalVector {
	tb.Helper()
	raw, err := os.ReadFile("testdata/conformance/canonical.json")
	if err != nil {
		tb.Fatal(err)
	}
	var suite struct {
		Vectors []canonicalVector `json:"vectors"`
	}
	if err := json.Unmarshal(raw, &suite); err != nil {
		tb.Fatal(err)
	}
	if len(suite.Vectors) == 0 {
		tb.Fatal("the canonical suite has no vectors")
	}
	return suite.Vectors
}

// duplicateKey reports whether an object in raw (valid JSON) has two keys that
// decode to the same string, however each is escaped.
func duplicateKey(raw []byte) bool {
	dec := json.NewDecoder(bytes.NewReader(raw))
	type frame struct {
		object    bool
		keys      map[string]bool
		expectKey bool
	}
	var stack []*frame
	for {
		tok, err := dec.Token()
		if err != nil {
			return false
		}
		var top *frame
		if len(stack) > 0 {
			top = stack[len(stack)-1]
		}
		if top != nil && top.object && top.expectKey {
			if key, ok := tok.(string); ok {
				if top.keys[key] {
					return true
				}
				top.keys[key] = true
				top.expectKey = false
				continue
			}
		}
		switch tok {
		case json.Delim('{'):
			stack = append(stack, &frame{object: true, keys: map[string]bool{}, expectKey: true})
			continue
		case json.Delim('['):
			stack = append(stack, &frame{})
			continue
		case json.Delim('}'), json.Delim(']'):
			stack = stack[:len(stack)-1]
		}
		if len(stack) > 0 && stack[len(stack)-1].object {
			stack[len(stack)-1].expectKey = true
		}
	}
}

// FuzzCheckStrictJSON: strict JSON is valid JSON without duplicate keys, a
// refusal always names one of the documented reasons, and what is accepted
// decodes.
func FuzzCheckStrictJSON(f *testing.F) {
	for _, s := range conformanceSeeds(f) {
		f.Add([]byte(s))
	}
	f.Fuzz(func(t *testing.T, raw []byte) {
		err := CheckStrictJSON(raw)
		if err != nil {
			var refused *StrictJSONError
			if !errors.As(err, &refused) || !strictReasons[refused.Reason] {
				t.Fatalf("CheckStrictJSON(%q) = %v: not a documented refusal", raw, err)
			}
			return
		}
		if !json.Valid(raw) {
			t.Fatalf("CheckStrictJSON accepted %q, which is not JSON", raw)
		}
		if duplicateKey(raw) {
			t.Fatalf("CheckStrictJSON accepted %q, which has a duplicate key", raw)
		}
		if _, err := decodeJSON(raw); err != nil {
			t.Fatalf("CheckStrictJSON accepted %q, which does not decode: %v", raw, err)
		}
	})
}

// escapeEveryUnit writes s as a JSON string with every UTF-16 code unit escaped.
func escapeEveryUnit(s string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, u := range utf16.Encode([]rune(s)) {
		fmt.Fprintf(&b, `\u%04x`, u)
	}
	b.WriteByte('"')
	return b.String()
}

// FuzzStrictJSONDuplicateKeys: a key written twice is a duplicate however
// each copy is escaped, and the escaping of a key does not change the
// canonical form.
func FuzzStrictJSONDuplicateKeys(f *testing.F) {
	for _, s := range []string{"", "a", "é", "€", "😀", "\u2028", `"\`, "a\x00b", "Z", "\ue000"} {
		f.Add(s, int64(1), int64(2))
	}
	f.Fuzz(func(t *testing.T, key string, a, b int64) {
		if !utf8.ValidString(key) {
			return
		}
		plain, _ := json.Marshal(key)
		escaped := escapeEveryUnit(key)
		for _, doc := range []string{
			fmt.Sprintf(`{%s:%d,%s:%d}`, plain, a, plain, b),
			fmt.Sprintf(`{%s:%d,%s:%d}`, plain, a, escaped, b),
			fmt.Sprintf(`{"outer":[{%s:%d,"z":0,%s:%d}]}`, escaped, a, plain, b),
		} {
			var refused *StrictJSONError
			if err := CheckStrictJSON([]byte(doc)); !errors.As(err, &refused) || refused.Reason != "duplicate_key" {
				t.Fatalf("CheckStrictJSON(%s) = %v, want duplicate_key", doc, err)
			}
		}
		one, err := CanonicalJSON([]byte(fmt.Sprintf(`{%s:%d}`, plain, a)))
		if err != nil {
			t.Fatalf("CanonicalJSON refused a one-key object: %v", err)
		}
		other, err := CanonicalJSON([]byte(fmt.Sprintf(`{%s:%d}`, escaped, a)))
		if err != nil || other != one {
			t.Fatalf("the escaped key changes the canonical form: %q (%v), want %q", other, err, one)
		}
	})
}

// FuzzCanonicalJSON: the canonical form is accepted strict JSON, printable ASCII,
// its own canonical form (a round trip changes nothing), and the same value
// as the input; only strict JSON has one.
func FuzzCanonicalJSON(f *testing.F) {
	for _, s := range conformanceSeeds(f) {
		f.Add([]byte(s))
	}
	f.Fuzz(func(t *testing.T, raw []byte) {
		canonical, err := CanonicalJSON(raw)
		strictErr := CheckStrictJSON(raw)
		if err != nil {
			if strictErr == nil {
				t.Fatalf("CanonicalJSON(%q) failed on strict JSON: %v", raw, err)
			}
			return
		}
		if strictErr != nil {
			t.Fatalf("CanonicalJSON(%q) = %q, but the input is refused: %v", raw, canonical, strictErr)
		}
		// ensure_ascii: printable ASCII only; control characters, DEL and
		// everything above are escaped.
		for i := 0; i < len(canonical); i++ {
			if canonical[i] < 0x20 || canonical[i] >= 0x7f {
				t.Fatalf("CanonicalJSON(%q) = %q has the unescaped byte %#x", raw, canonical, canonical[i])
			}
		}
		if err := CheckStrictJSON([]byte(canonical)); err != nil {
			t.Fatalf("the canonical form %q is refused: %v", canonical, err)
		}
		again, err := CanonicalJSON([]byte(canonical))
		if err != nil || again != canonical {
			t.Fatalf("CanonicalJSON is not a fixed point: %q -> %q (%v)", canonical, again, err)
		}
		var in, out interface{}
		if err := json.Unmarshal(raw, &in); err != nil {
			t.Fatalf("encoding/json refuses accepted input %q: %v", raw, err)
		}
		if err := json.Unmarshal([]byte(canonical), &out); err != nil {
			t.Fatalf("encoding/json refuses the canonical form %q: %v", canonical, err)
		}
		if !reflect.DeepEqual(in, out) {
			t.Fatalf("the canonical form changes the value: %q -> %q", raw, canonical)
		}
	})
}

// FuzzVerifyBoundaryDecision: the decision decoder never panics, refuses what
// is not strict JSON with an error, gives a reason for every refusal, and
// reaches the same outcome on the decision's canonical form (the signature
// covers the parsed value, not its spelling).
func FuzzVerifyBoundaryDecision(f *testing.F) {
	var opts DecisionVerifyOptions
	for _, v := range loadCanonicalSeedVectors(f) {
		if v.Kind != "verify_decision" {
			continue
		}
		var input struct {
			Decision json.RawMessage `json:"decision"`
			Keys     []string        `json:"operator_public_keys"`
			Now      string          `json:"now"`
		}
		if err := json.Unmarshal(v.Input, &input); err != nil {
			f.Fatal(err)
		}
		if v.ID == "verify-decision-canonical" {
			now, err := time.Parse(time.RFC3339Nano, input.Now)
			if err != nil {
				f.Fatal(err)
			}
			opts = DecisionVerifyOptions{OperatorPublicKeys: input.Keys, Now: now}
		}
		f.Add([]byte(input.Decision))
	}
	if opts.OperatorPublicKeys == nil {
		f.Fatal("the canonical suite has no verify-decision-canonical vector")
	}
	f.Add([]byte(`{}`))
	f.Add([]byte(`[]`))
	f.Add([]byte(`{"signature":null}`))
	f.Fuzz(func(t *testing.T, raw []byte) {
		got, err := VerifyBoundaryDecision(raw, opts)
		if CheckStrictJSON(raw) != nil {
			if err == nil {
				t.Fatalf("VerifyBoundaryDecision(%q) = %+v without an error, but the input is refused", raw, got)
			}
			return
		}
		if err != nil {
			return
		}
		if !got.Accepted && got.Reason == "" {
			t.Fatalf("VerifyBoundaryDecision(%q) refused without a reason", raw)
		}
		canonical, cerr := CanonicalJSON(raw)
		if cerr != nil {
			t.Fatalf("no canonical form for an accepted decision document %q: %v", raw, cerr)
		}
		again, err := VerifyBoundaryDecision([]byte(canonical), opts)
		if err != nil || again != got {
			t.Fatalf("the canonical form verifies differently: %+v, then %+v (%v)", got, again, err)
		}
	})
}
