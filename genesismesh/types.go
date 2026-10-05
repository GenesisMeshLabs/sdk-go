// Package genesismesh provides a typed client for the Genesis Mesh
// Network Authority HTTP API.
package genesismesh

import "encoding/json"

// The response types below carry the Network Authority's own JSON field names.
// Before 1.0.2 several declared names the NA never sends, so those fields were
// always empty (and verify results always reported Valid == false). The old
// names are kept as deprecated fields, filled in when a response is decoded,
// and never serialized: marshaling a decoded record gives the NA's fields.

// CapabilityOffer is the request body for POST /admin/agreements/offer.
type CapabilityOffer struct {
	OfferorSovereignID   string            `json:"offeror_sovereign_id,omitempty"`
	ResponderSovereignID string            `json:"responder_sovereign_id"`
	Capabilities         []string          `json:"capabilities"`
	Roles                []string          `json:"roles"`
	ValidFrom            string            `json:"valid_from"`
	ValidUntil           string            `json:"valid_until"`
	ExpiresAt            string            `json:"expires_at"`
	Metadata             map[string]string `json:"metadata,omitempty"`
}

// OfferRecord is returned by POST /admin/agreements/offer (a CapabilityOffer)
// and by POST /admin/agreements/counter (a CapabilityCounter). Pass an offer to
// Agreement.Accept to complete the handshake. A counter carries AgreedTerms and
// ResponderEvidence where an offer carries RequestedTerms and CreatedAt.
type OfferRecord struct {
	OfferID              string          `json:"offer_id"`
	OffererSovereignID   string          `json:"offerer_sovereign_id"`
	ResponderSovereignID string          `json:"responder_sovereign_id"`
	RequestedTerms       json.RawMessage `json:"requested_terms,omitempty"`
	AgreedTerms          json.RawMessage `json:"agreed_terms,omitempty"`
	OffererEvidence      json.RawMessage `json:"offerer_evidence,omitempty"`
	ResponderEvidence    json.RawMessage `json:"responder_evidence,omitempty"`
	Signatures           json.RawMessage `json:"signatures,omitempty"`
	GraphDigest          string          `json:"graph_digest,omitempty"`
	CreatedAt            string          `json:"created_at,omitempty"`
	ExpiresAt            string          `json:"expires_at"`
}

// AgreementRecord is returned by POST /admin/agreements/accept: the agreement
// with the signatures of both parties.
type AgreementRecord struct {
	AgreementID          string          `json:"agreement_id"`
	OfferID              string          `json:"offer_id"`
	OffererSovereignID   string          `json:"offerer_sovereign_id"`
	ResponderSovereignID string          `json:"responder_sovereign_id"`
	AgreedTerms          json.RawMessage `json:"agreed_terms,omitempty"`
	OffererEvidence      json.RawMessage `json:"offerer_evidence,omitempty"`
	ResponderEvidence    json.RawMessage `json:"responder_evidence,omitempty"`
	GraphDigest          string          `json:"graph_digest"`
	EstablishedAt        string          `json:"established_at"`
	ExpiresAt            string          `json:"expires_at"`
	Signatures           json.RawMessage `json:"signatures,omitempty"`

	// Deprecated: the capabilities in AgreedTerms, filled in when decoding.
	Capabilities []string `json:"-"`
	// Deprecated: agreements carry no roles; always empty.
	Roles []string `json:"-"`
	// Deprecated: the NA sends no agreement status; always empty.
	Status string `json:"-"`
	// Deprecated: use EstablishedAt, which it is filled from.
	CreatedAt string `json:"-"`
}

// UnmarshalJSON decodes the NA's agreement and fills the deprecated fields.
func (a *AgreementRecord) UnmarshalJSON(data []byte) error {
	type wire AgreementRecord
	var w wire
	if err := json.Unmarshal(data, &w); err != nil {
		return err
	}
	*a = AgreementRecord(w)
	var terms struct {
		Capabilities []string `json:"capabilities"`
	}
	if len(a.AgreedTerms) > 0 && json.Unmarshal(a.AgreedTerms, &terms) == nil {
		a.Capabilities = terms.Capabilities
	}
	a.CreatedAt = a.EstablishedAt
	return nil
}

