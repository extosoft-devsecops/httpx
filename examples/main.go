package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"extosoft.com/hrex/httpx"
)

func main() {
	// Create a structured logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	fmt.Println("=== hrex-httpx Examples ===")
	// Example 1: Simple GET request
	fmt.Println("Example 1: Simple GET Request")
	fmt.Println("------------------------------")
	simpleGet(logger)

	// Example 2: POST request with body
	fmt.Println("\nExample 2: POST Request with Body")
	fmt.Println("----------------------------------")
	postWithBody(logger)

	// Example 3: Request with retry
	fmt.Println("\nExample 3: Request with Retry on Server Error")
	fmt.Println("----------------------------------------------")
	requestWithRetry(logger)

	// Example 4: Request with timeout
	fmt.Println("\nExample 4: Request with Context Timeout")
	fmt.Println("---------------------------------------")
	requestWithTimeout(logger)

	// Example 5: Custom configuration
	fmt.Println("\nExample 5: Custom Client Configuration")
	fmt.Println("---------------------------------------")
	customConfiguration(logger)

	// Example 6: Multiple retries with exponential backoff
	fmt.Println("\nExample 6: Exponential Backoff")
	fmt.Println("------------------------------")
	exponentialBackoff(logger)

	// Example 7: Request headers
	fmt.Println("\nExample 7: Custom Headers")
	fmt.Println("-------------------------")
	customHeaders(logger)

	fmt.Println("\n=== All Examples Completed ===")
}

