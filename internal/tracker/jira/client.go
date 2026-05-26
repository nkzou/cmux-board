package jira

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

const (
	defaultRequestTimeout = 30 * time.Second
	maxRetryAfter         = 5 * time.Minute
	defaultRetryAfter     = 60 * time.Second
)

// jiraClient is a thin HTTP transport layer for the Jira Cloud REST API.
// It injects Basic auth on every request and enforces per-request timeouts.
// HTTP headers are NEVER logged — flat ban, no exceptions.
type jiraClient struct {
	httpClient *http.Client
	baseURL    string // e.g. "https://datadog.atlassian.net"
	creds      Credentials
}

// newClient constructs a jiraClient using http.DefaultClient.
// Tests may wrap this to inject a custom *http.Client.
func newClient(creds Credentials) *jiraClient {
	return &jiraClient{
		httpClient: http.DefaultClient,
		baseURL:    "https://" + creds.Site,
		creds:      creds,
	}
}

// newClientWithHTTP constructs a jiraClient with a custom *http.Client (for testing).
func newClientWithHTTP(creds Credentials, hc *http.Client) *jiraClient {
	return &jiraClient{
		httpClient: hc,
		baseURL:    "https://" + creds.Site,
		creds:      creds,
	}
}

// Do executes an HTTP request against the Jira API.
// It injects Authorization, Content-Type, and Accept headers, enforces a 30s timeout,
// and handles Retry-After on HTTP 429/503 by returning a typed errRetryable.
//
// SECURITY: HTTP headers are NEVER logged at any level. Body-level logging is permitted
// since Jira REST bodies do not carry credentials.
func (c *jiraClient) Do(ctx context.Context, method, path string, body io.Reader) (*http.Response, error) {
	ctx, cancel := context.WithTimeout(ctx, defaultRequestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return nil, fmt.Errorf("failed to build request: %w", err)
	}

	// Inject auth and content negotiation headers.
	// NEVER log these header values — flat ban per CONVENTIONS.md.
	req.Header.Set("Authorization", "Basic "+buildBasicAuth(c.creds.Email, c.creds.APIToken))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	slog.Debug("jira request", "method", method, "path", path)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}

	slog.Debug("jira response", "status", resp.StatusCode)

	// Handle Retry-After on 429 / 503 by returning a typed retryable error.
	// The caller retries at its own discretion; we do NOT sleep here.
	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode == http.StatusServiceUnavailable {
		delay := retryAfterFromHeader(resp)
		resp.Body.Close()
		return nil, &errRetryable{statusCode: resp.StatusCode, retryAfter: delay}
	}

	return resp, nil
}
