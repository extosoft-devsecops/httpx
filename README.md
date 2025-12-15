# hrex-httpx

A robust, production-ready HTTP client library for Go with built-in retry logic, exponential backoff, and comprehensive
logging.

## Features

- ✅ **Smart Retry Logic** - Automatically retries on transient errors (5xx, 429, 408)
- ✅ **Exponential Backoff** - Progressive delay increases between retry attempts
- ✅ **Request Body Preservation** - Safely retries requests with body content
- ✅ **Comprehensive Logging** - Detailed HTTP request/response logging with structured logs
- ✅ **Context Aware** - Respects context cancellation and timeouts
- ✅ **Highly Configurable** - Customize retries, timeouts, and logging behavior
- ✅ **Production Ready** - 94% test coverage with comprehensive test suite

## Installation

```bash
go get extosoft.com/hrex/httpx
```

## Quick Start

### Basic Usage

```go
package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"extosoft-devsecops/hrex-http/httpx"
)

func main() {
	// Create a logger
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	// Create HTTP client with default settings
	client := httpx.New(logger)

	// Make a request
	req, _ := http.NewRequest("GET", "https://api.example.com/data", nil)
	resp, err := client.Do(context.Background(), req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	fmt.Printf("Status: %d\n", resp.StatusCode)
}
```

### Advanced Configuration

```go
client := httpx.New(logger,
httpx.WithRetries(5), // Retry up to 5 times
httpx.WithTimeout(30*time.Second), // 30 second timeout
httpx.WithRetryDelay(200*time.Millisecond), // Initial retry delay
httpx.WithMaxRetryWait(10*time.Second), // Maximum retry delay
)
```

## Configuration Options

### `WithRetries(n int)`

Sets the maximum number of retry attempts (default: 1).

```go
client := httpx.New(logger, httpx.WithRetries(3))
```

### `WithTimeout(duration time.Duration)`

Sets the HTTP client timeout (default: 10 seconds).

```go
client := httpx.New(logger, httpx.WithTimeout(30*time.Second))
```

### `WithRetryDelay(duration time.Duration)`

Sets the initial retry delay for exponential backoff (default: 100ms).

```go
client := httpx.New(logger, httpx.WithRetryDelay(200*time.Millisecond))
```

### `WithMaxRetryWait(duration time.Duration)`

Sets the maximum retry delay cap (default: 5 seconds).

```go
client := httpx.New(logger, httpx.WithMaxRetryWait(10*time.Second))
```

## Retry Behavior

### Automatic Retries

The client automatically retries requests in the following cases:

- **Network Errors** - Connection failures, timeouts, etc.
- **429 Too Many Requests** - Rate limiting
- **408 Request Timeout** - Server timeout
- **5xx Server Errors** - Internal server errors (500-599)

### No Retries

The client does NOT retry for:

- **4xx Client Errors** - Bad Request (400), Unauthorized (401), Not Found (404), etc.
- **Successful Responses** - 2xx and 3xx status codes

### Exponential Backoff

Retry delays increase exponentially:

- 1st retry: 100ms (configurable)
- 2nd retry: 200ms
- 3rd retry: 400ms
- 4th retry: 800ms
- And so on, capped at `MaxRetryWait`

## Logging

### Logger Package

The library includes a comprehensive logging round tripper that logs all HTTP requests and responses.

```go
import "extosoft-devsecops/hrex-http/httpx/logger"

// Create a logging round tripper
transport := logger.NewLoggingRoundTripper(
log,
http.DefaultTransport,
logger.WithBodyLogging(true), // Enable body logging
logger.WithMaxBodySize(5*1024*1024), // 5MB max body size
)

client := &http.Client{Transport: transport}
```

### Logging Options

#### `WithBodyLogging(enabled bool)`

Enable/disable request and response body logging (default: false).

```go
logger.WithBodyLogging(true)
```

#### `WithMaxBodySize(size int64)`

Set maximum body size to log in bytes (default: 5MB).

```go
logger.WithMaxBodySize(10*1024*1024) // 10MB
```

### Log Levels

- **DEBUG** - Request/response bodies (when enabled)
- **INFO** - Successful request completion with timing
- **WARN** - Retry attempts and body read failures
- **ERROR** - Request failures after all retries

### Example Log Output

```json
{
  "time": "2025-12-12T10:30:00Z",
  "level": "INFO",
  "msg": "http request completed",
  "method": "POST",
  "url": "https://api.example.com/users",
  "status": 200,
  "duration": "125ms"
}
```

## Examples

### POST Request with Body