// Example 1: Simple GET request
func simpleGet(logger *slog.Logger) {
	client := httpx.New(logger)

	req, err := http.NewRequest("GET", "https://httpbin.org/get", nil)
	if err != nil {
		log.Printf("Error creating request: %v\n", err)
		return
	}

	resp, err := client.Do(context.Background(), req)
	if err != nil {
		log.Printf("Error executing request: %v\n", err)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	fmt.Printf("Status: %d\n", resp.StatusCode)
	fmt.Printf("Response (first 200 chars): %s...\n", limitString(string(body), 200))
}

// Example 2: POST request with JSON body
func postWithBody(logger *slog.Logger) {
	client := httpx.New(logger)

	jsonData := `{
		"name": "John Doe",
		"email": "john@example.com",
		"age": 30,
		"city": "Bangkok"
	}`

	req, err := http.NewRequest("POST", "https://httpbin.org/post", strings.NewReader(jsonData))
	if err != nil {
		log.Printf("Error creating request: %v\n", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.Do(context.Background(), req)
	if err != nil {
		log.Printf("Error executing request: %v\n", err)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	fmt.Printf("Status: %d\n", resp.StatusCode)
	fmt.Printf("Response (first 300 chars): %s...\n", limitString(string(body), 300))
}

// Example 3: Request with automatic retry on failure
func requestWithRetry(logger *slog.Logger) {
	// Configure client with retries
	client := httpx.New(logger,
		httpx.WithRetries(3),
		httpx.WithRetryDelay(200*time.Millisecond),
	)

	// This endpoint returns 500 status code
	// The client will retry automatically
	req, err := http.NewRequest("GET", "https://httpbin.org/status/500", nil)
	if err != nil {
		log.Printf("Error creating request: %v\n", err)
		return
	}

	fmt.Println("Attempting request to endpoint that returns 500...")
	resp, err := client.Do(context.Background(), req)
	if err != nil {
		fmt.Printf("Request failed after retries: %v\n", err)
		return
	}
	defer resp.Body.Close()

	fmt.Printf("Final Status: %d (after retries)\n", resp.StatusCode)
	fmt.Println("Note: The client automatically retried on 5xx errors")
}

// Example 4: Request with context timeout
func requestWithTimeout(logger *slog.Logger) {
	client := httpx.New(logger)

	// Create context with 2 second timeout
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// This endpoint delays for 3 seconds, so it will timeout
	req, err := http.NewRequest("GET", "https://httpbin.org/delay/3", nil)
	if err != nil {
		log.Printf("Error creating request: %v\n", err)
		return
	}

	fmt.Println("Attempting request with 2s timeout to endpoint that delays 3s...")
	resp, err := client.Do(ctx, req)
	if err != nil {
		fmt.Printf("Request timed out as expected: %v\n", err)
		return
	}
	defer resp.Body.Close()

	fmt.Printf("Status: %d\n", resp.StatusCode)
}

// Example 5: Custom client configuration
func customConfiguration(logger *slog.Logger) {
	// Create client with custom settings
	client := httpx.New(logger,
		httpx.WithRetries(5),                       // Retry up to 5 times
		httpx.WithTimeout(30*time.Second),          // 30 second overall timeout
		httpx.WithRetryDelay(200*time.Millisecond), // Start with 200ms delay
		httpx.WithMaxRetryWait(10*time.Second),     // Max 10 seconds between retries
	)

	req, err := http.NewRequest("GET", "https://httpbin.org/get", nil)
	if err != nil {
		log.Printf("Error creating request: %v\n", err)
		return
	}

	// Add custom headers
	req.Header.Set("User-Agent", "hrex-httpx-example/1.0")
	req.Header.Set("X-Custom-Header", "custom-value")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(context.Background(), req)
	if err != nil {
		log.Printf("Error executing request: %v\n", err)
		return
	}
	defer resp.Body.Close()

	fmt.Printf("Status: %d\n", resp.StatusCode)
	fmt.Printf("Client Configuration:\n")
	fmt.Printf("  - Max Retries: 5\n")
	fmt.Printf("  - Timeout: 30s\n")
	fmt.Printf("  - Initial Retry Delay: 200ms\n")
	fmt.Printf("  - Max Retry Wait: 10s\n")
}

// Example 6: Exponential backoff demonstration
func exponentialBackoff(logger *slog.Logger) {
	client := httpx.New(logger,
		httpx.WithRetries(4),
		httpx.WithRetryDelay(100*time.Millisecond),
		httpx.WithMaxRetryWait(5*time.Second),
	)

	// Try an endpoint that might fail
	req, err := http.NewRequest("GET", "https://httpbin.org/status/503", nil)
	if err != nil {
		log.Printf("Error creating request: %v\n", err)
		return
	}

	fmt.Println("Demonstrating exponential backoff...")
	fmt.Println("Delays: 100ms, 200ms, 400ms, 800ms...")
	start := time.Now()
	resp, err := client.Do(context.Background(), req)
	duration := time.Since(start)

	if err != nil {
		fmt.Printf("Request failed: %v\n", err)
		return
	}
	defer resp.Body.Close()

	fmt.Printf("Final Status: %d\n", resp.StatusCode)
	fmt.Printf("Total Duration: %v\n", duration)
	fmt.Println("Note: Each retry doubles the wait time (exponential backoff)")
}

// Example 7: Custom headers and authentication
func customHeaders(logger *slog.Logger) {
	client := httpx.New(logger)

	req, err := http.NewRequest("GET", "https://httpbin.org/headers", nil)
	if err != nil {
		log.Printf("Error creating request: %v\n", err)
		return
	}

	// Add various headers
	req.Header.Set("User-Agent", "hrex-httpx/1.0")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Accept-Language", "th-TH,th;q=0.9,en-US;q=0.8,en;q=0.7")
	req.Header.Set("X-API-Key", "demo-api-key-12345")
	req.Header.Set("X-Request-ID", "req-"+time.Now().Format("20060102150405"))

	resp, err := client.Do(context.Background(), req)
	if err != nil {
		log.Printf("Error executing request: %v\n", err)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	fmt.Printf("Status: %d\n", resp.StatusCode)
	fmt.Printf("Response (first 400 chars): %s...\n", limitString(string(body), 400))
}

// Helper function to limit string length
func limitString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}
