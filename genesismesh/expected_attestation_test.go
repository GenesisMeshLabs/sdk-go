package genesismesh

import (
	"testing"
)

// 1.9.2: VerifyBoundaryDecision checks an expected attestation ID or subject ID
// without the attestation itself. The decisions are the reference's interop
// vectors: bd-003 is bound to an attestation, bd-001 to none.

func decisionVector(t *testing.T, id string) (vector, DecisionVerifyOptions) {
	t.Helper()
	for _, v := range loadVectors(t) {
		if v.ID != id {
			continue
		}
		now, err := ParseTimestamp(text(t, v.Input["now"]))
		if err != nil {
			t.Fatal(err)
		}
		return v, DecisionVerifyOptions{OperatorPublicKeys: strings_(t, v.Input["operator_public_keys"]), Now: now}
	}
	t.Fatalf("no vector %s", id)
	return vector{}, DecisionVerifyOptions{}
}

func TestExpectedAttestationAndSubjectIDs(t *testing.T) {
	bound, base := decisionVector(t, "bd-003")
	const attestationID = "00000000-0000-4000-8000-000000000003"
	const subjectID = "sovereign-b"

	cases := []struct {
		name          string
		attestationID string
		subjectID     string
		accepted      bool
		reason        string
	}{
		{"neither set", "", "", true, "authorized"},
		{"both match", attestationID, subjectID, true, "authorized"},
		{"attestation matches", attestationID, "", true, "authorized"},
		{"subject matches", "", subjectID, true, "authorized"},
		{"another attestation", "00000000-0000-4000-8000-000000000099", "", false, "attestation_binding_mismatch"},
		{"another subject", "", "sovereign-c", false, "attestation_binding_mismatch"},
		{"attestation matches, subject does not", attestationID, "sovereign-c", false, "attestation_binding_mismatch"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			opts := base
			opts.ExpectedAttestationID = c.attestationID
			opts.ExpectedSubjectID = c.subjectID
			res, err := VerifyBoundaryDecision(bound.Input["decision"], opts)
			if err != nil {
				t.Fatal(err)
			}
			if res.Accepted != c.accepted || res.Reason != c.reason {
				t.Fatalf("got %v %q, want %v %q", res.Accepted, res.Reason, c.accepted, c.reason)
			}
		})
	}
}

func TestExpectedIDsNeedAnAttestationBinding(t *testing.T) {
	unbound, opts := decisionVector(t, "bd-001")
	res, err := VerifyBoundaryDecision(unbound.Input["decision"], opts)
	if err != nil || !res.Accepted {
		t.Fatalf("control: the unbound decision verifies without expectations: %+v %v", res, err)
	}
	for _, o := range []DecisionVerifyOptions{
		{OperatorPublicKeys: opts.OperatorPublicKeys, Now: opts.Now, ExpectedAttestationID: "a"},
		{OperatorPublicKeys: opts.OperatorPublicKeys, Now: opts.Now, ExpectedSubjectID: "sovereign-b"},
	} {
		res, err := VerifyBoundaryDecision(unbound.Input["decision"], o)
		if err != nil {
			t.Fatal(err)
		}
		if res.Accepted || res.Reason != "attestation_binding_missing" {
			t.Fatalf("got %v %q, want attestation_binding_missing", res.Accepted, res.Reason)
		}
	}
}
