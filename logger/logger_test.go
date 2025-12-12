package logger_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"extosoft.com/hrex/httpx/logger"
)

// mockRoundTripper implements http.RoundTripper for testing
type mockRoundTripper struct {
	response *http.Response
	err      error
	requests []*http.Request
}

func (m *mockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	m.requests = append(m.requests, req)
	if m.err != nil {
		return nil, m.err
	}
	return m.response, nil
}

func TestNewLoggingRoundTripper(t *testing.T) {
	logBuf := &bytes.Buffer{}
	log := slog.New(slog.NewJSONHandler(logBuf, nil))

	t.Run("with custom transport", func(t *testing.T) {
		mockTransport := &mockRoundTripper{}
		rt := logger.NewLoggingRoundTripper(log, mockTransport)
		if rt == nil {
			t.Fatal("expected non-nil LoggingRoundTripper")
		}
	})

	t.Run("with nil transport uses default", func(t *testing.T) {
		rt := logger.NewLoggingRoundTripper(log, nil)
		if rt == nil {
			t.Fatal("expected non-nil LoggingRoundTripper")
		}
	})

	t.Run("with options", func(t *testing.T) {
		rt := logger.NewLoggingRoundTripper(
			log,
			nil,
			logger.WithBodyLogging(true),
			logger.WithMaxBodySize(1024),
		)
		if rt == nil {
			t.Fatal("expected non-nil LoggingRoundTripper")
		}
	})
}

func TestLoggingRoundTripper_RoundTrip_Success(t *testing.T) {
	logBuf := &bytes.Buffer{}
	log := slog.New(slog.NewJSONHandler(logBuf, nil))

	mockResp := &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(strings.NewReader("response body")),
		Header:     make(http.Header),
	}

	mockTransport := &mockRoundTripper{
		response: mockResp,
	}

	rt := logger.NewLoggingRoundTripper(log, mockTransport)

	req := httptest.NewRequest("GET", "http://example.com/test", nil)
	resp, err := rt.RoundTrip(req)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp == nil {
		t.Fatal("expected non-nil response")
	}

	if resp.StatusCode != 200 {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	// Verify logging occurred
	logs := logBuf.String()
	if !strings.Contains(logs, "http request completed") {
		t.Error("expected 'http request completed' in logs")
	}
	if !strings.Contains(logs, "example.com") {
		t.Error("expected URL in logs")
	}
}

func TestLoggingRoundTripper_RoundTrip_Error(t *testing.T) {
	logBuf := &bytes.Buffer{}
	log := slog.New(slog.NewJSONHandler(logBuf, nil))

	expectedErr := errors.New("connection failed")
	mockTransport := &mockRoundTripper{
		err: expectedErr,
	}

	rt := logger.NewLoggingRoundTripper(log, mockTransport)

	req := httptest.NewRequest("POST", "http://example.com/api", nil)
	resp, err := rt.RoundTrip(req)

	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if resp != nil {
		t.Errorf("expected nil response on error, got %v", resp)
	}

	// Verify error logging
	logs := logBuf.String()
	if !strings.Contains(logs, "http request failed") {
		t.Error("expected 'http request failed' in logs")
	}
	if !strings.Contains(logs, "connection failed") {
		t.Error("expected error message in logs")
	}
}