// BoundaryDecision is returned by POST /admin/boundary/decide: the decision
// the NA signed. Authorized is the answer; DenialReason says why when it is
// false.
type BoundaryDecision struct {
	DecisionID          string          `json:"decision_id"`
	ContextID           string          `json:"context_id"`
	AgreementID         string          `json:"agreement_id"`
	Authorized          bool            `json:"authorized"`
	DenialReason        *string         `json:"denial_reason"`
	GateResults         json.RawMessage `json:"gate_results,omitempty"`
	DecisionMadeAt      string          `json:"decision_made_at"`
	DecisionValidUntil  string          `json:"decision_valid_until"`
	OperatorSovereignID string          `json:"operator_sovereign_id"`
	FreshnessProof      json.RawMessage `json:"freshness_proof,omitempty"`
	PolicyBinding       json.RawMessage `json:"policy_binding,omitempty"`
	AttestationBinding  json.RawMessage `json:"attestation_binding,omitempty"`
	Signature           json.RawMessage `json:"signature,omitempty"`

	// Deprecated: use Authorized, which it is filled from.
	Allowed bool `json:"-"`
	// Deprecated: use DenialReason, which it is filled from ("" when authorized).
	Reason string `json:"-"`
	// Deprecated: use DecisionMadeAt, which it is filled from.
	IssuedAt string `json:"-"`
	// Deprecated: not part of a decision (they are in its ContextRecord); always empty.
	RequestingAgentID string `json:"-"`
	// Deprecated: not part of a decision; always empty.
	TargetAgentID string `json:"-"`
	// Deprecated: not part of a decision; always empty.
	Capability string `json:"-"`
}

// UnmarshalJSON decodes the NA's decision and fills the deprecated fields.
func (d *BoundaryDecision) UnmarshalJSON(data []byte) error {
	type wire BoundaryDecision
	var w wire
	if err := json.Unmarshal(data, &w); err != nil {
		return err
	}
	*d = BoundaryDecision(w)
	d.Allowed = d.Authorized
	if d.DenialReason != nil {
		d.Reason = *d.DenialReason
	}
	d.IssuedAt = d.DecisionMadeAt
	return nil
}

// TrustDecision is the decision payload inside the Evidence.Build request.
// Evidence.Build wraps this in {"decision": ...} before posting. The NA signs
// what it is given: set Trusted, HopCount and the other fields from the
// decision being recorded.
type TrustDecision struct {
	SourceSovereignID string                   `json:"source_sovereign_id,omitempty"`
	TargetSovereignID string                   `json:"target_sovereign_id,omitempty"`
	Verdict           string                   `json:"verdict"` // "allow" | "block" | "escalate" | "warn"
	Reason            string                   `json:"reason"`
	RequestedRoles    []string                 `json:"requested_roles,omitempty"`
	Trusted           bool                     `json:"trusted,omitempty"`
	TrustPath         []map[string]interface{} `json:"trust_path,omitempty"`
	HopCount          int                      `json:"hop_count,omitempty"`
	Signals           []interface{}            `json:"signals,omitempty"`
	EvaluatedAt       string                   `json:"evaluated_at,omitempty"`
	// Deprecated: not read by the NA.
	SubjectID string `json:"subject_id,omitempty"`
	// Deprecated: not read by the NA.
	Context map[string]interface{} `json:"context,omitempty"`
}

// TrustEvidence is returned by POST /admin/trust-evidence.
type TrustEvidence struct {
	EvidenceID        string          `json:"evidence_id"`
	IssuerSovereignID string          `json:"issuer_sovereign_id"`
	SourceSovereignID string          `json:"source_sovereign_id"`
	TargetSovereignID string          `json:"target_sovereign_id"`
	Verdict           string          `json:"verdict"`
	Reason            string          `json:"reason"`
	Trusted           bool            `json:"trusted"`
	HopCount          int             `json:"hop_count"`
	RequestedRoles    []string        `json:"requested_roles"`
	Signals           json.RawMessage `json:"signals,omitempty"`
	GraphDigest       string          `json:"graph_digest"`
	EvaluatedAt       string          `json:"evaluated_at"`
	IssuedAt          string          `json:"issued_at"`
	IssuedBy          string          `json:"issued_by"`
	Metadata          json.RawMessage `json:"metadata,omitempty"`
	Signatures        json.RawMessage `json:"signatures,omitempty"`

	// Deprecated: use IssuerSovereignID, which it is filled from.
	IssuerID string `json:"-"`
	// Deprecated: use TargetSovereignID, which it is filled from.
	SubjectID string `json:"-"`
	// Deprecated: the first entry of Signatures, filled in when decoding.
	Signature json.RawMessage `json:"-"`
	// Deprecated: trust evidence names no decision; always empty.
	DecisionID string `json:"-"`
	// Deprecated: trust evidence does not embed the decision; always empty.
	Decision json.RawMessage `json:"-"`
}

