package genesismesh

// NA key history (v1.5.0): which Network Authority key was valid when.
//
// The genesis block names the NA's first public key and the root key that
// signed it. A KeySuccessionRecord names the next NA key: signed by the root
// key, which is kept offline, and by the outgoing NA key. A verifier holding
// only the root public key (supplied out of band) rebuilds the key history
// with VerifyKeyHistory and accepts an NA signature made by the key that was
// valid at the record's own time, so records signed before and after a
// succession keep verifying. This is a port of the Python reference
// (genesis_mesh/trust/key_succession.py) with its reason codes.
//
// Validity windows (NAKey.Accepts):
//   - The genesis key is valid from the start of the network until the first
//     succession takes effect. Its valid_to is not enforced.
//   - A key named by a succession is valid from the succession's effective_at
//     until the next succession takes effect, and never at or after its own
//     new_valid_to.
//   - A succeeded key is still accepted for records dated up to
//     SuccessionHandover after the next key took effect.
//
// A key with an end (a succeeded key, or one with an enforced valid_to)
// vouches for a decision only while the decision expires no later than that
// end, the hand-over and MaxDecisionLifetime (decision_outlives_key).
//
// The vouching horizon (1.8.0): judged for the present, a record signed by a
// key that has ended vouches only until that key's end, the hand-over and a
// grace of at most SuccessionVouchingGrace (RecordOutlivesKey,
// KeysVouchingAt). The NA re-issues what is still valid under the new key.
// Judged at its own time, as audit packs and exports judge it, a record is
// unchanged.
//
// Without a key history every verifier behaves as before: a list of NA keys
// is valid at every time.

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

// SuccessionHandover is how long after a succession takes effect the
// outgoing key is still accepted (records signed just before the hand-over
// and stored just after it).
const SuccessionHandover = 300 * time.Second

// MaxDecisionLifetime is the longest lifetime of a decision the NA issues
// (v1.5.0): a decision signed by a key with an end is refused when it stays
// valid past that end, the hand-over and this lifetime
// ("decision_outlives_key"), so the holder of a succeeded key cannot mint a
// long-lived decision dated inside its window.
const MaxDecisionLifetime = 300 * time.Second

// SuccessionVouchingGrace is the vouching horizon's grace (1.8.0): how long
// past its end and the hand-over a record of a key that has ended still
// vouches for the present. It is also the most a verifier allows:
// WithVouchingGrace only shortens it.
const SuccessionVouchingGrace = 24 * time.Hour

// NARecordTimeFields is the field giving the time an NA-signed record read
// alone is placed at in the key history (v1.5.0; from v1.6.0 also a grant,
// its revocation and a time token, read alone or nested in a use): the key
// that may sign it is the key valid then. In the evidence store every entry
// is placed at its envelope's recorded_at instead.
var NARecordTimeFields = map[string]string{
	"ArchiveRecord":         "archived_at",
	"AuditPackManifest":     "created_at",
	"BoundaryDecision":      "decision_made_at",
	"BoundaryPolicy":        "issued_at",
	"CapabilityGrant":       "issued_at",
	"GrantRevocationRecord": "revoked_at",
	"JudgementRecord":       "judged_at",
	"JustificationProof":    "proof_issued_at",
	"MembershipAttestation": "issued_at",
	"QuarantineRecord":      "quarantined_at",
	"RecognitionTreaty":     "issued_at",
	"RegistryRecord":        "effective_at",
	"RetentionCheckpoint":   "created_at",
	"ReviewRecord":          "decided_at",
	"StoreAnchor":           "anchored_at",
	"TimeToken":             "issued_at",
}

// NAKey is one key of an NA's key history and the times it is valid at.
type NAKey struct {
	// KeySequence is the key's position in the history (the genesis key is 1).
	KeySequence int
	// KeyID is the key's identifier, "" when the history does not name it.
	KeyID     string
	PublicKey string
	// ValidFrom is nil for the genesis key: valid from the start of the network.
	ValidFrom *time.Time
	// SucceededAt is the next succession's effective_at; nil while the key is current.
	SucceededAt *time.Time
	// ValidTo is the declared end of validity (the genesis block's, or a succession's new_valid_to).
	ValidTo *time.Time
	// ValidToEnforced is false for the genesis key.
	ValidToEnforced bool
}

