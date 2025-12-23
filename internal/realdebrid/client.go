package realdebrid

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/time/rate"
)

const (
	BaseURL        = "https://api.real-debrid.com/rest/1.0"
	RateLimitPerMin = 250
)

type Client struct {
	apiKey     string
	httpClient *http.Client
	limiter    *rate.Limiter
}

func NewClient(apiKey string) *Client {
	// 250 requests per minute = ~4.16 per second
	limiter := rate.NewLimiter(rate.Every(time.Minute/RateLimitPerMin), 5)

	return &Client{
		apiKey: apiKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		limiter: limiter,
	}
}

func (c *Client) doRequest(ctx context.Context, method, endpoint string, body io.Reader, contentType string) (*http.Response, error) {
	// Wait for rate limiter
	if err := c.limiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("rate limiter error: %w", err)
	}

	reqURL := BaseURL + endpoint
	req, err := http.NewRequestWithContext(ctx, method, reqURL, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error (status %d): %s", resp.StatusCode, string(bodyBytes))
	}

	return resp, nil
}

func (c *Client) get(ctx context.Context, endpoint string, result interface{}) error {
	resp, err := c.doRequest(ctx, http.MethodGet, endpoint, nil, "")
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if result != nil {
		if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
			return fmt.Errorf("failed to decode response: %w", err)
		}
	}

	return nil
}

func (c *Client) post(ctx context.Context, endpoint string, data url.Values, result interface{}) error {
	var body io.Reader
	var contentType string

	if data != nil {
		body = strings.NewReader(data.Encode())
		contentType = "application/x-www-form-urlencoded"
	}

	resp, err := c.doRequest(ctx, http.MethodPost, endpoint, body, contentType)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if result != nil {
		if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
			return fmt.Errorf("failed to decode response: %w", err)
		}
	}

	return nil
}

func (c *Client) put(ctx context.Context, endpoint string, body io.Reader, contentType string, result interface{}) error {
	resp, err := c.doRequest(ctx, http.MethodPut, endpoint, body, contentType)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if result != nil {
		if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
			return fmt.Errorf("failed to decode response: %w", err)
		}
	}

	return nil
}

func (c *Client) delete(ctx context.Context, endpoint string) error {
	resp, err := c.doRequest(ctx, http.MethodDelete, endpoint, nil, "")
	if err != nil {
		return err
	}
	resp.Body.Close()
	return nil
}
