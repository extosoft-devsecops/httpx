package httpx_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"extosoft.com/hrex/httpx"
)

// mockRoundTripper implements http.RoundTripper for testing
type mockRoundTripper struct {
	responses []*http.Response
	errors    []error
	callCount int
}

func (m *mockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	defer func() { m.callCount++ }()

	if m.callCount < len(m.errors) && m.errors[m.callCount] != nil {
		return nil, m.errors[m.callCount]
	}

	if m.callCount < len(m.responses) {
		return m.responses[m.callCount], nil
	}

	// Default response
	return &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(strings.NewReader("")),
		Header:     make(http.Header),
	}, nil
}

func newTestClient(opts ...httpx.ClientOption) httpx.Client {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	return httpx.New(logger, opts...)
}

func TestNew(t *testing.T) {
	t.Run("creates client with defaults", func(t *testing.T) {
		logger := slog.New(slog.NewTextHandler(io.Discard, nil))
		client := httpx.New(logger)
		if client == nil {
			t.Fatal("expected non-nil client")
		}
	})

	t.Run("applies options", func(t *testing.T) {
		logger := slog.New(slog.NewTextHandler(io.Discard, nil))
		client := httpx.New(logger,
			httpx.WithRetries(3),
			httpx.WithTimeout(30*time.Second),
			httpx.WithRetryDelay(200*time.Millisecond),
			httpx.WithMaxRetryWait(10*time.Second),
		)
		if client == nil {
			t.Fatal("expected non-nil client")
		}
	})
}

func TestClient_Do_Success(t *testing.T) {
	client := newTestClient()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	}))
	defer server.Close()

	req, err := http.NewRequest("GET", server.URL, nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	resp, err := client.Do(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if string(body) != "success" {
		t.Errorf("expected body 'success', got '%s'", string(body))
	}
}

func TestClient_Do_WithRequestBody(t *testing.T) {
	client := newTestClient()

	var receivedBody string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		receivedBody = string(body)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	requestBody := "test request body"
	req, err := http.NewRequest("POST", server.URL, strings.NewReader(requestBody))
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	resp, err := client.Do(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if receivedBody != requestBody {
		t.Errorf("expected body '%s', got '%s'", requestBody, receivedBody)
	}
}

func TestClient_Do_Retry_OnError(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		if callCount < 3 {
			// Simulate temporary failure by closing connection
			// In real scenario, this would be a network error
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	}))
	defer server.Close()

	client := newTestClient(
		httpx.WithRetries(3),
		httpx.WithRetryDelay(10*time.Millisecond),
	)

	req, err := http.NewRequest("GET", server.URL, nil)
	if err != nil {
		t.Fatalf("failed to create request: %v", err)
	}

	resp, err := client.Do(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}

	if callCount != 3 {
		t.Errorf("expected 3 attempts, got %d", callCount)
	}
}