// Accepts reports whether a record dated at may carry this key's signature
// (with the hand-over).
func (k NAKey) Accepts(at time.Time) bool {
	if k.ValidFrom != nil && at.Before(*k.ValidFrom) {
		return false
	}
	if k.SucceededAt != nil && !at.Before(k.SucceededAt.Add(SuccessionHandover)) {
		return false
	}
	if k.ValidToEnforced && k.ValidTo != nil && !at.Before(*k.ValidTo) {
		return false
	}
	return true
}

// End returns the key's end (1.8.0): the earlier of its succession and its
// enforced valid_to, or nil for a key with neither.
func (k NAKey) End() *time.Time {
	end := k.SucceededAt
	if k.ValidToEnforced && k.ValidTo != nil && (end == nil || k.ValidTo.Before(*end)) {
		end = k.ValidTo
	}
	return end
}

// NAKeyHistory is an NA's keys in order, from a verified genesis block and
// its successions (VerifyKeyHistory), or a list of keys valid at every time
// (NAKeysValidAlways).
type NAKeyHistory struct {
	SovereignID   string
	RootPublicKey string
	Keys          []NAKey
	// grace is the vouching horizon's grace (1.8.0); nil is SuccessionVouchingGrace.
	grace *time.Duration
}

// VouchingGrace returns the vouching horizon's grace of this history (1.8.0).
func (h *NAKeyHistory) VouchingGrace() time.Duration {
	if h.grace == nil {
		return SuccessionVouchingGrace
	}
	return *h.grace
}

// WithVouchingGrace returns a copy of this history with a shorter grace
// (1.8.0): a negative grace is 0, and one longer than the current grace
// keeps it. Use it for the NA's unsigned vouching_grace_seconds, which may
// only shorten the grace.
func (h *NAKeyHistory) WithVouchingGrace(grace time.Duration) *NAKeyHistory {
	if grace < 0 {
		grace = 0
	}
	if current := h.VouchingGrace(); grace > current {
		grace = current
	}
	out := *h
	out.Keys = append([]NAKey(nil), h.Keys...)
	out.grace = &grace
	return &out
}

// VouchesUntil returns until when a record signed by publicKey vouches for
// the present (1.8.0): its key's end, the hand-over and the grace. nil for a
// key with no end, or not in this history.
func (h *NAKeyHistory) VouchesUntil(publicKey string) *time.Time {
	k := h.keyFor(publicKey)
	if k == nil {
		return nil
	}
	end := k.End()
	if end == nil {
		return nil
	}
	until := end.Add(SuccessionHandover + h.VouchingGrace())
	return &until
}

// RecordOutlivesKey reports whether a record signed by publicKey no longer
// vouches for the present at now (1.8.0): its key ended more than the
// hand-over and the grace ago. The dynamic twin of the decision bound.
func (h *NAKeyHistory) RecordOutlivesKey(publicKey string, now time.Time) bool {
	until := h.VouchesUntil(publicKey)
	return until != nil && !now.Before(*until)
}

// KeysVouchingAt returns the public keys a record dated at may be signed
// with and that still vouch at now (1.8.0): KeysAt without the keys whose
// horizon has passed.
func (h *NAKeyHistory) KeysVouchingAt(at, now time.Time) []string {
	out := []string{}
	for _, k := range h.KeysAt(at) {
		if !h.RecordOutlivesKey(k, now) {
			out = append(out, k)
		}
	}
	return out
}

// NAKeysValidAlways returns a history whose keys are valid at every time:
// what a list of NA keys meant before 1.5.0 (for a party whose history the
// verifier does not hold).
func NAKeysValidAlways(publicKeys []string) *NAKeyHistory {
	h := &NAKeyHistory{}
	for i, k := range publicKeys {
		h.Keys = append(h.Keys, NAKey{KeySequence: i + 1, PublicKey: k})
	}
	return h
}

