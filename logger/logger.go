package logger

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"time"
)

const defaultMaxBodySize = 5 * 1024 * 1024 // 5MB

type LoggingRoundTripper struct {
	logger      *slog.Logger
	next        http.RoundTripper
	logBodies   bool
	maxBodySize int64
}

type LoggingOption func(*LoggingRoundTripper)

func WithBodyLogging(enabled bool) LoggingOption {
	return func(l *LoggingRoundTripper) { l.logBodies = enabled }
}

func WithMaxBodySize(size int64) LoggingOption {
	return func(l *LoggingRoundTripper) { l.maxBodySize = size }
}

func NewLoggingRoundTripper(logger *slog.Logger, next http.RoundTripper, opts ...LoggingOption) *LoggingRoundTripper {
	if next == nil {
		next = http.DefaultTransport
	}

	l := &LoggingRoundTripper{
		logger:      logger,
		next:        next,
		logBodies:   false,
		maxBodySize: defaultMaxBodySize,
	}

	for _, opt := range opts {
		opt(l)
	}
	return l
}

func (l *LoggingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	start := time.Now()
	ctx := req.Context()

	l.logRequest(ctx, req)

	resp, err := l.next.RoundTrip(req)
	duration := time.Since(start)

	if err != nil {
		l.logRequestError(ctx, req, duration, err)
		return nil, err
	}

	l.logResponse(ctx, req, resp, duration)

	return resp, nil
}

func (l *LoggingRoundTripper) logRequest(ctx context.Context, req *http.Request) {
	if !l.logBodies || req.Body == nil {
		return
	}

	bodyData, newReader, err := readBody(req.Body, l.maxBodySize)
	req.Body = newReader

	if err != nil {
		l.logger.WarnContext(ctx, "failed to read request body", slog.Any("error", err))
		return
	}

	l.logger.DebugContext(ctx, "http request body", slog.String("body", string(bodyData)))
}

func (l *LoggingRoundTripper) logRequestError(ctx context.Context, req *http.Request, duration time.Duration, err error) {
	l.logger.ErrorContext(ctx, "http request failed",
		slog.String("method", req.Method),
		slog.String("url", req.URL.String()),
		slog.Duration("duration", duration),
		slog.Any("error", err),
	)
}

func (l *LoggingRoundTripper) logResponse(ctx context.Context, req *http.Request, resp *http.Response, duration time.Duration) {
	if l.logBodies && resp.Body != nil {
		bodyData, newReader, err := readBody(resp.Body, l.maxBodySize)
		resp.Body = newReader

		if err != nil {
			l.logger.WarnContext(ctx, "failed to read response body", slog.Any("error", err))
		} else {
			l.logger.DebugContext(ctx, "http response body", slog.String("body", string(bodyData)))
		}
	}

	l.logger.InfoContext(ctx, "http request completed",
		slog.String("method", req.Method),
		slog.String("url", req.URL.String()),
		slog.Int("status", resp.StatusCode),
		slog.Duration("duration", duration),
	)
}

// readBody reads the body content, limits it for logging, and returns a new reader
// so the body can be read again by subsequent handlers.
func readBody(body io.ReadCloser, limit int64) ([]byte, io.ReadCloser, error) {
	data, err := io.ReadAll(body)
	_ = body.Close() // Close the original body after reading

	if err != nil {
		return nil, io.NopCloser(bytes.NewReader(nil)), err
	}

	// Determine how much data to return for logging
	logBytes := data
	if int64(len(data)) > limit {
		logBytes = data[:limit]
	}

	// Return a new reader with the full data so it can be read again
	return logBytes, io.NopCloser(bytes.NewReader(data)), nil
}
