package genesismesh

import (
	"encoding/json"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// The response types must model exactly what the Network Authority sends.
// testdata/contract/na_responses.json holds a live NA's response to each SDK
// call (captured by the live contract check); each one is decoded into the
// type the SDK method returns. Before 1.0.2 most of these types declared field
// names the NA never sends, so their fields were always empty.

func loadNAResponses(t *testing.T) map[string]json.RawMessage {
	t.Helper()
	data, err := os.ReadFile("testdata/contract/na_responses.json")
	if err != nil {
		t.Fatal(err)
	}
	var responses map[string]json.RawMessage
	if err := json.Unmarshal(data, &responses); err != nil {
		t.Fatal(err)
	}
	return responses
}

// jsonFieldsOf returns a struct's JSON names: true when required (no omitempty).
func jsonFieldsOf(typ reflect.Type) map[string]bool {
	fields := map[string]bool{}
	for i := 0; i < typ.NumField(); i++ {
		tag := typ.Field(i).Tag.Get("json")
		name := strings.Split(tag, ",")[0]
		if name != "" && name != "-" {
			fields[name] = !strings.Contains(tag, "omitempty")
		}
	}
	return fields
}

func keysOf(t *testing.T, raw json.RawMessage) map[string]bool {
	t.Helper()
	var obj map[string]json.RawMessage
	if err := json.Unmarshal(raw, &obj); err != nil {
		t.Fatal(err)
	}
	keys := map[string]bool{}
	for k := range obj {
		keys[k] = true
	}
	return keys
}

func sortedKeys(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func TestResponseTypesModelTheNAResponses(t *testing.T) {
	responses := loadNAResponses(t)
	cases := []struct {
		response string
		target   interface{}
	}{
		{"agreement_offer", &OfferRecord{}},
		{"agreement_counter", &OfferRecord{}},
		{"agreement_accept", &AgreementRecord{}},
		{"boundary_decide", &BoundaryDecision{}},
		{"evidence_build", &TrustEvidence{}},
		{"disclosure_commit", &CapabilityCommitment{}},
		{"disclosure_prove", &CapabilityMembershipProof{}},
		{"data_usage_policy", &DataLicensePolicy{}},
		{"data_usage_intent", &DataAccessIntent{}},
		{"consensus_vote", &ConsensusVote{}},
		{"consensus_proof", &ConsensusProof{}},
		{"consensus_verify", &ConsensusVerification{}},
		{"attestation_issue", &MembershipAttestation{}},
	}
	for _, tc := range cases {
		t.Run(tc.response, func(t *testing.T) {
			raw, ok := responses[tc.response]
			if !ok {
				t.Fatalf("no captured response %q", tc.response)
			}
			if err := json.Unmarshal(raw, tc.target); err != nil {
				t.Fatal(err)
			}
			sent := keysOf(t, raw)
			declared := jsonFieldsOf(reflect.TypeOf(tc.target).Elem())
			for key := range sent {
				if _, ok := declared[key]; !ok {
					t.Errorf("the NA sends %q, which %T does not model", key, tc.target)
				}
			}
			for name, required := range declared {
				if required && !sent[name] {
					t.Errorf("%T declares %q, which the NA never sends", tc.target, name)
				}
			}
			// Marshaling the decoded record gives back the NA's fields, and
			// none of the deprecated names.
			again, err := json.Marshal(tc.target)
			if err != nil {
				t.Fatal(err)
			}
			if got := keysOf(t, again); !reflect.DeepEqual(sortedKeys(got), sortedKeys(sent)) {
				t.Errorf("re-marshaled keys %v, want %v", sortedKeys(got), sortedKeys(sent))
			}
		})
	}
}

func TestVerifyResultReadsEitherAnswer(t *testing.T) {
	responses := loadNAResponses(t)
	for _, name := range []string{"agreement_verify", "boundary_verify", "evidence_verify", "disclosure_verify", "data_usage_verify"} {
		t.Run(name, func(t *testing.T) {
			raw := responses[name]
			var result VerifyResult
			if err := json.Unmarshal(raw, &result); err != nil {
				t.Fatal(err)
			}
			if !result.Valid || !result.Accepted {
				t.Errorf("valid = %v, accepted = %v; the NA accepted: %s", result.Valid, result.Accepted, raw)
			}
			declared := jsonFieldsOf(reflect.TypeOf(result))
			for key := range keysOf(t, raw) {
				if _, ok := declared[key]; !ok {
					t.Errorf("the NA sends %q, which VerifyResult does not model", key)
				}
			}
		})
	}
	var boundary VerifyResult
	if err := json.Unmarshal(responses["boundary_verify"], &boundary); err != nil {
		t.Fatal(err)
	}
	if boundary.Authorized == nil || !*boundary.Authorized || boundary.DecisionID == nil {
		t.Errorf("boundary verify: authorized = %v, decision_id = %v", boundary.Authorized, boundary.DecisionID)
	}
	rejected := VerifyResult{}
	if err := json.Unmarshal([]byte(`{"accepted": false, "reason": "invalid_signature"}`), &rejected); err != nil {
		t.Fatal(err)
	}
	if rejected.Valid || rejected.Accepted || rejected.Reason != "invalid_signature" {
		t.Errorf("rejected answer read as %+v", rejected)
	}
	violation := VerifyResult{}
	if err := json.Unmarshal([]byte(`{"valid": false, "violation_count": 1, "violation_reason": "source_not_allowed", "violations": [{}]}`), &violation); err != nil {
		t.Fatal(err)
	}
	if violation.Valid || violation.Reason != "source_not_allowed" || violation.ViolationCount == nil || *violation.ViolationCount != 1 {
		t.Errorf("data-usage violation read as %+v", violation)
	}
}

func TestDeprecatedFieldsAreFilledFromTheNAFields(t *testing.T) {
	responses := loadNAResponses(t)

	var decision BoundaryDecision
	if err := json.Unmarshal(responses["boundary_decide"], &decision); err != nil {
		t.Fatal(err)
	}
	if !decision.Authorized || !decision.Allowed || decision.IssuedAt != decision.DecisionMadeAt || decision.IssuedAt == "" {
		t.Errorf("decision: authorized = %v, allowed = %v, issued_at = %q", decision.Authorized, decision.Allowed, decision.IssuedAt)
	}

	var agreement AgreementRecord
	if err := json.Unmarshal(responses["agreement_accept"], &agreement); err != nil {
		t.Fatal(err)
	}
	if len(agreement.Capabilities) == 0 || agreement.CreatedAt != agreement.EstablishedAt {
		t.Errorf("agreement: capabilities = %v, created_at = %q", agreement.Capabilities, agreement.CreatedAt)
	}

	var attestation MembershipAttestation
	if err := json.Unmarshal(responses["attestation_issue"], &attestation); err != nil {
		t.Fatal(err)
	}
	if attestation.SubjectSovereignID != attestation.SubjectID || attestation.SubjectID == "" || len(attestation.Signature) == 0 {
		t.Errorf("attestation: subject %q / %q, signature %s", attestation.SubjectSovereignID, attestation.SubjectID, attestation.Signature)
	}

	var evidence TrustEvidence
	if err := json.Unmarshal(responses["evidence_build"], &evidence); err != nil {
		t.Fatal(err)
	}
	if evidence.IssuerID != evidence.IssuerSovereignID || evidence.SubjectID != evidence.TargetSovereignID || len(evidence.Signature) == 0 {
		t.Errorf("evidence: issuer %q, subject %q, signature %s", evidence.IssuerID, evidence.SubjectID, evidence.Signature)
	}

	var proof CapabilityMembershipProof
	if err := json.Unmarshal(responses["disclosure_prove"], &proof); err != nil {
		t.Fatal(err)
	}
	if proof.Capability != proof.RevealedCapability || proof.Capability == "" || len(proof.Proof) == 0 {
		t.Errorf("proof: capability %q, proof %s", proof.Capability, proof.Proof)
	}

	var commitment CapabilityCommitment
	if err := json.Unmarshal(responses["disclosure_commit"], &commitment); err != nil {
		t.Fatal(err)
	}
	if commitment.IssuedAt != commitment.CommittedAt || commitment.IssuedAt == "" {
		t.Errorf("commitment: issued_at %q", commitment.IssuedAt)
	}

	var policy DataLicensePolicy
	if err := json.Unmarshal(responses["data_usage_policy"], &policy); err != nil {
		t.Fatal(err)
	}
	if policy.LocalSovereignID != policy.LicensorSovereignID || policy.LocalSovereignID == "" {
		t.Errorf("policy: local %q, licensor %q", policy.LocalSovereignID, policy.LicensorSovereignID)
	}

	var intent DataAccessIntent
	if err := json.Unmarshal(responses["data_usage_intent"], &intent); err != nil {
		t.Fatal(err)
	}
	if len(intent.Sources) != len(intent.DeclaredSources) || len(intent.Sources) == 0 || len(intent.AccessTypes) == 0 {
		t.Errorf("intent: sources %v, access types %v", intent.Sources, intent.AccessTypes)
	}
}