// AllKeys returns every public key of the history, in order.
func (h *NAKeyHistory) AllKeys() []string {
	out := make([]string, 0, len(h.Keys))
	for _, k := range h.Keys {
		out = append(out, k.PublicKey)
	}
	return out
}

// KeysAt returns the public keys a record dated at may be signed with.
func (h *NAKeyHistory) KeysAt(at time.Time) []string {
	out := []string{}
	for _, k := range h.Keys {
		if k.Accepts(at) {
			out = append(out, k.PublicKey)
		}
	}
	return out
}

// keyFor returns the key of the history with this public key, nil when none.
func (h *NAKeyHistory) keyFor(publicKey string) *NAKey {
	for i := range h.Keys {
		if h.Keys[i].PublicKey == publicKey {
			return &h.Keys[i]
		}
	}
	return nil
}

// decisionOutlivesKey reports whether a decision signed by publicKey stays
// valid past what that key may vouch for: its end (its succession, or its
// enforced valid_to), the hand-over and MaxDecisionLifetime.
func (h *NAKeyHistory) decisionOutlivesKey(publicKey string, validUntil time.Time) bool {
	k := h.keyFor(publicKey)
	if k == nil {
		return false
	}
	end := k.End()
	return end != nil && validUntil.After(end.Add(SuccessionHandover+MaxDecisionLifetime))
}

// KeyHistoryVerification is the outcome of VerifyKeyHistory. When Verified
// is false, Reason names why and KeySequence the refused record's position
// (1 for the genesis block); nothing after it is accepted.
type KeyHistoryVerification struct {
	Verified    bool
	Reason      string
	KeySequence int
	History     *NAKeyHistory
}

// VerifyKeyHistory verifies a genesis block and its successions (a JSON
// array, as GET /na/key-history serves them; nil for none) in order, and
// builds the key history. rootPublicKey is the root key the verifier holds
// out of band: keys in the records prove nothing by themselves.
//
// Reasons, in the reference order: for the genesis block genesis_invalid,
// genesis_root_mismatch, genesis_invalid_signature; for each succession
// payload_invalid, invalid_signature, unknown_field, non_canonical_form,
// succession_sovereign_mismatch, succession_chain_break,
// succession_key_reused, succession_not_increasing, succession_after_expiry,
// succession_invalid_root_signature, succession_invalid_previous_signature.
func VerifyKeyHistory(genesisJSON, successionsJSON []byte, rootPublicKey string) (KeyHistoryVerification, error) {
	if rootPublicKey == "" {
		return KeyHistoryVerification{}, errors.New("genesismesh: the root public key, held out of band, is required")
	}
	genesis, err := decodeJSON(genesisJSON)
	if err != nil {
		return KeyHistoryVerification{}, fmt.Errorf("genesismesh: genesis block is not valid JSON: %w", err)
	}
	var successions []interface{}
	if len(bytes.TrimSpace(successionsJSON)) > 0 {
		v, err := decodeJSON(successionsJSON)
		if err != nil {
			return KeyHistoryVerification{}, fmt.Errorf("genesismesh: successions are not valid JSON: %w", err)
		}
		list, isList := v.([]interface{})
		if v != nil && !isList {
			return KeyHistoryVerification{}, errors.New("genesismesh: successions must be a JSON array")
		}
		successions = list
	}
	return verifyKeyHistory(genesis, successions, rootPublicKey), nil
}

// VerifyKeyHistoryResponse verifies the genesis block and successions of a
// GET /na/key-history response body (VerifyKeyHistory). The response's own
// keys and root key are not trusted: the history is rebuilt from
// rootPublicKey. From 1.8.0 the response's vouching_grace_seconds, which is
// unsigned, is taken only to shorten the grace.
func VerifyKeyHistoryResponse(responseJSON []byte, rootPublicKey string) (KeyHistoryVerification, error) {
	if rootPublicKey == "" {
		return KeyHistoryVerification{}, errors.New("genesismesh: the root public key, held out of band, is required")
	}
	body, err := decodeObject(responseJSON, "key history")
	if err != nil {
		return KeyHistoryVerification{}, err
	}
	successions, isList := body["successions"].([]interface{})
	if body["successions"] != nil && !isList {
		return KeyHistoryVerification{}, errors.New("genesismesh: successions must be a JSON array")
	}
	result := verifyKeyHistory(body["genesis"], successions, rootPublicKey)
	if hint, ok := numberOf(body["vouching_grace_seconds"]); ok && result.History != nil && hint >= 0 {
		result.History = result.History.WithVouchingGrace(time.Duration(hint * float64(time.Second)))
	}
	return result, nil
}

