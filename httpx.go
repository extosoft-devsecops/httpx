package httpx

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"extosoft-devsecops/hrex-http/httpx/logger"
)

const (
	defaultRetries      = 1
	defaultHTTPTimeout  = 10 * time.Second
	defaultRetryDelay   = 100 * time.Millisecond
	defaultMaxRetryWait = 5 * time.Second
)

type Client interface {
	Do(ctx context.Context, req *http.Request) (*http.Response, error)
}

type client struct {
	HttpClient   *http.Client
	Logger       *slog.Logger
	Retries      int
	RetryDelay   time.Duration
	MaxRetryWait time.Duration
}

type ClientOption func(*client)

func WithRetries(n int) ClientOption {
	return func(c *client) { c.Retries = n }
}

func WithTimeout(d time.Duration) ClientOption {
	return func(c *client) { c.HttpClient.Timeout = d }
}

func WithRetryDelay(d time.Duration) ClientOption {
	return func(c *client) { c.RetryDelay = d }
}

func WithMaxRetryWait(d time.Duration) ClientOption {
	return func(c *client) { c.MaxRetryWait = d }
}

func New(log *slog.Logger, opts ...ClientOption) Client {
	transport := logger.NewLoggingRoundTripper(
		log,
		http.DefaultTransport,
		logger.WithBodyLogging(false),
	)

	c := &client{
		HttpClient: &http.Client{
			Transport: transport,
			Timeout:   defaultHTTPTimeout,
		},
		Logger:       log,
		Retries:      defaultRetries,
		RetryDelay:   defaultRetryDelay,
		MaxRetryWait: defaultMaxRetryWait,
	}

	for _, opt := range opts {
		opt(c)
	}

	return c
}

func (c *client) Do(ctx context.Context, req *http.Request) (*http.Response, error) {
	var bodyBytes []byte
	var lastErr error

	// Preserve request body for retries
	if req.Body != nil {
		bodyBytes, lastErr = io.ReadAll(req.Body)
		if lastErr != nil {
			return nil, fmt.Errorf("failed to read request body: %w", lastErr)
		}
		_ = req.Body.Close()
	}

	// Use context from request if not provided
	if ctx == nil {
		ctx = req.Context()
	}
	req = req.WithContext(ctx)

	for attempt := 1; attempt <= c.Retries; attempt++ {
		// Restore request body for each attempt
		if bodyBytes != nil {
			req.Body = io.NopCloser(bytes.NewReader(bodyBytes))
		}

		resp, err := c.HttpClient.Do(req)

		if err != nil {
			lastErr = err
			c.Logger.WarnContext(ctx, "request attempt failed",
				slog.Int("attempt", attempt),
				slog.Int("max_retries", c.Retries),
				slog.String("method", req.Method),
				slog.String("url", req.URL.String()),
				slog.Any("error", err),
			)

			// Don't retry if it's the last attempt
			if attempt >= c.Retries {
				return nil, fmt.Errorf("request failed after %d attempts: %w", c.Retries, lastErr)
			}

			// Wait before retry with exponential backoff
			c.waitBeforeRetry(ctx, attempt)
			continue
		}

		// Check if we should retry based on status code
		if c.shouldRetry(resp.StatusCode) && attempt < c.Retries {
			// Close the response body before retry
			_ = resp.Body.Close()

			c.Logger.WarnContext(ctx, "retrying due to status code",
				slog.Int("status", resp.StatusCode),
				slog.Int("attempt", attempt),
				slog.Int("max_retries", c.Retries),
				slog.String("url", req.URL.String()),
			)

			c.waitBeforeRetry(ctx, attempt)
			continue
		}

		// Success
		return resp, nil
	}

	// Should not reach here, but handle gracefully
	if lastErr != nil {
		return nil, fmt.Errorf("request failed after %d attempts: %w", c.Retries, lastErr)
	}
	return nil, fmt.Errorf("request failed after %d attempts with unknown error", c.Retries)
}

// shouldRetry determines if a request should be retried based on the status code
func (c *client) shouldRetry(statusCode int) bool {
	// Retry on:
	// - 429 Too Many Requests
	// - 408 Request Timeout
	// - 5xx Server Errors
	return statusCode == http.StatusTooManyRequests ||
		statusCode == http.StatusRequestTimeout ||
		(statusCode >= 500 && statusCode < 600)
}

// waitBeforeRetry implements exponential backoff
func (c *client) waitBeforeRetry(ctx context.Context, attempt int) {
	// Exponential backoff: delay * 2^(attempt-1)
	delay := c.RetryDelay * time.Duration(1<<uint(attempt-1))

	// Cap at maximum retry wait time
	if delay > c.MaxRetryWait {
		delay = c.MaxRetryWait
	}

	c.Logger.DebugContext(ctx, "waiting before retry",
		slog.Duration("delay", delay),
		slog.Int("attempt", attempt),
	)

	select {
	case <-time.After(delay):
		// Continue with retry
	case <-ctx.Done():
		// Context cancelled, exit early
		return
	}
}
