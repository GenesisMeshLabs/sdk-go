//go:build live

package genesismesh

// Live contract check: every client method against a running Network
// Authority, each typed result compared with the JSON the NA sent.
//
//	GM_E2E_PYTHON=python go test -tags live -count=1 -run TestLiveContract ./genesismesh
//
// The Python environment needs the genesis-mesh core (scripts/contract_na.py
// starts a disposable NA). With GM_CONTRACT_FIXTURE set to a path, the NA's
// responses are also written there, to refresh testdata/contract/na_responses.json.

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"os/exec"
	"reflect"
	"sync"
	"testing"
	"time"
)

type recordingTransport struct {
	mu    sync.Mutex
	inner http.RoundTripper
	last  []byte
}

func (r *recordingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := r.inner.RoundTrip(req)
	if err != nil {
		return resp, err
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	resp.Body = io.NopCloser(bytes.NewReader(body))
	if req.URL.Path != "/sovereign.json" {
		r.mu.Lock()
		r.last = body
		r.mu.Unlock()
	}
	return resp, nil
}

func (r *recordingTransport) body() json.RawMessage {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append(json.RawMessage(nil), r.last...)
}

func TestLiveContract(t *testing.T) {
	python := os.Getenv("GM_E2E_PYTHON")
	if python == "" {
		t.Skip("GM_E2E_PYTHON is not set")
	}
	na := exec.Command(python, "../scripts/contract_na.py")
	na.Stderr = os.Stderr
	stdout, err := na.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := na.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = na.Process.Kill(); _ = na.Wait() })
	line, err := bufio.NewReader(stdout).ReadBytes('\n')
	if err != nil {
		t.Fatalf("the NA did not start: %v", err)
	}
	var in struct {
		BaseURL            string                 `json:"baseUrl"`
		Seed               string                 `json:"seed"`
		KeyID              string                 `json:"keyId"`
		NAPublicKey        string                 `json:"naPublicKey"`
		NetworkName        string                 `json:"networkName"`
		JustificationProof map[string]interface{} `json:"justificationProof"`
	}
	if err := json.Unmarshal(line, &in); err != nil {
		t.Fatalf("NA output %q: %v", line, err)
	}

	c, err := NewClient(ClientOptions{BaseURL: in.BaseURL, SigningKey: in.Seed, KeyID: in.KeyID})
	if err != nil {
		t.Fatal(err)
	}
	rec := &recordingTransport{inner: http.DefaultTransport}
	c.Agreement.t.httpClient.Transport = rec
	ctx := context.Background()
	now := time.Now().UTC()
	iso := func(d time.Duration) string { return now.Add(d).Format(time.RFC3339) }
	responses := map[string]json.RawMessage{}

	// check records the NA's response to the call just made and compares the
	// typed result with it: every key the NA sent is modeled, and every
	// required field of the type was sent.
	check := func(name string, typed interface{}, callErr error) map[string]interface{} {
		t.Helper()
		if callErr != nil {
			t.Fatalf("%s: %v", name, callErr)
		}
		body := rec.body()
		responses[name] = body
		raw := map[string]interface{}{}
		if err := json.Unmarshal(body, &raw); err != nil {
			t.Fatalf("%s: response %s: %v", name, body, err)
		}
		if typed == nil {
			return raw
		}
		declared := jsonFieldsOf(reflect.TypeOf(typed).Elem())
		for key := range raw {
			if _, ok := declared[key]; !ok {
				t.Errorf("%s: the NA sends %q, which %T does not model", name, key, typed)
			}
		}
		if _, isVerify := typed.(*VerifyResult); !isVerify {
			for field, required := range declared {
				if _, ok := raw[field]; required && !ok {
					t.Errorf("%s: %T declares %q, which the NA did not send", name, typed, field)
				}
			}
		}
		return raw
	}
	accepted := func(name string, result *VerifyResult) {
		t.Helper()
		if !result.Valid || !result.Accepted {
			t.Errorf("%s: valid = %v, accepted = %v; the NA answered %s", name, result.Valid, result.Accepted, responses[name])
		}
	}

	offer, err := c.Agreement.Offer(ctx, CapabilityOffer{ResponderSovereignID: "sovereign-b",
		Capabilities: []string{"read", "write"}, ValidFrom: iso(0), ValidUntil: iso(24 * time.Hour), ExpiresAt: iso(time.Hour)})
	rawOffer := check("agreement_offer", offer, err)
	counter, err := c.Agreement.Counter(ctx, map[string]interface{}{"offer": rawOffer, "capabilities": []string{"read"},
		"scope": map[string]interface{}{}, "valid_from": iso(0), "valid_until": iso(12 * time.Hour)})
	check("agreement_counter", counter, err)
	agreement, err := c.Agreement.Accept(ctx, offer)
	rawAgreement := check("agreement_accept", agreement, err)
	verified, err := c.Agreement.Verify(ctx, map[string]interface{}{"agreement": rawAgreement})
	check("agreement_verify", verified, err)
	accepted("agreement_verify", verified)

	decision, err := c.Boundary.Decide(ctx, map[string]interface{}{"agreement": rawAgreement, "requested_capability": "read"})
	rawDecision := check("boundary_decide", decision, err)
	if decision.Authorized != (rawDecision["authorized"] == true) || decision.Allowed != decision.Authorized {
		t.Errorf("boundary_decide: authorized = %v, allowed = %v; the NA sent %v", decision.Authorized, decision.Allowed, rawDecision["authorized"])
	}
	decisionCheck, err := c.Boundary.Verify(ctx, map[string]interface{}{"decision": rawDecision})
	check("boundary_verify", decisionCheck, err)
	accepted("boundary_verify", decisionCheck)

	evidence, err := c.Evidence.Build(ctx, TrustDecision{SourceSovereignID: in.NetworkName, TargetSovereignID: "sovereign-b",
		Verdict: "allow", Reason: "direct recognition", Trusted: true, HopCount: 1})
	rawEvidence := check("evidence_build", evidence, err)
	if !evidence.Trusted || evidence.HopCount != 1 {
		t.Errorf("evidence_build: trusted = %v, hop_count = %d; want the decision's true and 1", evidence.Trusted, evidence.HopCount)
	}
	evidenceCheck, err := c.Evidence.Verify(ctx, map[string]interface{}{"evidence": rawEvidence})
	check("evidence_verify", evidenceCheck, err)
	accepted("evidence_verify", evidenceCheck)

	commitment, err := c.Disclosure.Commit(ctx, map[string]interface{}{"capabilities": []string{"read", "write"}, "agreement": rawAgreement})
	rawCommitment := check("disclosure_commit", commitment, err)
	proof, err := c.Disclosure.Prove(ctx, map[string]interface{}{"capability": "read", "capabilities": []string{"read", "write"},
		"commitment": rawCommitment, "prover_sovereign_id": "sovereign-b"})
	rawProof := check("disclosure_prove", proof, err)
	proofCheck, err := c.Disclosure.Verify(ctx, map[string]interface{}{"proof": rawProof, "commitment": rawCommitment})
	check("disclosure_verify", proofCheck, err)
	accepted("disclosure_verify", proofCheck)
	_, err = c.Disclosure.Nullifier(ctx, map[string]interface{}{"proof": rawProof})
	check("disclosure_nullifier", nil, err)

	policy, err := c.DataUsage.CreatePolicy(ctx, map[string]interface{}{"licensee_sovereign_id": "sovereign-b",
		"allowed_source_ids": []string{"src-1"}, "allowed_access_types": []string{"read"},
		"valid_from": iso(0), "valid_until": iso(720 * time.Hour)})
	rawPolicy := check("data_usage_policy", policy, err)
	active, err := c.DataUsage.GetPolicy(ctx)
	check("data_usage_get_policy", active, err)
	intent, err := c.DataUsage.CreateIntent(ctx, map[string]interface{}{"sources": []map[string]interface{}{{"source_id": "src-1",
		"source_type": "public", "owner_sovereign_id": in.NetworkName, "classification_tags": []string{}}},
		"access_types": []string{"read"}, "decision_id": "dec-001"})
	rawIntent := check("data_usage_intent", intent, err)
	usage, err := c.DataUsage.Verify(ctx, map[string]interface{}{"intent": rawIntent, "policy": rawPolicy})
	check("data_usage_verify", usage, err)
	accepted("data_usage_verify", usage)

	vote, err := c.Consensus.Vote(ctx, map[string]interface{}{"justification_proof": in.JustificationProof, "vote": true, "reason": "contract"})
	rawVote := check("consensus_vote", vote, err)
	consensus, err := c.Consensus.Proof(ctx, map[string]interface{}{"justification_proof": in.JustificationProof,
		"votes": []interface{}{rawVote}, "required_threshold": 1, "validator_sovereign_ids": []string{in.NetworkName}})
	rawConsensus := check("consensus_proof", consensus, err)
	consensusCheck, err := c.Consensus.Verify(ctx, map[string]interface{}{"proof": rawConsensus,
		"validator_public_keys": map[string]string{in.NetworkName: in.NAPublicKey}})
	check("consensus_verify", consensusCheck, err)
	if !consensusCheck.Valid {
		t.Errorf("consensus_verify: valid = false; the NA answered %s", responses["consensus_verify"])
	}

	attestation, err := c.Attestation.Issue(ctx, map[string]interface{}{"subject_id": "vendor-contract", "roles": []string{"role:client"}})
	check("attestation_issue", attestation, err)
	if attestation.SubjectID != "vendor-contract" || attestation.SubjectSovereignID != attestation.SubjectID {
		t.Errorf("attestation_issue: subject_id = %q, subject_sovereign_id = %q", attestation.SubjectID, attestation.SubjectSovereignID)
	}
	err = c.Attestation.SavePolicy(ctx, map[string]interface{}{"recognition_policy": map[string]interface{}{
		"local_sovereign_id": in.NetworkName, "recognized_issuers": []map[string]interface{}{{"sovereign_id": in.NetworkName,
			"public_keys": []string{in.NAPublicKey}, "allowed_roles": []string{"role:client"}, "accepted_statuses": []string{"active"}}},
		"revoked_attestation_ids": []string{}}})
	check("attestation_save_policy", nil, err)
	err = c.Attestation.Revoke(ctx, attestation.AttestationID, map[string]interface{}{"reason": "contract"})
	check("attestation_revoke", nil, err)

	if path := os.Getenv("GM_CONTRACT_FIXTURE"); path != "" {
		data, _ := json.MarshalIndent(responses, "", "  ")
		if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}
