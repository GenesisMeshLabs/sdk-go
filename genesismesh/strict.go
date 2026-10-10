package genesismesh

// Strict verification (v1.2.0): every signed field of a record is known.
//
// Before 1.2.0 this package copied every received field into the signed form,
// so a field a newer signer covered verified here and could change what a
// record means. The registry (canonical_registry.json, generated from the
// Python reference and shipped in the shared conformance suite
// "field_registry") lists every field of every record this package verifies.
// Verifiers check the signature over the record as received first; a signed
// field the registry does not list is then refused as "unknown_field", and a
// record signed over a timestamp the reference does not write as
// "non_canonical_form" (v1.2.0). See the core's reference page "Canonical Form
// of Signed Records".
//
// After copying a new suite to testdata/conformance/field_registry.json, run
// `python scripts/sync_canonical_registry.py`; the conformance test fails
// while the embedded registry differs from the suite.

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"time"
)

//go:embed canonical_registry.json
var canonicalRegistryJSON []byte

var canonicalRegistry = mustLoadRegistry(canonicalRegistryJSON)

type registryDoc struct {
	Version int                          `json:"version"`
	Models  map[string]registryModelSpec `json:"models"`
}

type registryModelSpec struct {
	Fields          map[string]interface{} `json:"fields"`
	SignatureField  string                 `json:"signature_field"`
	OmitWhenNone    []string               `json:"omit_when_none"`
	CanonicalFields []string               `json:"canonical_fields"`
}

func mustLoadRegistry(raw []byte) registryDoc {
	var doc registryDoc
	if err := json.Unmarshal(raw, &doc); err != nil {
		panic(fmt.Sprintf("genesismesh: the embedded canonical registry is invalid: %v", err))
	}
	return doc
}

// UnknownFields returns the dotted paths, sorted, of the signed fields in
// recordJSON that model does not define, at any depth
// ("policy_binding.policies.0.extra"). Only the signed projection is checked
// (not the signature, not an agreement's unsigned fields); free-form fields are
// not inspected; values of the wrong type are left to validation.
func UnknownFields(model string, recordJSON []byte) ([]string, error) {
	v, err := decodeJSON(recordJSON)
	if err != nil {
		return nil, fmt.Errorf("genesismesh: record is not valid JSON: %w", err)
	}
	found := unknownFieldsIn(model, v, "", true)
	sort.Strings(found)
	return found, nil
}

func outsideProjection(spec registryModelSpec, key string) bool {
	if key == spec.SignatureField {
		return true
	}
	if spec.CanonicalFields == nil {
		return false
	}
	for _, f := range spec.CanonicalFields {
		if f == key {
			return false
		}
	}
	return true
}

func unknownFieldsIn(model string, data interface{}, path string, projection bool) []string {
	spec, ok := canonicalRegistry.Models[model]
	record, isObject := data.(map[string]interface{})
	if !ok || !isObject {
		return nil
	}
	var found []string
	for key, value := range record {
		if projection && outsideProjection(spec, key) {
			continue
		}
		kind, known := spec.Fields[key]
		if !known {
			found = append(found, path+key)
			continue
		}
		nested, structured := kind.(map[string]interface{})
		if value == nil || !structured {
			continue
		}
		if m, ok := nested["object"].(string); ok {
			found = append(found, unknownFieldsIn(m, value, path+key+".", false)...)
		} else if m, ok := nested["list"].(string); ok {
			if items, ok := value.([]interface{}); ok {
				for i, item := range items {
					found = append(found, unknownFieldsIn(m, item, fmt.Sprintf("%s%s.%d.", path, key, i), false)...)
				}
			}
		} else if m, ok := nested["map"].(string); ok {
			if items, ok := value.(map[string]interface{}); ok {
				for k, item := range items {
					found = append(found, unknownFieldsIn(m, item, path+key+"."+k+".", false)...)
				}
			}
		}
	}
	return found
}

// hasUnknownFields reports whether a decoded record carries a signed field model does not define.
func hasUnknownFields(model string, record object) bool {
	return len(unknownFieldsIn(model, record, "", true)) > 0
}

var timestampForm = regexp.MustCompile(`^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2}):(\d{2})(?:\.(\d{6}))?(Z|[+-](\d{2}):(\d{2}))?$`)

// CanonicalTimestamp reports whether value is a timestamp in canonical form
// (v1.2.0): what the reference writes, YYYY-MM-DDTHH:MM:SS, six digits of
// microseconds when not all zero, then Z for UTC or +HH:MM / -HH:MM for
// another offset (none for a timestamp without one), naming an instant that
// exists.
func CanonicalTimestamp(value string) bool {
	m := timestampForm.FindStringSubmatch(value)
	if m == nil || m[7] == "000000" || m[8] == "+00:00" || m[8] == "-00:00" {
		return false
	}
	n := func(i int) int { v, _ := strconv.Atoi(m[i]); return v }
	if m[9] != "" && (n(9) > 23 || n(10) > 59) {
		return false
	}
	year, month, day := n(1), n(2), n(3)
	if year < 1 || month < 1 || month > 12 || day < 1 || n(4) > 23 || n(5) > 59 || n(6) > 59 {
		return false
	}
	return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC).Day() == day
}

// nonCanonicalTimestamps returns the dotted paths, sorted, of the timestamps in
// record's signed projection that are not in canonical form (v1.2.0).
func nonCanonicalTimestamps(model string, record interface{}) []string {
	var found []string
	var walk func(name string, data interface{}, prefix string, projection bool)
	walk = func(name string, data interface{}, prefix string, projection bool) {
		spec, ok := canonicalRegistry.Models[name]
		obj, isObject := data.(map[string]interface{})
		if !ok || !isObject {
			return
		}
		for key, value := range obj {
			if (projection && outsideProjection(spec, key)) || value == nil {
				continue
			}
			kind := spec.Fields[key]
			if kind == "timestamp" {
				items, isList := value.([]interface{})
				if !isList {
					items = []interface{}{value}
				}
				for _, item := range items {
					if s, isString := item.(string); isString && !CanonicalTimestamp(s) {
						found = append(found, prefix+key)
						break
					}
				}
				continue
			}
			nested, structured := kind.(map[string]interface{})
			if !structured {
				continue
			}
			if m, ok := nested["object"].(string); ok {
				walk(m, value, prefix+key+".", false)
			} else if m, ok := nested["list"].(string); ok {
				if items, ok := value.([]interface{}); ok {
					for i, item := range items {
						walk(m, item, fmt.Sprintf("%s%s.%d.", prefix, key, i), false)
					}
				}
			} else if m, ok := nested["map"].(string); ok {
				if items, ok := value.(map[string]interface{}); ok {
					for k, item := range items {
						walk(m, item, prefix+key+"."+k+".", false)
					}
				}
			}
		}
	}
	walk(model, record, "", true)
	sort.Strings(found)
	return found
}