```go
body := strings.NewReader(`{"name":"John","email":"john@example.com"}`)
req, _ := http.NewRequest("POST", "https://api.example.com/users", body)
req.Header.Set("Content-Type", "application/json")

resp, err := client.Do(context.Background(), req)
if err != nil {
log.Fatal(err)
}
defer resp.Body.Close()
```

### Request with Context Timeout

```go
ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
defer cancel()

req, _ := http.NewRequest("GET", "https://api.example.com/slow-endpoint", nil)
resp, err := client.Do(ctx, req)
if err != nil {
if errors.Is(err, context.DeadlineExceeded) {
log.Println("Request timed out")
}
return
}
defer resp.Body.Close()
```

### Retry with Custom Configuration

```go
client := httpx.New(logger,
httpx.WithRetries(5),
httpx.WithRetryDelay(500*time.Millisecond),
httpx.WithMaxRetryWait(30*time.Second),
)

req, _ := http.NewRequest("GET", "https://flaky-api.example.com/data", nil)
resp, err := client.Do(context.Background(), req)
// Will retry up to 5 times with exponential backoff
```

## Architecture

### Package Structure

```
hrex-httpx/
├── httpx.go           # Main HTTP client with retry logic
├── httpx_test.go      # Comprehensive test suite (94% coverage)
├── logger/
│   ├── logger.go      # HTTP logging round tripper
│   └── logger_test.go # Logger test suite (90.9% coverage)
├── go.mod
└── README.md
```

### Client Interface

```go
type Client interface {
Do(ctx context.Context, req *http.Request) (*http.Response, error)
}
```

The client implements a simple interface compatible with the standard `http.Client` pattern.

## Testing

### Run Tests

```bash
# Run all tests
go test ./...

# Run with coverage
go test ./... -cover

# Run with verbose output
go test ./... -v

# Generate coverage report
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out
```

### Test Coverage

- **httpx package**: 94.0% coverage
- **logger package**: 90.9% coverage
- **Total**: 18 test cases with 26 sub-tests

## Performance Considerations

### Request Body Handling

The client reads and stores request bodies in memory to enable retries. For very large request bodies:

1. Consider disabling retries for those requests
2. Use streaming uploads without retry logic
3. Monitor memory usage

### Retry Delays

Default retry delays are optimized for most use cases:

- Initial delay: 100ms
- Maximum delay: 5s

Adjust based on your API's rate limiting and timeout policies.

### Context Cancellation

The client respects context cancellation and will immediately stop retrying when the context is cancelled. Use
appropriate timeouts:

```go
ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
defer cancel()
```

## Best Practices

### 1. Use Context with Timeout

Always use context with appropriate timeout to prevent indefinite hangs:

```go
ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()
```

### 2. Close Response Bodies

Always close response bodies to prevent resource leaks:

```go
resp, err := client.Do(ctx, req)
if err != nil {
return err
}
defer resp.Body.Close()
```

### 3. Configure Retries Appropriately

- **Idempotent operations** (GET, PUT, DELETE): Enable retries
- **Non-idempotent operations** (POST): Consider the implications
- **Critical operations**: May want to disable retries and handle manually

### 4. Monitor and Log

Enable body logging in development but disable in production for sensitive data:

```go
bodyLogging := os.Getenv("ENV") == "development"
// Use bodyLogging in logger configuration
```

### 5. Set Reasonable Timeouts

Configure timeouts based on your API's expected response times:

```go
client := httpx.New(logger,
httpx.WithTimeout(30*time.Second), // Overall request timeout
httpx.WithRetries(3), // Max 3 retries
httpx.WithMaxRetryWait(10*time.Second), // Max 10s between retries
)
```

## Error Handling

### Error Types

The client returns detailed error messages:

```go
resp, err := client.Do(ctx, req)
if err != nil {
// Check for specific error types
if errors.Is(err, context.DeadlineExceeded) {
// Timeout
} else if errors.Is(err, context.Canceled) {
// Context cancelled
} else {
// Other errors (network, etc.)
}
}
```

### Retry Error Messages

When retries are exhausted:

```
request failed after 3 attempts: <original error>
```

## Requirements

- Go 1.25.1 or higher
- No external dependencies (uses only standard library)

## License

Copyright © 2025 Extosoft

## Contributing

Contributions are welcome! Please ensure:

1. All tests pass: `go test ./...`
2. Code coverage remains high
3. Follow existing code style
4. Add tests for new features

## Support

For issues, questions, or contributions, please contact the Extosoft team.

## Changelog

### v1.0.0 (2025-12-12)

- Initial release
- Smart retry logic with exponential backoff
- Comprehensive HTTP logging
- 94% test coverage
- Production-ready implementation

