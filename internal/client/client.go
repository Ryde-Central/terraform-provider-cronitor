// Package client is a small HTTP client for the Cronitor REST API.
//
// API reference: https://cronitor.io/docs/api
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	// DefaultBaseURL is the root of the Cronitor REST API.
	DefaultBaseURL = "https://cronitor.io/api"

	// APIVersion is the dated API version this client speaks. It is sent with
	// every request in the Cronitor-Version header; the request and response
	// shapes in this package (for example monitor "schedules") depend on it.
	APIVersion = "2025-11-28"

	maxRetries = 4
)

// Client talks to the Cronitor REST API.
type Client struct {
	baseURL    *url.URL
	apiKey     string
	userAgent  string
	httpClient *http.Client
}

// Option customises a Client.
type Option func(*Client)

// WithHTTPClient sets the underlying HTTP client.
func WithHTTPClient(h *http.Client) Option {
	return func(c *Client) { c.httpClient = h }
}

// WithUserAgent sets the User-Agent header sent with each request.
func WithUserAgent(ua string) Option {
	return func(c *Client) { c.userAgent = ua }
}

// New returns a Client for the API rooted at baseURL, authenticating with apiKey.
func New(baseURL, apiKey string, opts ...Option) (*Client, error) {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	u, err := url.Parse(strings.TrimRight(baseURL, "/"))
	if err != nil {
		return nil, fmt.Errorf("invalid base URL %q: %w", baseURL, err)
	}
	if u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("invalid base URL %q: scheme and host are required", baseURL)
	}
	if apiKey == "" {
		return nil, errors.New("an API key is required")
	}
	c := &Client{
		baseURL:    u,
		apiKey:     apiKey,
		userAgent:  "terraform-provider-cronitor",
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c, nil
}

// APIError is returned for any non-2xx response.
type APIError struct {
	StatusCode int
	Method     string
	Path       string
	Body       string
}

func (e *APIError) Error() string {
	body := strings.TrimSpace(e.Body)
	if body == "" {
		body = http.StatusText(e.StatusCode)
	}
	return fmt.Sprintf("cronitor: %s %s returned %d: %s", e.Method, e.Path, e.StatusCode, body)
}

// IsNotFound reports whether err is an API 404.
func IsNotFound(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.StatusCode == http.StatusNotFound
}

// do sends a request and decodes a JSON response into out (if non-nil).
// Rate-limited (429) and transient gateway errors are retried with backoff.
func (c *Client) do(ctx context.Context, method, path string, query url.Values, body, out any) error {
	// path is expected to have its dynamic segments escaped already.
	target := c.baseURL.String() + path
	if len(query) > 0 {
		target += "?" + query.Encode()
	}

	var payload []byte
	if body != nil {
		var err error
		if payload, err = json.Marshal(body); err != nil {
			return fmt.Errorf("encoding request body: %w", err)
		}
	}

	for attempt := 0; ; attempt++ {
		var reader io.Reader
		if payload != nil {
			reader = bytes.NewReader(payload)
		}
		req, err := http.NewRequestWithContext(ctx, method, target, reader)
		if err != nil {
			return err
		}
		req.SetBasicAuth(c.apiKey, "")
		req.Header.Set("Accept", "application/json")
		req.Header.Set("Cronitor-Version", APIVersion)
		req.Header.Set("User-Agent", c.userAgent)
		if payload != nil {
			req.Header.Set("Content-Type", "application/json")
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("cronitor: %s %s: %w", method, path, err)
		}
		respBody, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return fmt.Errorf("cronitor: reading response of %s %s: %w", method, path, err)
		}

		if retryable(resp.StatusCode) && attempt < maxRetries {
			if err := sleep(ctx, retryDelay(resp, attempt)); err != nil {
				return err
			}
			continue
		}

		if resp.StatusCode < 200 || resp.StatusCode > 299 {
			return &APIError{StatusCode: resp.StatusCode, Method: method, Path: path, Body: string(respBody)}
		}
		if out == nil || len(bytes.TrimSpace(respBody)) == 0 {
			return nil
		}
		if err := json.Unmarshal(respBody, out); err != nil {
			return fmt.Errorf("cronitor: decoding response of %s %s: %w", method, path, err)
		}
		return nil
	}
}

func retryable(status int) bool {
	switch status {
	case http.StatusTooManyRequests, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	}
	return false
}

func retryDelay(resp *http.Response, attempt int) time.Duration {
	if s := resp.Header.Get("Retry-After"); s != "" {
		if secs, err := strconv.Atoi(s); err == nil && secs >= 0 {
			return time.Duration(secs) * time.Second
		}
	}
	return time.Duration(1<<attempt) * time.Second
}

func sleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func escape(key string) string {
	return url.PathEscape(key)
}
