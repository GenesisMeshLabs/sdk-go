package genesismesh

// Offline verification (v0.61): check signed Genesis Mesh artifacts without
// calling the Network Authority. Each function is a port of the Python
// reference and returns the same reason codes, so a Go verifier and the NA
// agree on every decision (checked by the shared conformance vectors).
//
// Pass artifacts as the JSON received from the NA or the signer. Signatures
// cover the canonical form of exactly what was signed; re-encoding through a
// Go struct can drop fields or change number literals (1.0 becomes 1) and
// make a valid signature fail.

import (
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"
)

type object = map[string]interface{}

func decodeObject(raw []byte, what string) (object, error) {
	v, err := decodeJSON(raw)
	if err != nil {
		return nil, fmt.Errorf("genesismesh: %s is not valid JSON: %w", what, err)
	}
	obj, ok := v.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("genesismesh: %s must be a JSON object", what)
	}
	return obj, nil
}

// without returns a shallow copy minus `always`, and minus `whenNull` keys whose value is null.
func without(obj object, always []string, whenNull ...string) object {
	out := make(object, len(obj))
	for k, v := range obj {
		out[k] = v
	}
	for _, k := range always {
		delete(out, k)
	}
	for _, k := range whenNull {
		if v, ok := out[k]; ok && v == nil {
			delete(out, k)
		}
	}
	return out
}

func canonicalOf(v interface{}) (string, error) {
	b, err := marshalCanonical(v)
	return string(b), err
}

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

func str(obj object, key string) string {
	s, _ := obj[key].(string)
	return s
}

// verifyEd25519 reports whether sigB64 is a valid signature of message under any key.
func verifyEd25519(message string, sigB64 string, publicKeys []string) bool {
	sig, err := base64.StdEncoding.DecodeString(sigB64)
	if err != nil || len(sig) != ed25519.SignatureSize {
		return false
	}
	for _, k := range publicKeys {
		pub, err := base64.StdEncoding.DecodeString(k)
		if err != nil || len(pub) != ed25519.PublicKeySize {
			continue
		}
		if ed25519.Verify(ed25519.PublicKey(pub), []byte(message), sig) {
			return true
		}
	}
	return false
}

func signatureOf(obj object, field string) (string, bool) {
	sig, ok := obj[field].(map[string]interface{})
	if !ok {
		return "", false
	}
	s, ok := sig["sig"].(string)
	return s, ok && s != ""
}

func signaturesOf(obj object) []string {
	list, _ := obj["signatures"].([]interface{})
	out := make([]string, 0, len(list))
	for _, item := range list {
		if sig, ok := item.(map[string]interface{}); ok {
			if s, ok := sig["sig"].(string); ok {
				out = append(out, s)
			}
		}
	}
	return out
}

// ParseTimestamp parses an ISO 8601 timestamp as written by the NA (microseconds, Z or offset).
func ParseTimestamp(value string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		// Python isoformat() without an offset is UTC in Genesis Mesh artifacts.
		t, err = time.Parse("2006-01-02T15:04:05.999999999", value)
		if err != nil {
			return time.Time{}, fmt.Errorf("genesismesh: invalid timestamp %q", value)
		}
	}
	return t.UTC(), nil
}

// ── digests ────────────────────────────────────────────────────────────────

// AttestationDigest returns MembershipAttestation.digest(): SHA-256 of the canonical body without signatures.
func AttestationDigest(attestationJSON []byte) (string, error) {
	obj, err := decodeObject(attestationJSON, "attestation")
	if err != nil {
		return "", err
	}
	c, err := canonicalOf(without(obj, []string{"signatures"}))
	return sha256Hex(c), err
}

// PolicyDigest returns BoundaryPolicy.digest(): SHA-256 of the canonical body without the signature.
func PolicyDigest(policyJSON []byte) (string, error) {
	obj, err := decodeObject(policyJSON, "boundary policy")
	if err != nil {
		return "", err
	}
	return policyDigestOf(obj)
}

func policyDigestOf(obj object) (string, error) {
	c, err := canonicalOf(without(obj, []string{"signature"}))
	return sha256Hex(c), err
}

// AppliedPolicyRef identifies one policy version in a policy binding.
type AppliedPolicyRef struct {
	PolicyID     string
	Version      json.Number
	PolicyDigest string
}

// PolicySetDigest digests the ordered (policy_id, version, policy_digest) list of a PolicyBinding.
func PolicySetDigest(refs []AppliedPolicyRef) (string, error) {
	list := make([]interface{}, len(refs))
	for i, r := range refs {
		list[i] = []interface{}{r.PolicyID, r.Version, r.PolicyDigest}
	}
	c, err := canonicalOf(list)
	return sha256Hex(c), err
}

