package genesismesh

// The vouching horizon for NA-signed records (1.8.0): a port of the Python
// reference's trust.key_succession.na_record_refusal, pinned by the
// "vouching" conformance suite.
//
// A record is placed at the time its kind names (NAVouchingTimeFields), its
// signature checked over the record as received with the NA keys valid then
// and, when now is given, the horizon applied: a record whose key ended more
// than the hand-over and the grace before now no longer vouches for the
// present (outlives_key). Without now the record is judged at its own time
// only, as audit packs and exports judge it.

import (
	"errors"
	"fmt"
	"time"
)

// NAVouchingTimeFields are the record kinds the vouching horizon applies to,
// and the field each is placed at (1.8.0).
var NAVouchingTimeFields = map[string]string{
	"CapabilityGrant":         "issued_at",
	"MembershipAttestation":   "issued_at",
	"RecognitionTreaty":       "issued_at",
	"SovereignRevocationFeed": "issued_at",
}

// GrantMaxValidity is the longest validity of a grant, and how long after
// its issue a grant may start (1.8.0, with GrantClockSkew either side).
const GrantMaxValidity = 24 * time.Hour

// GrantClockSkew is the clock skew a grant's start may have before its issue.
const GrantClockSkew = 300 * time.Second

// NARecordRefusal returns why an NA-signed record of model does not verify,
// or "" when it does (1.8.0). Reasons, in order: unknown_field or
// non_canonical_form, na_key_not_valid_at_time, invalid_signature,
// record_malformed (signed, but refused by the reference's time rules for its
// kind, such as a grant that starts more than 24 hours after its issue), and
// outlives_key (only with now).
func NARecordRefusal(model string, recordJSON []byte, history *NAKeyHistory, now *time.Time) (string, error) {
	field, known := NAVouchingTimeFields[model]
	if !known {
		return "", fmt.Errorf("genesismesh: %s is not a kind the vouching horizon applies to", model)
	}
	if history == nil {
		return "", errors.New("genesismesh: the NA's key history is required")
	}
	value, err := decodeJSON(recordJSON)
	if err != nil {
		return "", fmt.Errorf("genesismesh: record is not valid JSON: %w", err)
	}
	obj, isObject := value.(map[string]interface{})
	if !isObject {
		return "invalid_signature", nil
	}
	raw, isString := obj[field].(string)
	if !isString {
		return "invalid_signature", nil
	}
	at, err := ParseTimestamp(raw)
	if err != nil {
		return "invalid_signature", nil
	}
	keys := history.KeysAt(at)
	signer := ""
	for _, k := range keys {
		if signedAsReceived(model, obj, []string{k}) {
			signer = k
			break
		}
	}
	if signer == "" {
		others := []string{}
		for _, k := range history.AllKeys() {
			if !containsString(keys, k) {
				others = append(others, k)
			}
		}
		if len(others) > 0 && signedAsReceived(model, obj, others) {
			return "na_key_not_valid_at_time", nil
		}
		return "invalid_signature", nil
	}
	if refusal := strictRefusal(model, obj, []string{signer}); refusal != "" {
		return refusal, nil
	}
	if !modelTimesAccepted(model, obj) {
		return "record_malformed", nil
	}
	if now != nil && history.RecordOutlivesKey(signer, *now) {
		return "outlives_key", nil
	}
	return "", nil
}

// modelTimesAccepted applies the reference model's time rules for model: a
// grant is valid for at most GrantMaxValidity and starts within it of its
// issue; an attestation or a treaty expires after it starts, and is not
// issued after it expires.
func modelTimesAccepted(model string, obj object) bool {
	times := func(names ...string) ([]time.Time, bool) {
		out := make([]time.Time, 0, len(names))
		for _, name := range names {
			t, err := ParseTimestamp(str(obj, name))
			if err != nil {
				return nil, false
			}
			out = append(out, t)
		}
		return out, true
	}
	switch model {
	case "CapabilityGrant":
		t, ok := times("issued_at", "valid_from", "valid_until")
		if !ok {
			return false
		}
		issued, from, until := t[0], t[1], t[2]
		return from.Before(until) && until.Sub(from) <= GrantMaxValidity &&
			!from.Before(issued.Add(-GrantClockSkew)) && !from.After(issued.Add(GrantMaxValidity+GrantClockSkew))
	case "MembershipAttestation", "RecognitionTreaty":
		t, ok := times("issued_at", "valid_from", "expires_at")
		if !ok {
			return false
		}
		return t[2].After(t[1]) && !t[0].After(t[2])
	}
	return true
}