// numberOf reads a JSON number decoded by decodeJSON (json.Number or float64).
func numberOf(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	case float64:
		return n, true
	}
	return 0, false
}

func verifyKeyHistory(genesis interface{}, successions []interface{}, root string) KeyHistoryVerification {
	fail := func(position int, reason string) KeyHistoryVerification {
		return KeyHistoryVerification{Reason: reason, KeySequence: position}
	}
	block, isObject := genesis.(map[string]interface{})
	if !isObject || !validGenesis(block) {
		return fail(1, "genesis_invalid")
	}
	if str(block, "root_public_key") != root {
		return fail(1, "genesis_root_mismatch")
	}
	// v1.5.0 review fixes: strict like every other signed root.
	if reason := strictRefusal("GenesisBlock", block, []string{root}); reason == "unknown_field" || reason == "non_canonical_form" {
		return fail(1, reason)
	}
	if !signedAsReceived("GenesisBlock", block, []string{root}) {
		return fail(1, "genesis_invalid_signature")
	}
	sovereign := str(block, "network_name")
	na := block["network_authority"].(map[string]interface{})
	validTo, _ := ParseTimestamp(str(na, "valid_to"))
	keys := []NAKey{{KeySequence: 1, PublicKey: str(na, "public_key"), ValidTo: &validTo}}
	previousDigest := ""
	for _, raw := range successions {
		previous := keys[len(keys)-1]
		record, reason := successionRefusal(raw, previous, previousDigest, sovereign, root, keys)
		if reason != "" {
			position := previous.KeySequence + 1
			if record != nil {
				position = record.keySequence
			}
			return fail(position, reason)
		}
		effective, validTo := record.effectiveAt, record.newValidTo
		last := &keys[len(keys)-1]
		last.SucceededAt = &effective
		if last.KeyID == "" {
			last.KeyID = record.previousKeyID
		}
		keys = append(keys, NAKey{
			KeySequence: record.keySequence, KeyID: record.newKeyID, PublicKey: record.newPublicKey,
			ValidFrom: &effective, ValidTo: &validTo, ValidToEnforced: true,
		})
		previousDigest = record.digest
	}
	return KeyHistoryVerification{
		Verified: true, History: &NAKeyHistory{SovereignID: sovereign, RootPublicKey: root, Keys: keys},
	}
}

// validGenesis reports whether a genesis block has the fields the reference
// requires, of the types it reads (otherwise genesis_invalid).
func validGenesis(g object) bool {
	for _, k := range []string{"network_name", "network_version", "root_public_key"} {
		if _, ok := g[k].(string); !ok {
			return false
		}
	}
	na, ok := g["network_authority"].(map[string]interface{})
	if !ok {
		return false
	}
	if _, ok := na["public_key"].(string); !ok {
		return false
	}
	for _, k := range []string{"valid_from", "valid_to"} {
		if _, err := ParseTimestamp(str(na, k)); err != nil {
			return false
		}
	}
	manifest, ok := g["policy_manifest"].(map[string]interface{})
	if !ok {
		return false
	}
	if _, ok := manifest["hash"].(string); !ok {
		return false
	}
	if url, present := manifest["url"]; present && url != nil {
		if _, ok := url.(string); !ok {
			return false
		}
	}
	for _, k := range []string{"allowed_crypto_suites", "allowed_transports"} {
		if v, present := g[k]; present && !isStringList(v) {
			return false
		}
	}
	for k, fields := range map[string][]string{"bootstrap_anchors": {"id", "endpoint"}, "signatures": {"key_id", "sig"}} {
		v, present := g[k]
		if !present {
			continue
		}
		list, ok := v.([]interface{})
		if !ok {
			return false
		}
		for _, item := range list {
			obj, ok := item.(map[string]interface{})
			if !ok {
				return false
			}
			for _, f := range fields {
				if _, ok := obj[f].(string); !ok {
					return false
				}
			}
		}
	}
	return true
}