// UnmarshalJSON decodes the NA's evidence and fills the deprecated fields.
func (e *TrustEvidence) UnmarshalJSON(data []byte) error {
	type wire TrustEvidence
	var w wire
	if err := json.Unmarshal(data, &w); err != nil {
		return err
	}
	*e = TrustEvidence(w)
	e.IssuerID = e.IssuerSovereignID
	e.SubjectID = e.TargetSovereignID
	e.Signature = firstSignature(e.Signatures)
	return nil
}

// MembershipAttestation is returned by POST /admin/attestations.
type MembershipAttestation struct {
	AttestationID     string          `json:"attestation_id"`
	IssuerSovereignID string          `json:"issuer_sovereign_id"`
	SubjectID         string          `json:"subject_id"`
	SubjectPublicKey  *string         `json:"subject_public_key"`
	Roles             []string        `json:"roles"`
	Status            string          `json:"status"`
	IssuedAt          string          `json:"issued_at"`
	ValidFrom         string          `json:"valid_from"`
	ExpiresAt         string          `json:"expires_at"`
	IssuedBy          string          `json:"issued_by"`
	Claims            json.RawMessage `json:"claims,omitempty"`
	Signatures        json.RawMessage `json:"signatures,omitempty"`

	// Deprecated: use SubjectID, which it is filled from.
	SubjectSovereignID string `json:"-"`
	// Deprecated: the first entry of Signatures, filled in when decoding.
	Signature json.RawMessage `json:"-"`
}

// UnmarshalJSON decodes the NA's attestation and fills the deprecated fields.
func (m *MembershipAttestation) UnmarshalJSON(data []byte) error {
	type wire MembershipAttestation
	var w wire
	if err := json.Unmarshal(data, &w); err != nil {
		return err
	}
	*m = MembershipAttestation(w)
	m.SubjectSovereignID = m.SubjectID
	m.Signature = firstSignature(m.Signatures)
	return nil
}

// RecognizedIssuer is an entry in a RecognitionPolicy.
type RecognizedIssuer struct {
	IssuerSovereignID string   `json:"issuer_sovereign_id"`
	AllowedRoles      []string `json:"allowed_roles"`
}

// RecognitionPolicy is the body for POST /admin/recognition-policy.
type RecognitionPolicy struct {
	LocalSovereignID  string             `json:"local_sovereign_id"`
	RecognizedIssuers []RecognizedIssuer `json:"recognized_issuers"`
}

// CapabilityCommitment is returned by POST /admin/disclosure/commit.
type CapabilityCommitment struct {
	CommitmentID      string          `json:"commitment_id"`
	AgreementID       string          `json:"agreement_id"`
	IssuerSovereignID string          `json:"issuer_sovereign_id"`
	MerkleRoot        string          `json:"merkle_root"`
	CapabilityCount   int             `json:"capability_count"`
	CommittedAt       string          `json:"committed_at"`
	Signature         json.RawMessage `json:"signature,omitempty"`

	// Deprecated: use CommittedAt, which it is filled from.
	IssuedAt string `json:"-"`
}

// UnmarshalJSON decodes the NA's commitment and fills the deprecated fields.
func (c *CapabilityCommitment) UnmarshalJSON(data []byte) error {
	type wire CapabilityCommitment
	var w wire
	if err := json.Unmarshal(data, &w); err != nil {
		return err
	}
	*c = CapabilityCommitment(w)
	c.IssuedAt = c.CommittedAt
	return nil
}

// CapabilityMembershipProof is returned by POST /disclosure/prove.
type CapabilityMembershipProof struct {
	ProofID            string          `json:"proof_id"`
	CommitmentID       string          `json:"commitment_id"`
	ProverSovereignID  string          `json:"prover_sovereign_id"`
	RevealedCapability string          `json:"revealed_capability"`
	LeafHash           string          `json:"leaf_hash"`
	MerklePath         json.RawMessage `json:"merkle_path,omitempty"`
	ProvedAt           string          `json:"proved_at"`

	// Deprecated: use RevealedCapability, which it is filled from.
	Capability string `json:"-"`
	// Deprecated: use MerklePath, which it is filled from.
	Proof json.RawMessage `json:"-"`
}