// ── agreements ─────────────────────────────────────────────────────────────

// AgreementVerification is the outcome of VerifyAgreement.
type AgreementVerification struct {
	Accepted    bool
	Reason      string
	AgreementID string
}

// agreementCanonical is the body both parties sign; identical for CapabilityCounter and AgreementRecord.
// agreementCanonicalFields are the fields both parties sign (checked against the field registry).
var agreementCanonicalFields = []string{
	"agreed_terms", "graph_digest", "offer_id", "offerer_evidence",
	"offerer_sovereign_id", "responder_evidence", "responder_sovereign_id",
}

// decisionOmittedWhenAbsent are the decision fields left out of the signed form when null
// (checked against the field registry).
var decisionOmittedWhenAbsent = []string{"policy_binding", "attestation_binding"}

func agreementCanonical(obj object) (string, error) {
	body := object{}
	for _, k := range agreementCanonicalFields {
		body[k] = obj[k]
	}
	return canonicalOf(body)
}

// VerifyAgreement checks an AgreementRecord's dual signatures (and optional graph binding).
// expectedGraphDigest "" skips the graph check. Reasons: accepted,
// missing_offerer_signature, invalid_offerer_signature, missing_responder_signature,
// invalid_responder_signature, graph_digest_mismatch.
func VerifyAgreement(agreementJSON []byte, offererKeys, responderKeys []string, expectedGraphDigest string) (AgreementVerification, error) {
	obj, err := decodeObject(agreementJSON, "agreement")
	if err != nil {
		return AgreementVerification{}, err
	}
	result := func(accepted bool, reason string) AgreementVerification {
		return AgreementVerification{Accepted: accepted, Reason: reason, AgreementID: str(obj, "agreement_id")}
	}
	sigs := signaturesOf(obj)
	if len(sigs) == 0 {
		return result(false, "missing_offerer_signature"), nil
	}
	canonical, err := agreementCanonical(obj)
	if err != nil {
		return AgreementVerification{}, err
	}
	anyValid := func(keys []string) bool {
		for _, s := range sigs {
			if verifyEd25519(canonical, s, keys) {
				return true
			}
		}
		return false
	}
	if !anyValid(offererKeys) {
		return result(false, "invalid_offerer_signature"), nil
	}
	if !anyValid(responderKeys) {
		if len(sigs) < 2 {
			return result(false, "missing_responder_signature"), nil
		}
		return result(false, "invalid_responder_signature"), nil
	}
	if expectedGraphDigest != "" && str(obj, "graph_digest") != expectedGraphDigest {
		return result(false, "graph_digest_mismatch"), nil
	}
	// v1.2.0: an authentic agreement with a signed field this SDK does not know (strict.go).
	if hasUnknownFields("AgreementRecord", obj) {
		return result(false, "unknown_field"), nil
	}
	return result(true, "accepted"), nil
}

// ── boundary decisions ─────────────────────────────────────────────────────

// DecisionVerifyOptions configures VerifyBoundaryDecision.
type DecisionVerifyOptions struct {
	// OperatorPublicKeys are the NA keys that may sign decisions.
	OperatorPublicKeys []string
	// Now is the verification time (zero: time.Now()).
	Now time.Time
	// FreshnessProofIssuerKeys, when set, must verify an embedded freshness proof.
	FreshnessProofIssuerKeys []string
	// ExpectedPolicies (signed BoundaryPolicy JSON), when non-nil, must equal the decision's policy binding.
	ExpectedPolicies [][]byte
	// ExpectedAttestation (signed MembershipAttestation JSON), when set, must match the attestation binding.
	ExpectedAttestation []byte
}

// DecisionVerification is the outcome of VerifyBoundaryDecision. Accepted means the
// decision verified; Authorized is what it decided (a verified denial is Accepted, not Authorized).
type DecisionVerification struct {
	Accepted   bool
	Reason     string
	Authorized bool
	DecisionID string
}

var attestationGateNames = map[string]bool{"attestation_status": true, "attestation_validity": true}
var builtinGateNames = map[string]bool{
	"capability_check": true, "validity_window": true, "freshness_check": true, "freshness_proof": true,
	"attestation_status": true, "attestation_validity": true,
}

