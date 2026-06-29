package genesismesh

// Client is the entry point for the Genesis Mesh Go SDK.
// It exposes one sub-client per Trust API domain.
//
//	client, err := genesismesh.NewClient(genesismesh.ClientOptions{
//	    BaseURL:    "http://127.0.0.1:9443",
//	    SigningKey: os.Getenv("OPERATOR_KEY"),
//	    KeyID:      "operator-local",
//	})
type Client struct {
	Agreement   *AgreementClient
	Attestation *AttestationClient
	Boundary    *BoundaryClient
	Consensus   *ConsensusClient
	DataUsage   *DataUsageClient
	Disclosure  *DisclosureClient
	Evidence    *EvidenceClient

	transport *transport
}

// NewClient constructs a Client from the given options.
// SigningKey and KeyID are optional when only calling public verify endpoints.
func NewClient(opts ClientOptions) (*Client, error) {
	t, err := newTransport(opts)
	if err != nil {
		return nil, err
	}
	return &Client{
		Agreement:   &AgreementClient{t: t},
		Attestation: &AttestationClient{t: t},
		Boundary:    &BoundaryClient{t: t},
		Consensus:   &ConsensusClient{t: t},
		DataUsage:   &DataUsageClient{t: t},
		Disclosure:  &DisclosureClient{t: t},
		Evidence:    &EvidenceClient{t: t},
		transport:   t,
	}, nil
}