// UnmarshalJSON decodes the NA's proof and fills the deprecated fields.
func (p *CapabilityMembershipProof) UnmarshalJSON(data []byte) error {
	type wire CapabilityMembershipProof
	var w wire
	if err := json.Unmarshal(data, &w); err != nil {
		return err
	}
	*p = CapabilityMembershipProof(w)
	p.Capability = p.RevealedCapability
	p.Proof = p.MerklePath
	return nil
}

// ConsensusVote is a validator's signed vote on a JustificationProof
// (Python ValidatorVote), returned by POST /admin/consensus/vote. Pass it back
// unchanged when assembling a proof: every field is covered by the signature.
type ConsensusVote struct {
	VoteID               string          `json:"vote_id"`
	ProofID              string          `json:"proof_id"`
	DecisionID           string          `json:"decision_id"`
	ValidatorSovereignID string          `json:"validator_sovereign_id"`
	Vote                 bool            `json:"vote"` // true approves
	Reason               *string         `json:"reason"`
	VotedAt              string          `json:"voted_at"`
	ContextDigest        *string         `json:"context_digest"` // required on approve votes (v0.38)
	Signature            json.RawMessage `json:"signature"`
}

// ConsensusProof is a K-of-N approval over a JustificationProof, signed by the
// assembler (Python ConsensusProof). Returned by POST /admin/consensus/proof.
type ConsensusProof struct {
	ConsensusID             string          `json:"consensus_id"`
	ProofID                 string          `json:"proof_id"`
	DecisionID              string          `json:"decision_id"`
	RequiredThreshold       int             `json:"required_threshold"` // K: distinct named validators
	ValidatorSovereignIDs   []string        `json:"validator_sovereign_ids"`
	Votes                   []ConsensusVote `json:"votes"`
	ReachedAt               string          `json:"reached_at"`
	ExpiresAt               string          `json:"expires_at"`
	CascadeAssessmentDigest *string         `json:"cascade_assessment_digest"`
	Signature               json.RawMessage `json:"signature"`
}

// ConsensusVerification is returned by POST /consensus/verify. Reason is one of
// valid, missing_signature, invalid_assembler_signature, threshold_not_met,
// invalid_vote_signature, unknown_validator_key, vote_not_in_validator_set,
// expired, proof_id_mismatch, cascade_detected, missing_context_digest.
type ConsensusVerification struct {
	Valid       bool    `json:"valid"`
	Reason      string  `json:"reason"`
	ConsensusID *string `json:"consensus_id"`
}

// DataSourceDescriptor describes a data source in a DataAccessIntent.
// All three fields are required by the NA; missing any returns HTTP 422.
type DataSourceDescriptor struct {
	SourceID             string   `json:"source_id"`
	SourceType           string   `json:"source_type"` // "personal"|"proprietary"|"public"|"synthetic"
	OwnerSovereignID     string   `json:"owner_sovereign_id"`
	ClassificationTags   []string `json:"classification_tags,omitempty"`
	EstimatedVolumeBytes *int64   `json:"estimated_volume_bytes,omitempty"`
}

// DataLicensePolicy is returned by POST /admin/data-usage/policy and
// GET /data-usage/policy.
type DataLicensePolicy struct {
	PolicyID                     string          `json:"policy_id"`
	LicensorSovereignID          string          `json:"licensor_sovereign_id"`
	LicenseeSovereignID          string          `json:"licensee_sovereign_id"`
	AllowedSourceIDs             []string        `json:"allowed_source_ids"`
	AllowedAccessTypes           []string        `json:"allowed_access_types"`
	ProhibitedClassificationTags []string        `json:"prohibited_classification_tags"`
	MaxVolumeBytesPerSession     *int64          `json:"max_volume_bytes_per_session"`
	ValidFrom                    string          `json:"valid_from"`
	ValidUntil                   string          `json:"valid_until"`
	Signature                    json.RawMessage `json:"signature,omitempty"`

	// Deprecated: use LicensorSovereignID, which it is filled from.
	LocalSovereignID string `json:"-"`
	// Deprecated: policies name no purposes (see AllowedAccessTypes); always empty.
	AllowedPurposes []string `json:"-"`
	// Deprecated: use ValidFrom, which it is filled from.
	IssuedAt string `json:"-"`
}