// VerifyBoundaryDecision checks a BoundaryDecision's signature, expiry and bindings offline.
func VerifyBoundaryDecision(decisionJSON []byte, opts DecisionVerifyOptions) (DecisionVerification, error) {
	d, err := decodeObject(decisionJSON, "boundary decision")
	if err != nil {
		return DecisionVerification{}, err
	}
	authorized, _ := d["authorized"].(bool)
	result := func(accepted bool, reason string, auth bool) DecisionVerification {
		return DecisionVerification{Accepted: accepted, Reason: reason, Authorized: auth, DecisionID: str(d, "decision_id")}
	}
	reject := func(reason string) (DecisionVerification, error) { return result(false, reason, authorized), nil }

	sig, ok := signatureOf(d, "signature")
	if !ok {
		return reject("missing_signature")
	}
	now := opts.Now
	if now.IsZero() {
		now = time.Now()
	}
	validUntil, err := ParseTimestamp(str(d, "decision_valid_until"))
	if err != nil {
		return DecisionVerification{}, err
	}
	if now.After(validUntil) {
		return reject("decision_expired")
	}
	canonical, err := canonicalOf(without(d, []string{"signature"}, decisionOmittedWhenAbsent...))
	if err != nil {
		return DecisionVerification{}, err
	}
	if !verifyEd25519(canonical, sig, opts.OperatorPublicKeys) {
		return reject("invalid_signature")
	}
	// v1.2.0: an authentic decision with a signed field this SDK does not know, or expected
	// inputs it cannot read, is refused by name: upgrade this SDK (strict.go).
	if hasUnknownFields("BoundaryDecision", d) {
		return reject("unknown_field")
	}
	for _, raw := range opts.ExpectedPolicies {
		if p, err := decodeObject(raw, "expected policy"); err == nil && hasUnknownFields("BoundaryPolicy", p) {
			return reject("unknown_field")
		}
	}
	if opts.ExpectedAttestation != nil {
		if a, err := decodeObject(opts.ExpectedAttestation, "expected attestation"); err == nil && hasUnknownFields("MembershipAttestation", a) {
			return reject("unknown_field")
		}
	}

	if proof, ok := d["freshness_proof"].(map[string]interface{}); ok && len(opts.FreshnessProofIssuerKeys) > 0 {
		proofSig, has := signatureOf(proof, "signature")
		pc, err := canonicalOf(without(proof, []string{"signature"}))
		if err != nil {
			return DecisionVerification{}, err
		}
		if !has || !verifyEd25519(pc, proofSig, opts.FreshnessProofIssuerKeys) {
			return reject("freshness_proof_invalid_signature")
		}
		pvu, err1 := ParseTimestamp(str(proof, "proof_valid_until"))
		made, err2 := ParseTimestamp(str(d, "decision_made_at"))
		if err1 != nil || err2 != nil {
			return reject("freshness_proof_invalid_signature")
		}
		if pvu.Before(made) {
			return reject("freshness_proof_expired")
		}
	}

	binding, _ := d["policy_binding"].(map[string]interface{})
	if opts.ExpectedPolicies != nil {
		if binding == nil {
			return reject("policy_binding_missing")
		}
		type ref struct {
			id      string
			version json.Number
			digest  string
		}
		expected := make([]ref, 0, len(opts.ExpectedPolicies))
		for _, raw := range opts.ExpectedPolicies {
			p, err := decodeObject(raw, "expected policy")
			if err != nil {
				return DecisionVerification{}, err
			}
			digest, err := policyDigestOf(p)
			if err != nil {
				return DecisionVerification{}, err
			}
			v, _ := p["version"].(json.Number)
			expected = append(expected, ref{str(p, "policy_id"), v, digest})
		}
		sort.SliceStable(expected, func(i, j int) bool {
			if expected[i].id != expected[j].id {
				return expected[i].id < expected[j].id
			}
			vi, _ := expected[i].version.Int64()
			vj, _ := expected[j].version.Int64()
			return vi < vj
		})
		applied, _ := binding["policies"].([]interface{})
		refs := make([]AppliedPolicyRef, 0, len(applied))
		bound := make([]ref, 0, len(applied))
		for _, item := range applied {
			a, _ := item.(map[string]interface{})
			v, _ := a["version"].(json.Number)
			bound = append(bound, ref{str(a, "policy_id"), v, str(a, "policy_digest")})
			refs = append(refs, AppliedPolicyRef{str(a, "policy_id"), v, str(a, "policy_digest")})
		}
		same := len(expected) == len(bound)
		for i := 0; same && i < len(expected); i++ {
			same = expected[i].id == bound[i].id && expected[i].version.String() == bound[i].version.String() &&
				expected[i].digest == bound[i].digest
		}
		setDigest, err := PolicySetDigest(refs)
		if err != nil {
			return DecisionVerification{}, err
		}
		if !same || setDigest != str(binding, "policy_set_digest") {
			return reject("policy_binding_mismatch")
		}
	}

	if opts.ExpectedAttestation != nil {
		ab, _ := d["attestation_binding"].(map[string]interface{})
		if ab == nil {
			return reject("attestation_binding_missing")
		}
		att, err := decodeObject(opts.ExpectedAttestation, "expected attestation")
		if err != nil {
			return DecisionVerification{}, err
		}
		digest, err := AttestationDigest(opts.ExpectedAttestation)
		if err != nil {
			return DecisionVerification{}, err
		}
		if str(ab, "attestation_id") != str(att, "attestation_id") || ab["subject_id"] != att["subject_id"] ||
			ab["issuer_sovereign_id"] != att["issuer_sovereign_id"] || str(ab, "attestation_digest") != digest {
			return reject("attestation_binding_mismatch")
		}
	}

	if !authorized {
		gates, _ := d["gate_results"].([]interface{})
		builtinFailed := false
		for _, item := range gates {
			g, _ := item.(map[string]interface{})
			passed, _ := g["passed"].(bool)
			if passed {
				continue
			}
			if attestationGateNames[str(g, "gate_name")] {
				return result(true, "unauthorized_attestation_basis", false), nil
			}
			if builtinGateNames[str(g, "gate_name")] {
				builtinFailed = true
			}
		}
		if binding != nil && !builtinFailed {
			resolutionFailed := str(binding, "resolution_status") == "failed"
			enforceFailed := false
			evals, _ := binding["gate_evaluations"].([]interface{})
			for _, item := range evals {
				e, _ := item.(map[string]interface{})
				passed, _ := e["passed"].(bool)
				if str(e, "mode") == "enforce" && !passed {
					enforceFailed = true
				}
			}
			if resolutionFailed {
				return result(true, "unauthorized_policy_resolution_failed", false), nil
			}
			if enforceFailed {
				return result(true, "unauthorized_policy_gate_failure", false), nil
			}
		}
		denial, _ := d["denial_reason"].(string)
		switch {
		case strings.Contains(denial, "capability"):
			return result(true, "unauthorized_capability_out_of_scope", false), nil
		case strings.Contains(denial, "validity") || strings.Contains(denial, "window"):
			return result(true, "unauthorized_outside_validity_window", false), nil
		case strings.Contains(denial, "freshness"):
			return result(true, "unauthorized_insufficient_freshness", false), nil
		default:
			return result(true, "unauthorized_gate_failure", false), nil
		}
	}
	return result(true, "authorized", true), nil
}