func isStringList(v interface{}) bool {
	list, ok := v.([]interface{})
	if !ok {
		return false
	}
	for _, item := range list {
		if _, ok := item.(string); !ok {
			return false
		}
	}
	return true
}

type succession struct {
	keySequence                                      int
	previousKeyID, previousPublicKey, previousDigest string
	newKeyID, newPublicKey, digest                   string
	effectiveAt, newValidTo, issuedAt                time.Time
}

// successionRefusal returns the parsed record (nil when it does not parse)
// and why it is refused, "" when it is accepted, in the reference order
// (v1.5.0 review fixes): payload_invalid; unknown_field or
// invalid_signature, non_canonical_form; succession_sovereign_mismatch,
// succession_chain_break, succession_key_reused (the root key included),
// succession_backdated, succession_not_increasing, succession_after_expiry;
// then the root key's, the new key's and the outgoing key's signatures.
func successionRefusal(raw interface{}, previous NAKey, previousDigest, sovereign, root string, held []NAKey) (*succession, string) {
	const model = "KeySuccessionRecord"
	obj, isObject := raw.(map[string]interface{})
	if !isObject {
		return nil, "payload_invalid"
	}
	// Parsed with its unknown fields set aside.
	record, ok := parseSuccession(obj)
	if !ok {
		return nil, "payload_invalid"
	}
	signers := []string{root, previous.PublicKey}
	if validPublicKey(record.newPublicKey) {
		signers = append(signers, record.newPublicKey)
	}
	if reason := strictRefusal(model, obj, signers); reason != "" {
		return record, reason
	}
	if str(obj, "sovereign_id") != sovereign {
		return record, "succession_sovereign_mismatch"
	}
	if record.keySequence != previous.KeySequence+1 || record.previousPublicKey != previous.PublicKey ||
		record.previousDigest != previousDigest {
		return record, "succession_chain_break"
	}
	if !validPublicKey(record.newPublicKey) || record.newPublicKey == root {
		return record, "succession_key_reused"
	}
	for _, k := range held {
		if k.PublicKey == record.newPublicKey {
			return record, "succession_key_reused"
		}
	}
	if record.issuedAt.After(record.effectiveAt) {
		return record, "succession_backdated"
	}
	if (previous.ValidFrom != nil && !record.effectiveAt.After(*previous.ValidFrom)) ||
		!record.newValidTo.After(record.effectiveAt) {
		return record, "succession_not_increasing"
	}
	if previous.ValidToEnforced && previous.ValidTo != nil && record.effectiveAt.After(*previous.ValidTo) {
		return record, "succession_after_expiry"
	}
	if !signedAsReceived(model, obj, []string{root}) {
		return record, "succession_invalid_root_signature"
	}
	// The new key's signature proves it exists and is held.
	if !signedAsReceived(model, obj, []string{record.newPublicKey}) {
		return record, "succession_invalid_new_key_signature"
	}
	if !signedAsReceived(model, obj, []string{previous.PublicKey}) {
		return record, "succession_invalid_previous_signature"
	}
	return record, ""
}

// strictRefusal returns why a record must be refused for its form, "" when
// it need not be: with a field this release does not know, or not in
// canonical form, "unknown_field" or "non_canonical_form" when a signature
// by one of signers verifies over it as received, "invalid_signature"
// otherwise.
func strictRefusal(model string, obj object, signers []string) string {
	unknown := hasUnknownFields(model, obj)
	if !unknown && len(nonCanonicalFields(model, obj)) == 0 {
		return ""
	}
	if !signedAsReceived(model, obj, signers) {
		return "invalid_signature"
	}
	if unknown {
		return "unknown_field"
	}
	return "non_canonical_form"
}

