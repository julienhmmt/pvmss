# Feature Plan: Shared Resty Client with Connection Pooling

## Goal
Optimize backend resource usage and improve API latency by preventing the continuous recreation of HTTP clients, taking advantage of TCP Keep-Alive and connection pooling.

## Current State
Depending on the current implementation, the backend may be creating new Resty clients (`resty.New()`) for different modules or, in worst cases, per request. This causes overhead due to repeated TCP handshakes and TLS negotiations with the Proxmox API.

## Backend Implementation

1. **Singleton Client Initialization**:
   - Create a centralized, singleton instance of `*resty.Client` at application startup (e.g., within `main.go` or initialized inside `state/manager.go`).
   - This client will be shared across all Proxmox API communication handlers.

2. **Transport & Connection Pooling Tuning**:
   - Configure the underlying `http.Transport` to reuse connections efficiently.
   - Example configuration:
     ```go
     transport := &http.Transport{
         MaxIdleConns:          100,
         MaxIdleConnsPerHost:   20,
         IdleConnTimeout:       90 * time.Second,
         TLSHandshakeTimeout:   10 * time.Second,
         ExpectContinueTimeout: 1 * time.Second,
         // Skip TLS verify if configured
         TLSClientConfig:       &tls.Config{InsecureSkipVerify: skipVerify},
     }
     
     sharedClient := resty.New()
     sharedClient.SetTransport(transport)
     sharedClient.SetTimeout(30 * time.Second)
     ```

3. **Global Configurations**:
   - Apply global settings to the shared Resty client, such as base URLs, default headers, and standard retry mechanisms.
   - Example:
     ```go
     sharedClient.SetRetryCount(3)
     sharedClient.SetRetryWaitTime(1 * time.Second)
     ```

4. **Dependency Injection**:
   - Refactor handlers and aggregator functions to accept the shared `*resty.Client` as a parameter (or access it via the state manager), rather than instantiating their own.
   - Example: `proxmox.GetVMsResty(ctx, sharedClient)`

## Frontend Implementation
- No frontend changes are required. The frontend will implicitly benefit from faster API response times.

## Challenges & Considerations
- **Concurrency Safety**: The `go-resty` client is thread-safe for concurrent request execution, but modifying global client settings (like base URL or timeouts) *after* initialization is not thread-safe. All client configuration must happen once at startup. Request-specific configurations should happen at the request level (`client.R().SetHeader(...)`).
- **Memory Leaks**: Ensure that response bodies are fully read and closed (Resty handles this automatically for the most part) so that connections are successfully returned to the idle pool.
