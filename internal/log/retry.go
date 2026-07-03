package log

import (
	"context"
	"fmt"
	"math"
	"math/rand/v2"
	"net/http"
	"strconv"
	"time"
)

const (
	defaultMaxRetries    = 5
	defaultBaseDelay     = 1 * time.Second
	defaultMaxDelay      = 60 * time.Second
	defaultJitterFraction = 0.25
)

// RetryTransport wraps an http.RoundTripper with exponential backoff for
// retryable responses (429 Too Many Requests, 5xx server errors) and
// transient network errors.
type RetryTransport struct {
	Transport  http.RoundTripper
	MaxRetries int
	BaseDelay  time.Duration
	MaxDelay   time.Duration
}

// RoundTrip implements http.RoundTripper with exponential backoff.
func (rt *RetryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	transport := rt.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}
	maxRetries := rt.MaxRetries
	if maxRetries <= 0 {
		maxRetries = defaultMaxRetries
	}
	baseDelay := rt.BaseDelay
	if baseDelay <= 0 {
		baseDelay = defaultBaseDelay
	}
	maxDelay := rt.MaxDelay
	if maxDelay <= 0 {
		maxDelay = defaultMaxDelay
	}

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			delay := backoffDuration(baseDelay, maxDelay, attempt-1)
			select {
			case <-req.Context().Done():
				return nil, req.Context().Err()
			case <-time.After(delay):
			}
		}

		// Clone the request body for retries since it may have been consumed.
		var reqCopy *http.Request
		if attempt < maxRetries && req.Body != nil && req.Body != http.NoBody {
			var err error
			reqCopy, err = cloneRequest(req)
			if err != nil {
				return nil, fmt.Errorf("failed to clone request for retry: %w", err)
			}
		} else {
			reqCopy = req
		}

		resp, err := transport.RoundTrip(reqCopy)
		if err != nil {
			lastErr = err
			if !isRetryableNetworkError(err) {
				return nil, err
			}
			continue
		}

		if !isRetryableStatus(resp.StatusCode) {
			return resp, nil
		}

		// Check for Retry-After header.
		retryAfter := retryAfterDelay(resp)
		resp.Body.Close()

		if retryAfter > 0 {
			select {
			case <-req.Context().Done():
				return nil, req.Context().Err()
			case <-time.After(retryAfter):
			}
		}

		lastErr = fmt.Errorf("retryable HTTP status: %d %s", resp.StatusCode, resp.Status)
	}

	return nil, fmt.Errorf("max retries (%d) exceeded: %w", maxRetries, lastErr)
}

// NewRetryHTTPClient creates an HTTP client with exponential backoff retry
// for 429 and 5xx responses. The transport also logs requests/responses
// at debug level, combining retry + debug logging.
func NewRetryHTTPClient() *http.Client {
	return &http.Client{
		Transport: &RetryTransport{
			Transport: &HTTPRoundTripLogger{
				Transport: http.DefaultTransport,
			},
		},
	}
}

// NewRetryHTTPClientWithDebug returns an HTTP client with retry. When debug
// is true, request/response bodies are logged.
func NewRetryHTTPClientWithDebug(debug bool) *http.Client {
	if debug {
		return NewRetryHTTPClient()
	}
	return &http.Client{
		Transport: &RetryTransport{
			Transport: http.DefaultTransport,
		},
	}
}

// backoffDuration computes the exponential backoff delay with jitter.
func backoffDuration(base, max time.Duration, attempt int) time.Duration {
	d := float64(base) * math.Pow(2, float64(attempt))
	if d > float64(max) {
		d = float64(max)
	}
	// Add jitter: ±25%.
	jitter := d * defaultJitterFraction * (2*rand.Float64() - 1)
	d += jitter
	if d < 0 {
		d = float64(base)
	}
	return time.Duration(d)
}

// isRetryableStatus returns true for 429 and 5xx responses.
func isRetryableStatus(code int) bool {
	return code == http.StatusTooManyRequests || code >= 500
}

// isRetryableNetworkError returns true for transient network errors.
func isRetryableNetworkError(err error) bool {
	if err == nil {
		return false
	}
	// Context cancellation is not retryable.
	if err == context.Canceled || err == context.DeadlineExceeded {
		return false
	}
	return true
}

// retryAfterDelay parses the Retry-After header. Returns 0 if absent or
// unparseable, in which case exponential backoff is used.
func retryAfterDelay(resp *http.Response) time.Duration {
	v := resp.Header.Get("Retry-After")
	if v == "" {
		return 0
	}
	if seconds, err := strconv.Atoi(v); err == nil {
		return time.Duration(seconds) * time.Second
	}
	if t, err := time.Parse(http.TimeFormat, v); err == nil {
		if d := time.Until(t); d > 0 {
			return d
		}
	}
	return 0
}

// cloneRequest clones an HTTP request, preserving the body.
func cloneRequest(req *http.Request) (*http.Request, error) {
	clone := req.Clone(req.Context())
	if req.Body == nil || req.Body == http.NoBody {
		return clone, nil
	}
	var err error
	clone.Body, req.Body, err = drainBody(req.Body)
	return clone, err
}
