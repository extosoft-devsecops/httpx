# hrex-httpx Examples

This directory contains working examples demonstrating how to use the hrex-httpx HTTP client library.

## Running the Examples

```bash
cd examples
go run main.go
```

## Examples Included

### Example 1: Simple GET Request
Basic GET request to fetch data from an API.

```go
client := httpx.New(logger)
req, _ := http.NewRequest("GET", "https://httpbin.org/get", nil)
resp, _ := client.Do(context.Background(), req)
```

### Example 2: POST Request with Body
Sending JSON data in a POST request.

```go
jsonData := `{"name":"John Doe","email":"john@example.com"}`
req, _ := http.NewRequest("POST", "https://httpbin.org/post", strings.NewReader(jsonData))
req.Header.Set("Content-Type", "application/json")
resp, _ := client.Do(context.Background(), req)
```

### Example 3: Automatic Retry on Server Error
Demonstrates automatic retry when server returns 5xx errors.

```go
client := httpx.New(logger,
    httpx.WithRetries(3),
    httpx.WithRetryDelay(200*time.Millisecond),
)
// Request to endpoint that returns 500
// Client will automatically retry
```

### Example 4: Request with Context Timeout
Using context to set request timeout.

```go
ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
defer cancel()

req, _ := http.NewRequest("GET", "https://httpbin.org/delay/3", nil)
resp, err := client.Do(ctx, req)
// Will timeout after 2 seconds
```

### Example 5: Custom Client Configuration
Creating a client with custom settings.

```go
client := httpx.New(logger,
    httpx.WithRetries(5),                       // Max 5 retries
    httpx.WithTimeout(30*time.Second),          // 30s timeout
    httpx.WithRetryDelay(200*time.Millisecond), // Initial delay
    httpx.WithMaxRetryWait(10*time.Second),     // Max delay cap
)
```

### Example 6: Exponential Backoff
Shows how retry delays increase exponentially.

```go
client := httpx.New(logger,
    httpx.WithRetries(4),
    httpx.WithRetryDelay(100*time.Millisecond),
    httpx.WithMaxRetryWait(5*time.Second),
)
// Delays: 100ms, 200ms, 400ms, 800ms...
```

### Example 7: Custom Headers
Adding custom headers to requests.

```go
req.Header.Set("User-Agent", "hrex-httpx/1.0")
req.Header.Set("Accept", "application/json")
req.Header.Set("X-API-Key", "your-api-key")
```

## Expected Output

When you run the examples, you'll see:

1. **Successful responses** from httpbin.org endpoints
2. **Retry attempts** logged for server errors
3. **Timeout behavior** for slow endpoints
4. **Exponential backoff** timing in action
5. **Custom headers** being sent and echoed back

## API Endpoints Used

All examples use [httpbin.org](https://httpbin.org) endpoints:

- `GET /get` - Returns GET data
- `POST /post` - Returns POST data
- `GET /status/:code` - Returns specified status code
- `GET /delay/:n` - Delays response by n seconds
- `GET /headers` - Returns request headers

## Customization

Feel free to modify these examples to:
- Use your own API endpoints
- Add authentication
- Test different retry strategies
- Experiment with timeouts
- Try different HTTP methods

## Troubleshooting

### Network Issues
If you get connection errors, check your internet connection and firewall settings.

### Timeout Errors
Adjust timeout values if your network is slow:
```go
httpx.WithTimeout(60*time.Second)
```

### Too Many Retries
Reduce retry attempts for faster feedback:
```go
httpx.WithRetries(2)
```

## Learn More

See the main [README.md](../README.md) for complete documentation.

