package walrus

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Client struct {
	apiURL string
	client *http.Client
}

type WriteResponse struct {
	BlobID string `json:"blob_id"`
	Status string `json:"status"`
}

type ReadResponse struct {
	Data string `json:"data"`
}

func NewClient(apiURL string) *Client {
	return &Client{
		apiURL: apiURL,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *Client) Write(ctx context.Context, data []byte) (string, error) {
	url := fmt.Sprintf("%s/v1/store", c.apiURL)

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/octet-stream")

	resp, err := c.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to write to Walrus: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return "", fmt.Errorf("Walrus write failed: status %d (body unreadable: %v)", resp.StatusCode, readErr)
		}
		return "", fmt.Errorf("Walrus write failed: status %d, body: %s", resp.StatusCode, string(body))
	}

	var result WriteResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	return result.BlobID, nil
}