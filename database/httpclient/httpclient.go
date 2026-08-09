// Package httpclient wraps net/http with the two things every outbound
// call to another service needs and stdlib doesn't give you for free:
// automatic retry with backoff on transient failures, and OTel context
// propagation so a downstream call is a child span of whatever request
// triggered it (required for the trace to stay connected across service
// boundaries)
package httpclient

import (
	"context"
	"fmt"
	"io"
	"math"
	"net/http"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// Config configures New
type Config struct {
	Timeout    time.Duration // per-attempt timeout, default 10s
	MaxRetires int           // default 3; retires on networks errors and 5xx/429
	BaseDelay  time.Duration // default 200ms, doubles each retry
}

// Client wraps *http.Client with retry and traicing
type Client struct {
	http    *http.Client
	retries int
	delay   time.Duration
}

// New builds a Client. The underlying transport is wrapped with
// otelhttp, so every request Do makes is a traced span carrying the
// W3C traceparent header to the callee - no extra code needed at call
// sites to get distributed tracing across a HTTP call
func New(cfg Config) *Client {
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}

	retries := cfg.MaxRetires
	if retries <= 0 {
		retries = 3
	}

	delay := cfg.BaseDelay
	if delay <= 0 {
		delay = 200 * time.Millisecond
	}

	return &Client{
		http: &http.Client{
			Timeout:   timeout,
			Transport: otelhttp.NewTransport(http.DefaultTransport),
		},
		retries: retries,
		delay:   delay,
	}
}

// Do sends req, retrying on network errors and 429/5xx responses with
// exponential backoff. req.Body, if any, must support GetBody (
// as produced by http.NewRequest with a non-nil body) so it can be replayed
func (c *Client) Do(req *http.Request) (*http.Response, error) {
	var lastErr error
	for attempt := 0; attempt <= c.retries; attempt++ {
		if attempt > 0 {
			if req.GetBody != nil {
				body, err := req.GetBody()
				if err != nil {
					return nil, fmt.Errorf("httpclient: rewinding request body for retry: %w", err)
				}
				req.Body = body
			}
			select {
			case <-req.Context().Done():
				return nil, req.Context().Err()
			case <-time.After(backoff(c.delay, attempt)):
			}
		}

		resp, err := c.http.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			lastErr = fmt.Errorf("httpclient: status %d", resp.StatusCode)
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			continue
		}
		return resp, nil
	}
	return nil, fmt.Errorf("httpclient: giving up after %d attempts: %w", c.retries+1, lastErr)
}

// Get is a convenience wrapper arround Do for simple GET requests
func (c *Client) Get(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	return c.Do(req)
}

func backoff(base time.Duration, attempt int) time.Duration {
	return time.Duration(float64(base) * math.Pow(2, float64(attempt-1)))
}
