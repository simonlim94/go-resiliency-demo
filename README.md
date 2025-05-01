# Go Resiliency Demo

This project demonstrates the various resiliency patterns implemented in the [go-resiliency](https://github.com/eapache/go-resiliency) library.

## Patterns Demonstrated

1. **Circuit Breaker** - Prevents cascading failures by breaking the circuit after a certain number of consecutive errors
2. **Semaphore** - Limits concurrent access to a resource
3. **Deadline** - Sets a timeout for operations
4. **Retrier** - Automatically retries failed operations with configurable backoff
5. **Batcher** - Batches multiple operations together for efficiency

## Running the Demo

```bash
go run cmd/demo/main.go
```

## Demo Structure

The demo includes standalone examples of each pattern, as well as an integrated HTTP service that combines multiple patterns to create a resilient API endpoint.

### Circuit Breaker

Demonstrates how a circuit breaker trips after multiple consecutive failures and prevents further calls until a cooldown period has elapsed.

### Semaphore

Shows how a semaphore can limit concurrent access to a resource, preventing overload.

### Deadline

Illustrates how operations can be automatically cancelled if they exceed a specified timeout.

### Retrier

Demonstrates automatic retries of failed operations with exponential backoff.

### Batcher

Shows how multiple operations can be batched together for efficiency.

### Integrated HTTP Service

Combines all the patterns above into a resilient HTTP service that can handle failures gracefully.

## License

MIT 