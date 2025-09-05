package api

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

const (
	langfuseDefaultEndpoint = "https://cloud.langfuse.com"
	ingestionPath           = "/api/public/ingestion"
	defaultTimeout          = 30 * time.Second
)

type Client struct {
	httpClient *http.Client
	baseURL    string
	publicKey  string
	secretKey  string
}

func New() *Client {
	langfuseHost := os.Getenv("LANGFUSE_HOST")
	if langfuseHost == "" {
		langfuseHost = langfuseDefaultEndpoint
	}

	publicKey := os.Getenv("LANGFUSE_PUBLIC_KEY")
	secretKey := os.Getenv("LANGFUSE_SECRET_KEY")

	return &Client{
		httpClient: &http.Client{
			Timeout: defaultTimeout,
		},
		baseURL:   langfuseHost,
		publicKey: publicKey,
		secretKey: secretKey,
	}
}

func (c *Client) Ingestion(ctx context.Context, req *Ingestion, res *IngestionResponse) error {
	jsonData, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("failed to marshal request: %w", err)
	}

	url := c.baseURL + ingestionPath
	httpReq, reqErr := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if reqErr != nil {
		return fmt.Errorf("failed to create request: %w", reqErr)
	}

	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", c.basicAuth())

	resp, respErr := c.httpClient.Do(httpReq)
	if respErr != nil {
		return fmt.Errorf("failed to send request: %w", respErr)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			log.Printf("Failed to close response body: %v", closeErr)
		}
	}()

	body, bodyErr := io.ReadAll(resp.Body)
	if bodyErr != nil {
		return fmt.Errorf("failed to read response: %w", bodyErr)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return fmt.Errorf("unexpected status code %d: %s", resp.StatusCode, string(body))
	}

	if unmarshalErr := json.Unmarshal(body, res); unmarshalErr != nil {
		return fmt.Errorf("failed to unmarshal response: %w", unmarshalErr)
	}

	return nil
}

func (c *Client) basicAuth() string {
	auth := c.publicKey + ":" + c.secretKey
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(auth))
}