// ── data usage ─────────────────────────────────────────────────────────────

// VerifyDataLicensePolicySignature reports whether the licensor signed the DataLicensePolicy.
func VerifyDataLicensePolicySignature(policyJSON []byte, licensorKeys []string) (bool, error) {
	p, err := decodeObject(policyJSON, "data license policy")
	if err != nil {
		return false, err
	}
	sig, ok := signatureOf(p, "signature")
	if !ok {
		return false, nil
	}
	c, err := canonicalOf(without(p, []string{"signature"}))
	if err != nil {
		return false, err
	}
	if hasUnknownFields("DataLicensePolicy", p) {
		return false, nil
	}
	return verifyEd25519(c, sig, licensorKeys), nil
}

// DataUsageViolation is one reason an intent is not compliant with a license policy.
type DataUsageViolation struct {
	ViolationType string
	Detail        string
}

// DataIntentVerification is the outcome of VerifyDataAccessIntent.
type DataIntentVerification struct {
	Valid           bool
	ViolationReason string // first violation type; "" when valid
	Violations      []DataUsageViolation
}

func stringList(v interface{}) []string {
	list, _ := v.([]interface{})
	out := make([]string, 0, len(list))
	for _, item := range list {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func toSet(list []string) map[string]bool {
	set := make(map[string]bool, len(list))
	for _, s := range list {
		set[s] = true
	}
	return set
}

// VerifyDataAccessIntent checks an agent-signed DataAccessIntent against a DataLicensePolicy
// at time `at` (zero: now): the agent signature, then expiry, licensed sources, prohibited
// classifications, permitted access types and the volume cap, in the reference order.
func VerifyDataAccessIntent(intentJSON, policyJSON []byte, agentKeys []string, at time.Time) (DataIntentVerification, error) {
	intent, err := decodeObject(intentJSON, "data access intent")
	if err != nil {
		return DataIntentVerification{}, err
	}
	policy, err := decodeObject(policyJSON, "data license policy")
	if err != nil {
		return DataIntentVerification{}, err
	}
	if at.IsZero() {
		at = time.Now()
	}
	fail := func(violations []DataUsageViolation) DataIntentVerification {
		return DataIntentVerification{Valid: false, ViolationReason: violations[0].ViolationType, Violations: violations}
	}
	// v1.2.0: fields this SDK does not know, as the reference reports them (strict.go).
	intentUnknown := unknownFieldsIn("DataAccessIntent", intent, "", true)
	if len(intentUnknown) > 0 {
		s, has := signatureOf(intent, "signature")
		c, err := canonicalOf(without(intent, []string{"signature"}))
		if err != nil {
			return DataIntentVerification{}, err
		}
		if !has || !verifyEd25519(c, s, agentKeys) {
			return fail([]DataUsageViolation{{"intent_exceeds_license", "Invalid intent signature"}}), nil
		}
	}
	unknown := append(intentUnknown, unknownFieldsIn("DataLicensePolicy", policy, "policy.", true)...)
	if len(unknown) > 0 {
		sort.Strings(unknown)
		return fail([]DataUsageViolation{{"intent_exceeds_license", "Unknown field: " + strings.Join(unknown, ", ")}}), nil
	}
	sig, ok := signatureOf(intent, "signature")
	if !ok {
		return fail([]DataUsageViolation{{"intent_exceeds_license", "Missing intent signature"}}), nil
	}
	c, err := canonicalOf(without(intent, []string{"signature"}))
	if err != nil {
		return DataIntentVerification{}, err
	}
	if !verifyEd25519(c, sig, agentKeys) {
		return fail([]DataUsageViolation{{"intent_exceeds_license", "Invalid intent signature"}}), nil
	}

	var violations []DataUsageViolation
	add := func(kind, detail string) { violations = append(violations, DataUsageViolation{kind, detail}) }
	expires, err := ParseTimestamp(str(intent, "expires_at"))
	if err != nil {
		return DataIntentVerification{}, err
	}
	if at.After(expires) {
		add("intent_expired", fmt.Sprintf("Intent expired at %s", str(intent, "expires_at")))
	}
	from, err1 := ParseTimestamp(str(policy, "valid_from"))
	until, err2 := ParseTimestamp(str(policy, "valid_until"))
	if err1 != nil || err2 != nil {
		return DataIntentVerification{}, fmt.Errorf("genesismesh: policy validity window is invalid")
	}
	if at.After(until) || at.Before(from) {
		add("policy_expired", "Policy not valid at verification time")
	}
	allowed := toSet(stringList(policy["allowed_source_ids"]))
	prohibited := toSet(stringList(policy["prohibited_classification_tags"]))
	sources, _ := intent["declared_sources"].([]interface{})
	for _, item := range sources {
		src, _ := item.(map[string]interface{})
		id := str(src, "source_id")
		if !allowed[id] {
			add("source_not_licensed", fmt.Sprintf("Source '%s' not in allowed_source_ids", id))
		}
		var overlap []string
		for _, tag := range stringList(src["classification_tags"]) {
			if prohibited[tag] {
				overlap = append(overlap, tag)
			}
		}
		if len(overlap) > 0 {
			sort.Strings(overlap)
			add("prohibited_classification", fmt.Sprintf("Source '%s' has prohibited tags: %v", id, overlap))
		}
	}
	allowedTypes := toSet(stringList(policy["allowed_access_types"]))
	if len(allowedTypes) == 0 {
		allowedTypes = map[string]bool{"read": true}
	}
	for _, accessType := range stringList(intent["declared_access_types"]) {
		if !allowedTypes[accessType] {
			add("access_type_not_permitted", fmt.Sprintf("Access type '%s' not in allowed_access_types", accessType))
		}
	}
	if limit, ok := policy["max_volume_bytes_per_session"].(json.Number); ok {
		if vol, ok := intent["estimated_volume_bytes"].(json.Number); ok {
			l, err1 := limit.Int64()
			v, err2 := vol.Int64()
			if err1 == nil && err2 == nil && v > l {
				add("volume_cap_exceeded", fmt.Sprintf("Volume %d > max %d", v, l))
			}
		}
	}
	if len(violations) > 0 {
		return fail(violations), nil
	}
	return DataIntentVerification{Valid: true}, nil
}
