// Package genesismesh provides a typed client for the Genesis Mesh
// Network Authority HTTP API.
package genesismesh

import "encoding/json"

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

// OfferRecord is returned by POST /admin/agreements/offer.
// Pass it to Agreement.Accept to complete the handshake.
type OfferRecord struct {
	OfferID              string          `json:"offer_id"`
	OffererSovereignID   string          `json:"offerer_sovereign_id"`
	ResponderSovereignID string          `json:"responder_sovereign_id"`
	RequestedTerms       json.RawMessage `json:"requested_terms,omitempty"`
	OffererEvidence      json.RawMessage `json:"offerer_evidence,omitempty"`
	Signatures           json.RawMessage `json:"signatures,omitempty"`
	GraphDigest          string          `json:"graph_digest,omitempty"`
	CreatedAt            string          `json:"created_at"`
	ExpiresAt            string          `json:"expires_at"`
}

// AgreementRecord is returned by POST /admin/agreements/accept and counter.
type AgreementRecord struct {
	AgreementID          string          `json:"agreement_id"`
	OfferID              string          `json:"offer_id,omitempty"`
	OffererSovereignID   string          `json:"offerer_sovereign_id"`
	ResponderSovereignID string          `json:"responder_sovereign_id"`
	Capabilities         []string        `json:"capabilities"`
	Roles                []string        `json:"roles"`
	Status               string          `json:"status"`
	Signatures           json.RawMessage `json:"signatures,omitempty"`
	CreatedAt            string          `json:"created_at"`
	ExpiresAt            string          `json:"expires_at"`
}

// BoundaryDecision is returned by POST /admin/boundary/decide.
type BoundaryDecision struct {
	DecisionID        string          `json:"decision_id"`
	AgreementID       string          `json:"agreement_id"`
	RequestingAgentID string          `json:"requesting_agent_id"`
	TargetAgentID     string          `json:"target_agent_id"`
	Capability        string          `json:"capability"`
	Allowed           bool            `json:"allowed"`
	Reason            string          `json:"reason"`
	Signature         json.RawMessage `json:"signature,omitempty"`
	IssuedAt          string          `json:"issued_at"`
}

// TrustDecision is the decision payload inside the Evidence.Build request.
// Evidence.Build wraps this in {"decision": ...} before posting.
type TrustDecision struct {
	SourceSovereignID string                 `json:"source_sovereign_id,omitempty"`
	TargetSovereignID string                 `json:"target_sovereign_id,omitempty"`
	SubjectID         string                 `json:"subject_id,omitempty"`
	Verdict           string                 `json:"verdict"` // "allow" | "block" | "escalate" | "warn"
	Reason            string                 `json:"reason"`
	Signals           []interface{}          `json:"signals,omitempty"`
	Context           map[string]interface{} `json:"context,omitempty"`
}

// TrustEvidence is returned by POST /admin/trust-evidence.
type TrustEvidence struct {
	EvidenceID string          `json:"evidence_id"`
	DecisionID string          `json:"decision_id"`
	SubjectID  string          `json:"subject_id"`
	Verdict    string          `json:"verdict"`
	Decision   json.RawMessage `json:"decision,omitempty"`
	Signature  json.RawMessage `json:"signature,omitempty"`
	IssuerID   string          `json:"issuer_id,omitempty"`
	IssuedAt   string          `json:"issued_at"`
}

// MembershipAttestation is returned by POST /admin/attestations.
type MembershipAttestation struct {
	AttestationID      string          `json:"attestation_id"`
	SubjectSovereignID string          `json:"subject_sovereign_id"`
	Roles              []string        `json:"roles"`
	Signature          json.RawMessage `json:"signature,omitempty"`
	IssuedAt           string          `json:"issued_at"`
	ExpiresAt          string          `json:"expires_at"`
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
	CommitmentID string          `json:"commitment_id"`
	MerkleRoot   string          `json:"merkle_root"`
	Signature    json.RawMessage `json:"signature,omitempty"`
	IssuedAt     string          `json:"issued_at"`
}

// CapabilityMembershipProof is returned by POST /disclosure/prove.
type CapabilityMembershipProof struct {
	CommitmentID string          `json:"commitment_id"`
	Capability   string          `json:"capability"`
	Proof        json.RawMessage `json:"proof,omitempty"`
	LeafHash     string          `json:"leaf_hash"`
}

// ConsensusVote is returned by POST /admin/consensus/vote.
type ConsensusVote struct {
	VoteID      string          `json:"vote_id"`
	ProposalID  string          `json:"proposal_id"`
	ValidatorID string          `json:"validator_id"`
	Decision    string          `json:"decision"`
	Signature   json.RawMessage `json:"signature,omitempty"`
	CastAt      string          `json:"cast_at"`
}

// ConsensusProof is returned by POST /admin/consensus/proof.
type ConsensusProof struct {
	ProofID     string          `json:"proof_id"`
	ProposalID  string          `json:"proposal_id"`
	Threshold   int             `json:"threshold"`
	Votes       []ConsensusVote `json:"votes"`
	Signature   json.RawMessage `json:"signature,omitempty"`
	AssembledAt string          `json:"assembled_at"`
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

// DataLicensePolicy is returned by POST /admin/data-usage/policy.
type DataLicensePolicy struct {
	PolicyID         string          `json:"policy_id"`
	LocalSovereignID string          `json:"local_sovereign_id"`
	AllowedPurposes  []string        `json:"allowed_purposes"`
	Signature        json.RawMessage `json:"signature,omitempty"`
	IssuedAt         string          `json:"issued_at"`
}

// DataAccessIntent is returned by POST /admin/data-usage/intent.
type DataAccessIntent struct {
	IntentID         string                 `json:"intent_id"`
	AgentSovereignID string                 `json:"agent_sovereign_id"`
	DecisionID       string                 `json:"decision_id"`
	Sources          []DataSourceDescriptor `json:"sources"`
	AccessTypes      []string               `json:"access_types"`
	PolicyID         string                 `json:"policy_id,omitempty"`
	IssuerID         string                 `json:"issuer_id,omitempty"`
	Signature        json.RawMessage        `json:"signature,omitempty"`
	IssuedAt         string                 `json:"issued_at"`
}

// VerifyResult is returned by public verify endpoints.
type VerifyResult struct {
	Valid  bool   `json:"valid"`
	Reason string `json:"reason,omitempty"`
}