// parseSuccession reads a KeySuccessionRecord as the reference validates it,
// its unknown fields set aside (otherwise payload_invalid).
func parseSuccession(obj object) (*succession, bool) {
	text := func(key string, min, max int) (string, bool) {
		s, ok := obj[key].(string)
		n := utf8.RuneCountInString(s)
		return s, ok && n >= min && n <= max
	}
	// The reference gives an absent succession_id a fresh one: the form
	// check then refuses the record (non_canonical_form or invalid_signature).
	if v, present := obj["succession_id"]; present {
		if _, ok := text("succession_id", 1, 128); !ok || v == nil {
			return nil, false
		}
	}
	r := &succession{}
	var ok bool
	if _, ok = text("sovereign_id", 1, 256); !ok {
		return nil, false
	}
	for key, dst := range map[string]*string{
		"previous_key_id": &r.previousKeyID, "previous_public_key": &r.previousPublicKey,
		"new_key_id": &r.newKeyID, "new_public_key": &r.newPublicKey,
	} {
		if *dst, ok = text(key, 1, 128); !ok {
			return nil, false
		}
	}
	if v := obj["previous_succession_digest"]; v != nil {
		if r.previousDigest, ok = text("previous_succession_digest", 64, 64); !ok {
			return nil, false
		}
	}
	n, isNumber := obj["key_sequence"].(json.Number)
	if !isNumber {
		return nil, false
	}
	seq, err := n.Int64()
	if err != nil || seq < 2 || seq > int64(^uint(0)>>1) {
		return nil, false
	}
	r.keySequence = int(seq)
	for key, dst := range map[string]*time.Time{"effective_at": &r.effectiveAt, "new_valid_to": &r.newValidTo, "issued_at": &r.issuedAt} {
		t, err := time.Parse(time.RFC3339Nano, str(obj, key))
		if err != nil {
			return nil, false
		}
		if _, offset := t.Zone(); offset != 0 {
			return nil, false
		}
		*dst = t.UTC()
	}
	if v, present := obj["signatures"]; present {
		list, ok := v.([]interface{})
		if !ok {
			return nil, false
		}
		for _, item := range list {
			sig, ok := item.(map[string]interface{})
			if !ok {
				return nil, false
			}
			if _, ok := sig["key_id"].(string); !ok {
				return nil, false
			}
			if _, ok := sig["sig"].(string); !ok {
				return nil, false
			}
		}
	}
	canonical, err := receivedCanonical("KeySuccessionRecord", obj)
	if err != nil {
		return nil, false
	}
	r.digest = sha256Hex(canonical)
	return r, true
}

// validPublicKey reports whether value is a base64 Ed25519 public key (32 bytes).
func validPublicKey(value string) bool {
	if strings.ContainsAny(value, "\r\n") {
		return false
	}
	raw, err := base64.StdEncoding.DecodeString(value)
	return err == nil && len(raw) == 32
}

// receivedCanonical returns the signed form of a record as received, from
// the field registry: every field but the signature (those left out when
// null omitted), or the fixed field list of a root signed over one.
func receivedCanonical(model string, obj object) (string, error) {
	spec, ok := canonicalRegistry.Models[model]
	if !ok {
		return "", fmt.Errorf("genesismesh: no registry entry for %s", model)
	}
	if spec.CanonicalFields != nil {
		body := object{}
		for _, k := range spec.CanonicalFields {
			body[k] = obj[k]
		}
		return canonicalOf(body)
	}
	var signature []string
	if spec.SignatureField != nil {
		signature = []string{*spec.SignatureField}
	}
	return canonicalOf(without(obj, signature, spec.OmitWhenNone...))
}

// signedAsReceived reports whether a signature on the record verifies over
// its received form with one of keys.
func signedAsReceived(model string, obj object, keys []string) bool {
	spec := canonicalRegistry.Models[model]
	canonical, err := receivedCanonical(model, obj)
	if err != nil || spec.SignatureField == nil {
		return false
	}
	var sigs []string
	if *spec.SignatureField == "signatures" {
		sigs = signaturesOf(obj)
	} else if s, ok := signatureOf(obj, *spec.SignatureField); ok {
		sigs = []string{s}
	}
	for _, s := range sigs {
		if verifyEd25519(canonical, s, keys) {
			return true
		}
	}
	return false
}

func containsString(list []string, s string) bool {
	for _, item := range list {
		if item == s {
			return true
		}
	}
	return false
}