func TestLoggingRoundTripper_WithBodyLogging_Request(t *testing.T) {
	logBuf := &bytes.Buffer{}
	log := slog.New(slog.NewJSONHandler(logBuf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	mockResp := &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(strings.NewReader("")),
		Header:     make(http.Header),
	}

	mockTransport := &mockRoundTripper{
		response: mockResp,
	}

	rt := logger.NewLoggingRoundTripper(
		log,
		mockTransport,
		logger.WithBodyLogging(true),
	)

	requestBody := "test request body"
	req := httptest.NewRequest("POST", "http://example.com/api", strings.NewReader(requestBody))

	_, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	logs := logBuf.String()
	if !strings.Contains(logs, "http request body") {
		t.Error("expected 'http request body' in logs")
	}
	if !strings.Contains(logs, requestBody) {
		t.Error("expected request body content in logs")
	}

	// Verify that the request body can still be read by the transport
	if len(mockTransport.requests) == 0 {
		t.Fatal("expected at least one request")
	}
	capturedReq := mockTransport.requests[0]
	if capturedReq.Body != nil {
		bodyBytes, _ := io.ReadAll(capturedReq.Body)
		if string(bodyBytes) != requestBody {
			t.Errorf("expected body '%s', got '%s'", requestBody, string(bodyBytes))
		}
	}
}

func TestLoggingRoundTripper_WithBodyLogging_Response(t *testing.T) {
	logBuf := &bytes.Buffer{}
	log := slog.New(slog.NewJSONHandler(logBuf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	responseBody := "test response body"
	mockResp := &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(strings.NewReader(responseBody)),
		Header:     make(http.Header),
	}

	mockTransport := &mockRoundTripper{
		response: mockResp,
	}

	rt := logger.NewLoggingRoundTripper(
		log,
		mockTransport,
		logger.WithBodyLogging(true),
	)

	req := httptest.NewRequest("GET", "http://example.com/api", nil)
	resp, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	logs := logBuf.String()
	if !strings.Contains(logs, "http response body") {
		t.Error("expected 'http response body' in logs")
	}
	if !strings.Contains(logs, responseBody) {
		t.Error("expected response body content in logs")
	}

	// Verify that the response body can still be read
	if resp.Body == nil {
		t.Fatal("expected non-nil response body")
	}
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}
	if string(bodyBytes) != responseBody {
		t.Errorf("expected body '%s', got '%s'", responseBody, string(bodyBytes))
	}
}

func TestLoggingRoundTripper_MaxBodySize(t *testing.T) {
	logBuf := &bytes.Buffer{}
	log := slog.New(slog.NewJSONHandler(logBuf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	largeBody := strings.Repeat("a", 1000)
	mockResp := &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(strings.NewReader(largeBody)),
		Header:     make(http.Header),
	}

	mockTransport := &mockRoundTripper{
		response: mockResp,
	}

	rt := logger.NewLoggingRoundTripper(
		log,
		mockTransport,
		logger.WithBodyLogging(true),
		logger.WithMaxBodySize(100),
	)

	req := httptest.NewRequest("GET", "http://example.com/api", nil)
	resp, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// The full body should still be readable
	if resp.Body == nil {
		t.Fatal("expected non-nil response body")
	}
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}
	if len(bodyBytes) != 1000 {
		t.Errorf("expected full body length 1000, got %d", len(bodyBytes))
	}

	// The logged body should be truncated (we can't verify exact log content easily,
	// but we verified the body is still complete)
	logs := logBuf.String()
	if !strings.Contains(logs, "http response body") {
		t.Error("expected response body to be logged")
	}
}

func TestLoggingRoundTripper_WithoutBodyLogging(t *testing.T) {
	logBuf := &bytes.Buffer{}
	log := slog.New(slog.NewJSONHandler(logBuf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	mockResp := &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(strings.NewReader("response body")),
		Header:     make(http.Header),
	}

	mockTransport := &mockRoundTripper{
		response: mockResp,
	}

	rt := logger.NewLoggingRoundTripper(
		log,
		mockTransport,
		logger.WithBodyLogging(false),
	)

	requestBody := "request body"
	req := httptest.NewRequest("POST", "http://example.com/api", strings.NewReader(requestBody))

	_, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	logs := logBuf.String()
	if strings.Contains(logs, "http request body") {
		t.Error("did not expect 'http request body' in logs when body logging is disabled")
	}
	if strings.Contains(logs, "http response body") {
		t.Error("did not expect 'http response body' in logs when body logging is disabled")
	}
	if !strings.Contains(logs, "http request completed") {
		t.Error("expected 'http request completed' in logs")
	}
}

func TestLoggingRoundTripper_NilRequestBody(t *testing.T) {
	logBuf := &bytes.Buffer{}
	log := slog.New(slog.NewJSONHandler(logBuf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	mockResp := &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(strings.NewReader("")),
		Header:     make(http.Header),
	}

	mockTransport := &mockRoundTripper{
		response: mockResp,
	}

	rt := logger.NewLoggingRoundTripper(
		log,
		mockTransport,
		logger.WithBodyLogging(true),
	)

	req := httptest.NewRequest("GET", "http://example.com/api", nil)
	_, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should not panic or error with nil body
	logs := logBuf.String()
	if !strings.Contains(logs, "http request completed") {
		t.Error("expected 'http request completed' in logs")
	}
}

func TestLoggingRoundTripper_NilResponseBody(t *testing.T) {
	logBuf := &bytes.Buffer{}
	log := slog.New(slog.NewJSONHandler(logBuf, &slog.HandlerOptions{
		Level: slog.LevelDebug,
	}))

	mockResp := &http.Response{
		StatusCode: 204,
		Body:       nil,
		Header:     make(http.Header),
	}

	mockTransport := &mockRoundTripper{
		response: mockResp,
	}

	rt := logger.NewLoggingRoundTripper(
		log,
		mockTransport,
		logger.WithBodyLogging(true),
	)

	req := httptest.NewRequest("GET", "http://example.com/api", nil)
	resp, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if resp.StatusCode != 204 {
		t.Errorf("expected status 204, got %d", resp.StatusCode)
	}

	// Should not panic with nil response body
	logs := logBuf.String()
	if !strings.Contains(logs, "http request completed") {
		t.Error("expected 'http request completed' in logs")
	}
}

func TestLoggingRoundTripper_ContextPropagation(t *testing.T) {
	logBuf := &bytes.Buffer{}
	log := slog.New(slog.NewJSONHandler(logBuf, nil))

	mockResp := &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(strings.NewReader("")),
		Header:     make(http.Header),
	}

	mockTransport := &mockRoundTripper{
		response: mockResp,
	}

	rt := logger.NewLoggingRoundTripper(log, mockTransport)

	ctx := context.WithValue(context.Background(), "test-key", "test-value")
	req := httptest.NewRequest("GET", "http://example.com/test", nil)
	req = req.WithContext(ctx)

	_, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify context was passed to transport
	if len(mockTransport.requests) == 0 {
		t.Fatal("expected at least one request")
	}
	capturedReq := mockTransport.requests[0]
	if capturedReq.Context() != ctx {
		t.Error("expected context to be propagated")
	}
}

func TestLoggingRoundTripper_DurationTracking(t *testing.T) {
	logBuf := &bytes.Buffer{}
	log := slog.New(slog.NewJSONHandler(logBuf, nil))

	mockResp := &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(strings.NewReader("")),
		Header:     make(http.Header),
	}

	// Mock transport with artificial delay
	mockTransport := &mockRoundTripper{
		response: mockResp,
	}

	rt := logger.NewLoggingRoundTripper(log, mockTransport)

	req := httptest.NewRequest("GET", "http://example.com/test", nil)
	_, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	logs := logBuf.String()
	if !strings.Contains(logs, "duration") {
		t.Error("expected 'duration' in logs")
	}
}

func TestWithBodyLogging(t *testing.T) {
	logBuf := &bytes.Buffer{}
	log := slog.New(slog.NewJSONHandler(logBuf, nil))

	rt := logger.NewLoggingRoundTripper(
		log,
		nil,
		logger.WithBodyLogging(true),
	)

	if rt == nil {
		t.Fatal("expected non-nil LoggingRoundTripper")
	}
}

func TestWithMaxBodySize(t *testing.T) {
	logBuf := &bytes.Buffer{}
	log := slog.New(slog.NewJSONHandler(logBuf, nil))

	rt := logger.NewLoggingRoundTripper(
		log,
		nil,
		logger.WithMaxBodySize(2048),
	)

	if rt == nil {
		t.Fatal("expected non-nil LoggingRoundTripper")
	}
}

func TestLoggingRoundTripper_HTTPMethods(t *testing.T) {
	methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			logBuf := &bytes.Buffer{}
			log := slog.New(slog.NewJSONHandler(logBuf, nil))

			mockResp := &http.Response{
				StatusCode: 200,
				Body:       io.NopCloser(strings.NewReader("")),
				Header:     make(http.Header),
			}

			mockTransport := &mockRoundTripper{
				response: mockResp,
			}

			rt := logger.NewLoggingRoundTripper(log, mockTransport)

			req := httptest.NewRequest(method, "http://example.com/test", nil)
			_, err := rt.RoundTrip(req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			logs := logBuf.String()
			if !strings.Contains(logs, method) {
				t.Errorf("expected method '%s' in logs", method)
			}
		})
	}
}

