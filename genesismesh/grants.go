package genesismesh

// Pre-issued grants (v1.6.0): a grant's NA signature, checked offline.
//
// A CapabilityGrant is issued by the Network Authority before the bearer acts
// and lets the bearer act within it while the NA is unreachable. Requesting
// grants, holding time tokens, acting within a grant and recording its uses
// live in the TypeScript and Rust SDKs; this SDK checks that a grant it is
// shown is the NA's: signed, with every signed field known and in canonical
// form, by the key of the NA's key history valid at the grant's issued_at.

import (
	"errors"
	"fmt"
	"time"
)

// GrantVerification is the outcome of VerifyCapabilityGrant. Valid means the
// grant is the NA's (Reason "accepted"); otherwise Reason names why not.
type GrantVerification struct {
	Valid   bool
	Reason  string
	GrantID string
}

// VerifyCapabilityGrant (v1.6.0) checks a CapabilityGrant's NA signature
// offline, as received, with the NA's key history (VerifyKeyHistory; use
// NAKeysValidAlways for a plain list of NA keys). Reasons, in the order
// VerifyBoundaryDecision checks them: missing_signature, invalid_signature (no key of the
// history signed it), unknown_field, non_canonical_form, then
// na_key_not_valid_at_time (signed by a key of the history that is not valid
// at the grant's issued_at). The grant's limits (validity, budget, envelope)
// and its revocation are not checked here: they are the NA's and the
// holder's.
func VerifyCapabilityGrant(grantJSON []byte, history *NAKeyHistory) (GrantVerification, error) {
	return verifyCapabilityGrant(grantJSON, history, nil)
}

// VerifyCapabilityGrantAt (1.8.0) is VerifyCapabilityGrant judged for the
// present: after the checks above, a grant whose signing key has passed its
// vouching horizon at now is refused as outlives_key (RecordOutlivesKey).
func VerifyCapabilityGrantAt(grantJSON []byte, history *NAKeyHistory, now time.Time) (GrantVerification, error) {
	return verifyCapabilityGrant(grantJSON, history, &now)
}

func verifyCapabilityGrant(grantJSON []byte, history *NAKeyHistory, now *time.Time) (GrantVerification, error) {
	const model = "CapabilityGrant"
	if history == nil {
		return GrantVerification{}, errors.New("genesismesh: the NA's key history is required")
	}
	obj, err := decodeObject(grantJSON, "grant")
	if err != nil {
		return GrantVerification{}, err
	}
	result := func(reason string) (GrantVerification, error) {
		return GrantVerification{Valid: reason == "accepted", Reason: reason, GrantID: str(obj, "grant_id")}, nil
	}
	if signatureAbsent(obj) {
		return result("missing_signature")
	}
	if !signedAsReceived(model, obj, history.AllKeys()) {
		return result("invalid_signature")
	}
	if hasUnknownFields(model, obj) {
		return result("unknown_field")
	}
	if len(nonCanonicalFields(model, obj)) > 0 {
		return result("non_canonical_form")
	}
	at, err := ParseTimestamp(str(obj, NARecordTimeFields[model]))
	if err != nil {
		return GrantVerification{}, fmt.Errorf("genesismesh: grant issued_at: %w", err)
	}
	if !signedAsReceived(model, obj, history.KeysAt(at)) {
		return result("na_key_not_valid_at_time")
	}
	if now != nil && len(history.KeysVouchingAt(at, *now)) < len(history.KeysAt(at)) &&
		!signedAsReceived(model, obj, history.KeysVouchingAt(at, *now)) {
		return result("outlives_key")
	}
	return result("accepted")
}