// UnmarshalJSON decodes the NA's policy and fills the deprecated fields.
func (p *DataLicensePolicy) UnmarshalJSON(data []byte) error {
	type wire DataLicensePolicy
	var w wire
	if err := json.Unmarshal(data, &w); err != nil {
		return err
	}
	*p = DataLicensePolicy(w)
	p.LocalSovereignID = p.LicensorSovereignID
	p.IssuedAt = p.ValidFrom
	return nil
}

// DataAccessIntent is returned by POST /admin/data-usage/intent.
type DataAccessIntent struct {
	IntentID             string                 `json:"intent_id"`
	AgentSovereignID     string                 `json:"agent_sovereign_id"`
	DecisionID           string                 `json:"decision_id"`
	DeclaredSources      []DataSourceDescriptor `json:"declared_sources"`
	DeclaredAccessTypes  []string               `json:"declared_access_types"`
	EstimatedVolumeBytes *int64                 `json:"estimated_volume_bytes"`
	DeclaredAt           string                 `json:"declared_at"`
	ExpiresAt            string                 `json:"expires_at"`
	Signature            json.RawMessage        `json:"signature,omitempty"`

	// Deprecated: use DeclaredSources, which it is filled from.
	Sources []DataSourceDescriptor `json:"-"`
	// Deprecated: use DeclaredAccessTypes, which it is filled from.
	AccessTypes []string `json:"-"`
	// Deprecated: use DeclaredAt, which it is filled from.
	IssuedAt string `json:"-"`
	// Deprecated: intents name no policy; always empty.
	PolicyID string `json:"-"`
	// Deprecated: intents name no issuer (see AgentSovereignID); always empty.
	IssuerID string `json:"-"`
}

// UnmarshalJSON decodes the NA's intent and fills the deprecated fields.
func (i *DataAccessIntent) UnmarshalJSON(data []byte) error {
	type wire DataAccessIntent
	var w wire
	if err := json.Unmarshal(data, &w); err != nil {
		return err
	}
	*i = DataAccessIntent(w)
	i.Sources = i.DeclaredSources
	i.AccessTypes = i.DeclaredAccessTypes
	i.IssuedAt = i.DeclaredAt
	return nil
}

// VerifyResult is returned by the public verify endpoints. Agreement, boundary
// and trust-evidence verification answer "accepted"; disclosure and data-usage
// verification answer "valid". Valid and Accepted are both true when the NA
// accepted the material, whichever name it used. The other fields are set by
// the endpoints that send them.
type VerifyResult struct {
	Valid    bool   `json:"valid"`
	Accepted bool   `json:"accepted"`
	Reason   string `json:"reason,omitempty"`

	// Boundary verification: whether the verified decision authorizes the action.
	Authorized *bool `json:"authorized,omitempty"`

	AgreementID       *string `json:"agreement_id,omitempty"`
	DecisionID        *string `json:"decision_id,omitempty"`
	EvidenceID        *string `json:"evidence_id,omitempty"`
	CommitmentID      *string `json:"commitment_id,omitempty"`
	IssuerSovereignID *string `json:"issuer_sovereign_id,omitempty"`
	Verdict           *string `json:"verdict,omitempty"`

	// Data-usage verification: the violations found, if any. Reason is set
	// from ViolationReason.
	ViolationCount  *int            `json:"violation_count,omitempty"`
	ViolationReason *string         `json:"violation_reason,omitempty"`
	Violations      json.RawMessage `json:"violations,omitempty"`
}

// UnmarshalJSON decodes a verify answer, under either name for the result.
func (v *VerifyResult) UnmarshalJSON(data []byte) error {
	type wire VerifyResult
	var w wire
	if err := json.Unmarshal(data, &w); err != nil {
		return err
	}
	*v = VerifyResult(w)
	ok := v.Valid || v.Accepted
	v.Valid, v.Accepted = ok, ok
	if v.Reason == "" && v.ViolationReason != nil {
		v.Reason = *v.ViolationReason
	}
	return nil
}

// firstSignature returns the first entry of a signatures array, or nil.
func firstSignature(signatures json.RawMessage) json.RawMessage {
	var list []json.RawMessage
	if len(signatures) == 0 || json.Unmarshal(signatures, &list) != nil || len(list) == 0 {
		return nil
	}
	return list[0]
}