func TestLoggingRoundTripper_StatusCodes(t *testing.T) {
	testCases := []struct {
		name       string
		statusCode int
	}{
		{"OK", 200},
		{"Created", 201},
		{"No Content", 204},
		{"Bad Request", 400},
		{"Unauthorized", 401},
		{"Forbidden", 403},
		{"Not Found", 404},
		{"Internal Server Error", 500},
		{"Service Unavailable", 503},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			logBuf := &bytes.Buffer{}
			log := slog.New(slog.NewJSONHandler(logBuf, nil))

			mockResp := &http.Response{
				StatusCode: tc.statusCode,
				Body:       io.NopCloser(strings.NewReader("")),
				Header:     make(http.Header),
			}

			mockTransport := &mockRoundTripper{
				response: mockResp,
			}

			rt := logger.NewLoggingRoundTripper(log, mockTransport)

			req := httptest.NewRequest("GET", "http://example.com/test", nil)
			resp, err := rt.RoundTrip(req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if resp.StatusCode != tc.statusCode {
				t.Errorf("expected status %d, got %d", tc.statusCode, resp.StatusCode)
			}

			logs := logBuf.String()
			if !strings.Contains(logs, "http request completed") {
				t.Error("expected 'http request completed' in logs")
			}
		})
	}
}
