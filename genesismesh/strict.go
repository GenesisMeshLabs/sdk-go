package genesismesh

// Strict verification (v1.2.0): every field of a signed record is known.
//
// A verifier that copies every received field into the signed form accepts a
// field it does not understand whenever the signer covered it, so a field
// added in a later release could change what a record means. The registry
// (canonical_registry.json, generated from the Python reference and shipped
// in the shared conformance suite "canonical") lists every field of every
// record this SDK verifies; anything else is refused as "unknown_field". See
// the core's reference page "Canonical Form of Signed Records".
//
// After copying a new suite to testdata/conformance/canonical.json, write its
// "registry" member to canonical_registry.json; the conformance test fails
// while the two differ.

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
	Version    int                          `json:"version"`
	EntryKinds []string                     `json:"entry_kinds"`
	Models     map[string]registryModelSpec `json:"models"`
}

type registryModelSpec struct {
	Fields map[string]interface{} `json:"fields"`
}

func mustLoadRegistry(raw []byte) registryDoc {
	var doc registryDoc
	if err := json.Unmarshal(raw, &doc); err != nil {
		panic(fmt.Sprintf("genesismesh: the embedded canonical registry is invalid: %v", err))
	}
	return doc
}

// CanonicalRegistryJSON returns the embedded field registry of signed records.
func CanonicalRegistryJSON() []byte {
	return append([]byte(nil), canonicalRegistryJSON...)
}

// UnknownFields returns the dotted paths, sorted, of the fields in recordJSON
// that model does not define, at any depth ("policy_binding.policies.0.extra").
// Free-form fields are not inspected; values of the wrong type are left to
// validation.
func UnknownFields(model string, recordJSON []byte) ([]string, error) {
	v, err := decodeJSON(recordJSON)
	if err != nil {
		return nil, fmt.Errorf("genesismesh: record is not valid JSON: %w", err)
	}
	found := unknownFieldsIn(model, v, "")
	sort.Strings(found)
	return found, nil
}

// IsKnownEntryKind reports whether this SDK knows the evidence entry kind.
func IsKnownEntryKind(kind string) bool {
	for _, k := range canonicalRegistry.EntryKinds {
		if k == kind {
			return true
		}
	}
	return false
}

func unknownFieldsIn(model string, data interface{}, path string) []string {
	spec, ok := canonicalRegistry.Models[model]
	record, isObject := data.(map[string]interface{})
	if !ok || !isObject {
		return nil
	}
	var found []string
	for key, value := range record {
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
			found = append(found, unknownFieldsIn(m, value, path+key+".")...)
		} else if m, ok := nested["list"].(string); ok {
			if items, ok := value.([]interface{}); ok {
				for i, item := range items {
					found = append(found, unknownFieldsIn(m, item, fmt.Sprintf("%s%s.%d.", path, key, i))...)
				}
			}
		} else if m, ok := nested["map"].(string); ok {
			if items, ok := value.(map[string]interface{}); ok {
				for k, item := range items {
					found = append(found, unknownFieldsIn(m, item, path+key+"."+k+".")...)
				}
			}
		}
	}
	return found
}

// hasUnknownFields reports whether a decoded record carries a field model does not define.
func hasUnknownFields(model string, record object) bool {
	return len(unknownFieldsIn(model, record, "")) > 0
}