func TestClient_Do_Retry_OnStatusCode(t *testing.T) {
	testCases := []struct {
		name        string
		statusCode  int
		shouldRetry bool
	}{
		{"429 Too Many Requests", http.StatusTooManyRequests, true},
		{"408 Request Timeout", http.StatusRequestTimeout, true},
		{"500 Internal Server Error", http.StatusInternalServerError, true},
		{"502 Bad Gateway", http.StatusBadGateway, true},
		{"503 Service Unavailable", http.StatusServiceUnavailable, true},
		{"504 Gateway Timeout", http.StatusGatewayTimeout, true},
		{"400 Bad Request", http.StatusBadRequest, false},
		{"401 Unauthorized", http.StatusUnauthorized, false},
		{"404 Not Found", http.StatusNotFound, false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			callCount := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				callCount++
				if callCount < 2 {
					w.WriteHeader(tc.statusCode)
					return
				}
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			client := newTestClient(
				httpx.WithRetries(3),
				httpx.WithRetryDelay(10*time.Millisecond),
			)

			req, _ := http.NewRequest("GET", server.URL, nil)
			resp, err := client.Do(context.Background(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			defer resp.Body.Close()

			if tc.shouldRetry {
				if callCount < 2 {
					t.Errorf("expected retry, but only %d calls made", callCount)
				}
			} else {
				if callCount != 1 {
					t.Errorf("expected no retry, but %d calls made", callCount)
				}
			}
		})
	}
}

func TestClient_Do_Retry_WithBody(t *testing.T) {
	callCount := 0
	var lastBody string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		body, _ := io.ReadAll(r.Body)
		lastBody = string(body)

		if callCount < 2 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := newTestClient(
		httpx.WithRetries(3),
		httpx.WithRetryDelay(10*time.Millisecond),
	)

	requestBody := "test body content"
	req, _ := http.NewRequest("POST", server.URL, strings.NewReader(requestBody))

	resp, err := client.Do(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if callCount < 2 {
		t.Errorf("expected at least 2 attempts, got %d", callCount)
	}

	if lastBody != requestBody {
		t.Errorf("body not preserved across retries: expected '%s', got '%s'", requestBody, lastBody)
	}
}

func TestClient_Do_MaxRetriesReached(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := newTestClient(
		httpx.WithRetries(3),
		httpx.WithRetryDelay(10*time.Millisecond),
	)

	req, _ := http.NewRequest("GET", server.URL, nil)
	resp, err := client.Do(context.Background(), req)

	// Should still return the last response even though it failed
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", resp.StatusCode)
	}

	if callCount != 3 {
		t.Errorf("expected 3 attempts, got %d", callCount)
	}
}

func TestClient_Do_ContextCancellation(t *testing.T) {
	callCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := newTestClient(
		httpx.WithRetries(5),
		httpx.WithRetryDelay(100*time.Millisecond),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()

	req, _ := http.NewRequest("GET", server.URL, nil)
	_, err := client.Do(ctx, req)

	if err == nil {
		t.Error("expected error due to context cancellation")
	}

	// Should have made at least one attempt but not all 5
	if callCount == 0 {
		t.Error("expected at least one attempt")
	}
	if callCount >= 5 {
		t.Errorf("expected context cancellation to stop retries, but got %d attempts", callCount)
	}
}

func TestClient_Do_ExponentialBackoff(t *testing.T) {
	client := newTestClient(
		httpx.WithRetries(4),
		httpx.WithRetryDelay(100*time.Millisecond),
		httpx.WithMaxRetryWait(500*time.Millisecond),
	)

	callTimes := []time.Time{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callTimes = append(callTimes, time.Now())
		if len(callTimes) < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	req, _ := http.NewRequest("GET", server.URL, nil)
	start := time.Now()
	resp, err := client.Do(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	duration := time.Since(start)

	// With exponential backoff: 100ms, 200ms
	// Total expected: ~300ms minimum
	if duration < 250*time.Millisecond {
		t.Errorf("expected backoff delays, but completed too quickly: %v", duration)
	}

	// Should have made 3 attempts
	if len(callTimes) != 3 {
		t.Errorf("expected 3 attempts, got %d", len(callTimes))
	}

	// Check that delays are increasing
	if len(callTimes) >= 3 {
		delay1 := callTimes[1].Sub(callTimes[0])
		delay2 := callTimes[2].Sub(callTimes[1])

		if delay2 <= delay1 {
			t.Errorf("expected exponential backoff, but delay2 (%v) <= delay1 (%v)", delay2, delay1)
		}
	}
}

func TestClient_Do_NilContext(t *testing.T) {
	client := newTestClient()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	req, _ := http.NewRequest("GET", server.URL, nil)
	resp, err := client.Do(nil, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", resp.StatusCode)
	}
}

func TestClient_Do_ReadBodyError(t *testing.T) {
	client := newTestClient()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Create a request with a body that returns an error
	errorBody := &errorReader{err: errors.New("read error")}
	req, _ := http.NewRequest("POST", server.URL, errorBody)

	_, err := client.Do(context.Background(), req)
	if err == nil {
		t.Error("expected error when reading body fails")
	}

	if !strings.Contains(err.Error(), "failed to read request body") {
		t.Errorf("expected error message about reading body, got: %v", err)
	}
}

func TestClient_Do_HTTPMethods(t *testing.T) {
	methods := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			var receivedMethod string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				receivedMethod = r.Method
				w.WriteHeader(http.StatusOK)
			}))
			defer server.Close()

			client := newTestClient()
			req, _ := http.NewRequest(method, server.URL, nil)
			resp, err := client.Do(context.Background(), req)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			defer resp.Body.Close()

			if receivedMethod != method {
				t.Errorf("expected method %s, got %s", method, receivedMethod)
			}
		})
	}
}

func TestClient_Do_MaxRetryWaitCap(t *testing.T) {
	client := newTestClient(
		httpx.WithRetries(10),
		httpx.WithRetryDelay(100*time.Millisecond),
		httpx.WithMaxRetryWait(300*time.Millisecond), // Cap at 300ms
	)

	callTimes := []time.Time{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callTimes = append(callTimes, time.Now())
		if len(callTimes) < 5 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	req, _ := http.NewRequest("GET", server.URL, nil)
	resp, err := client.Do(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	// Check that later delays don't exceed the cap
	if len(callTimes) >= 4 {
		// Delay between attempt 3 and 4 should be capped at 300ms
		delay := callTimes[3].Sub(callTimes[2])
		if delay > 350*time.Millisecond {
			t.Errorf("expected delay to be capped at ~300ms, got %v", delay)
		}
	}
}

func TestWithRetries(t *testing.T) {
	client := newTestClient(httpx.WithRetries(5))
	if client == nil {
		t.Fatal("expected non-nil client")
	}
}

func TestWithTimeout(t *testing.T) {
	client := newTestClient(httpx.WithTimeout(30 * time.Second))
	if client == nil {
		t.Fatal("expected non-nil client")
	}
}

func TestWithRetryDelay(t *testing.T) {
	client := newTestClient(httpx.WithRetryDelay(200 * time.Millisecond))
	if client == nil {
		t.Fatal("expected non-nil client")
	}
}

func TestWithMaxRetryWait(t *testing.T) {
	client := newTestClient(httpx.WithMaxRetryWait(10 * time.Second))
	if client == nil {
		t.Fatal("expected non-nil client")
	}
}

// errorReader is a helper that always returns an error when read
type errorReader struct {
	err error
}

func (e *errorReader) Read(p []byte) (n int, err error) {
	return 0, e.err
}

func (e *errorReader) Close() error {
	return nil
}
