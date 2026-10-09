package genesismesh

// Strict verification (v1.2.0): every signed field of a record is known.
//
// Before 1.2.0 this package copied every received field into the signed form,
// so a field a newer signer covered verified here and could change what a
// record means. The registry (canonical_registry.json, generated from the
// Python reference and shipped in the shared conformance suite
// "field_registry") lists every field of every record this package verifies.
// Verifiers check the signature over the record as received first; a signed
// field the registry does not list is then refused as "unknown_field". See the
// core's reference page "Canonical Form of Signed Records".
//
// After copying a new suite to testdata/conformance/field_registry.json, run
// `python scripts/sync_canonical_registry.py`; the conformance test fails
// while the embedded registry differs from the suite.

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"sort"
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
