package gateway

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// NLPClient calls the external NLP intent parsing service.
type NLPClient struct {
	endpoint string
	client   *http.Client
}

// NLPParseRequest is the request body for intent parsing.
type NLPParseRequest struct {
	Text string `json:"text"`
}

// NLPParseResponse is the parsed intent response.
type NLPParseResponse struct {
	Action string            `json:"action"`
	Params map[string]string `json:"params"`
	Error  string            `json:"error,omitempty"`
}

// NewNLPClient creates a new NLP client pointing to the given endpoint.
func NewNLPClient(endpoint string) *NLPClient {
	return &NLPClient{
		endpoint: endpoint,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// ParseIntent calls the NLP service to parse natural language into structured intent.
func (c *NLPClient) ParseIntent(ctx context.Context, text string) (*NLPParseResponse, error) {
	reqBody, err := json.Marshal(NLPParseRequest{Text: text})
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.endpoint+"/parse", bytes.NewReader(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call NLP service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("NLP service returned status %d", resp.StatusCode)
	}

	var result NLPParseResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &result, nil
}
