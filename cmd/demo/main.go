package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"sync"
	"time"

	"github.com/eapache/go-resiliency/batcher"
	"github.com/eapache/go-resiliency/breaker"
	"github.com/eapache/go-resiliency/deadline"
	"github.com/eapache/go-resiliency/retrier"
	"github.com/eapache/go-resiliency/semaphore"
)

// Simulates an external service that sometimes fails
// In a ticketing system, this could represent:
// - Payment processor API calls
// - Venue seating inventory lookup
// - Customer authentication service
func unreliableService(failureRate float64) error {
	if rand.Float64() < failureRate {
		return errors.New("service error: random failure")
	}
	// Simulate some processing time
	time.Sleep(time.Duration(rand.Intn(100)) * time.Millisecond)
	return nil
}

// Demo for Circuit Breaker pattern
// For concert ticketing:
// - Protects system when payment processor is failing
// - Prevents cascading failures during high-volume sales
// - Provides fail-fast response instead of slow failures
func demoCircuitBreaker() {
	fmt.Println("\n=== Circuit Breaker Demo ===")

	// Create a circuit breaker with:
	// - 5 consecutive failures to trip
	// - 1 second wait time before retry
	cb := breaker.New(5, 1, 500*time.Millisecond)

	// Run several operations
	for i := 0; i < 30; i++ {
		result := cb.Run(func() error {
			// Simulate a service with 70% failure rate
			return unreliableService(0.7)
		})

		if result == nil {
			fmt.Printf("Operation %d: Success\n", i)
		} else if errors.Is(result, breaker.ErrBreakerOpen) {
			fmt.Printf("Operation %d: Circuit breaker is triggered\n", i)
		} else {
			fmt.Printf("Operation %d: Failed with error: %v\n", i, result)
		}

		// Small delay between operations
		time.Sleep(100 * time.Millisecond)
	}
}

// Demo for Semaphore pattern
// For concert ticketing:
// - Controls number of concurrent ticket reservation requests
// - Prevents system overload during popular concert on-sales
// - Maintains fair access to limited ticket inventory
func demoSemaphore() {
	fmt.Println("\n=== Concurrency Control Demo ===")

	// Create a semaphore with 3 tickets capacity
	// For ticket sales, this could limit concurrent database connections
	// or number of simultaneous payment processing operations
	sem := semaphore.New(3, 0) // No timeout for simplicity

	var wg sync.WaitGroup

	// Launch 10 concurrent operations
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			err := sem.Acquire()
			if err != nil {
				fmt.Printf("Worker %d: Failed to acquire semaphore: %v\n", id, err)
				return
			}

			fmt.Printf("Worker %d: Acquired semaphore, working...\n", id)
			// Simulate work
			time.Sleep(time.Duration(500+rand.Intn(500)) * time.Millisecond)
			fmt.Printf("Worker %d: Completed work, releasing semaphore\n", id)

			sem.Release()
		}(i)
	}

	wg.Wait()
}

// Demo for Timeout pattern
// For concert ticketing:
// - Enforces time limits on ticket reservation holds
// - Prevents users from blocking inventory indefinitely
// - Ensures responsive UX even when backend services are slow
func demoDeadline() {
	fmt.Println("\n=== Timeout Demo ===")

	// Create a deadline with 500ms timeout
	// In ticketing, timeouts might be longer (e.g., 30 seconds for checkout)
	d := deadline.New(500 * time.Millisecond)

	// Test with operations of different durations
	durations := []time.Duration{
		300 * time.Millisecond,
		600 * time.Millisecond,
		400 * time.Millisecond,
		700 * time.Millisecond,
	}
	for i, duration := range durations {
		result := d.Run(func(cancelCh <-chan struct{}) error {
			select {
			case <-cancelCh:
				return context.DeadlineExceeded
			case <-time.After(duration):
				return nil
			}
		})

		if result == nil {
			fmt.Printf("Operation %d: Completed within deadline (duration: %v)\n", i, duration)
		} else if errors.Is(result, context.DeadlineExceeded) {
			fmt.Printf("Operation %d: Deadline exceeded (duration: %v)\n", i, duration)
		} else {
			fmt.Printf("Operation %d: Failed with error: %v\n", i, result)
		}
	}
}

// Demo for Retrier pattern
// For concert ticketing:
// - Handles transient payment processing failures
// - Retries seat allocation when database contention occurs
// - Improves success rate of ticket fulfillment operations
func demoRetrier() {
	fmt.Println("\n=== Retrier Demo ===")

	// Create a retrier with 5 attempts and exponential backoff
	// Critical for payment processing or database write operations
	r := retrier.New(retrier.ExponentialBackoff(5, 100*time.Millisecond), nil)

	// Test with a service that has a high failure rate
	attempt := 0
	err := r.Run(func() error {
		attempt++
		fmt.Printf("Attempt %d: Trying operation...\n", attempt)

		// 80% failure rate for first few attempts
		var failRate float64
		if attempt < 3 {
			failRate = 0.8
		} else {
			failRate = 0.3 // Better chance of success later
		}

		return unreliableService(failRate)
	})

	if err != nil {
		fmt.Printf("Retrier eventually failed: %v\n", err)
	} else {
		fmt.Printf("Retrier eventually succeeded after %d attempts\n", attempt)
	}
}

// Demo for Batcher pattern
// For concert ticketing:
// - Batches multiple ticket reservations into single inventory updates
// - Reduces database load by consolidating operations
// - Improves throughput during high-volume on-sales
func demoBatcher() {
	fmt.Println("\n=== Batcher Demo ===")

	// Create a batcher with:
	// - Max delay of 1 second
	// - Process function
	b := batcher.New(1*time.Second, func(items []interface{}) error {
		fmt.Printf("Processing batch of %d items\n", len(items))
		// In ticketing, this could batch multiple ticket reservations
		// or multiple payment confirmations into a single database operation
		time.Sleep(200 * time.Millisecond)
		return nil
	})

	// Submit items
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			fmt.Printf("Submitting item %d\n", id)
			err := b.Run(id)
			if err != nil {
				fmt.Printf("Error submitting item %d: %v\n", id, err)
			}
			// Random delay between submissions
			time.Sleep(time.Duration(rand.Intn(300)) * time.Millisecond)
		}(i)
	}

	wg.Wait()
	// Wait a bit for the last batch to process
	time.Sleep(1500 * time.Millisecond)
}

// Simple HTTP service demo using multiple patterns together
// This simulates a complete concert ticketing API endpoint
// that handles high concurrency and maintains resiliency
func demoHttpService() {
	fmt.Println("\n=== HTTP Service with Resiliency Patterns Demo ===")

	// Circuit breaker for the service
	// Protects when downstream services (payment, inventory) fail
	cb := breaker.New(3, 1, 1000*time.Millisecond)

	// Semaphore to limit concurrent requests
	// Controls number of simultaneous ticket purchase attempts
	sem := semaphore.New(5, 0)

	// Setup a handler that uses these patterns
	http.HandleFunc("/resilient", func(w http.ResponseWriter, r *http.Request) {
		err := sem.Acquire()
		if err != nil {
			w.WriteHeader(http.StatusTooManyRequests)
			fmt.Fprintln(w, "Too many concurrent requests")
			return
		}

		defer sem.Release()
		defer fmt.Println("Request completed, semaphore released")

		// Use a deadline for the overall request
		// Ensures ticket reservations don't hold resources indefinitely
		d := deadline.New(2 * time.Second)

		result := d.Run(func(cancelCh <-chan struct{}) error {
			return cb.Run(func() error {
				// Try to make a potentially failing call with retries
				// This could represent payment processing with multiple attempts
				err := retrier.New(retrier.ExponentialBackoff(3, 50*time.Millisecond), nil).Run(func() error {
					// Simulate a flaky external service call
					fmt.Println("Making request to service")
					return unreliableService(0.8)
				})

				if err != nil {
					return err
				}

				select {
				case <-cancelCh:
					return context.DeadlineExceeded
				default:
					fmt.Fprintf(w, "Request processed successfully at %v\n", time.Now())
					return nil
				}
			})
		})

		if result != nil {
			switch {
			case errors.Is(result, breaker.ErrBreakerOpen):
				w.WriteHeader(http.StatusServiceUnavailable)
				fmt.Fprintln(w, "Service is currently unavailable (circuit open)")
			case errors.Is(result, context.DeadlineExceeded):
				w.WriteHeader(http.StatusGatewayTimeout)
				fmt.Fprintln(w, "Request timed out")
			default:
				w.WriteHeader(http.StatusInternalServerError)
				fmt.Fprintf(w, "Error: %v\n", result)
			}
		}
	})

	// Start the server in a goroutine
	go func() {
		fmt.Println("Starting HTTP server on :8080")
		log.Fatal(http.ListenAndServe(":8080", nil))
	}()

	// Make some sample requests to show it working
	fmt.Println("Making some sample requests to our resilient endpoint...")

	var wg sync.WaitGroup
	for i := 0; i < 30; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			resp, err := http.Get("http://localhost:8080/resilient")
			if err != nil {
				fmt.Printf("Request %d failed: %v\n", id, err)
				return
			}
			defer resp.Body.Close()
			fmt.Printf("Request %d: Status %s\n", id, resp.Status)
		}(i)

		// Small delay between requests
		time.Sleep(100 * time.Millisecond)
	}

	wg.Wait()
}

// Main function demonstrating various resiliency patterns
// for a high-traffic concert ticketing system
// These patterns help maintain system stability during:
// - Popular concert on-sales with high traffic spikes
// - Flash sales with thousands of concurrent users
// - Integration with external payment systems
func main() {
	fmt.Println("Go Resiliency Patterns Demo")
	fmt.Println("===========================")

	// Run each demo
	demoCircuitBreaker()
	demoSemaphore()
	demoDeadline()
	demoRetrier()
	demoBatcher()

	// // Integrated demo
	// demoHttpService()

	// Give HTTP server time to serve requests
	fmt.Println("\nDemo complete! Press Ctrl+C to exit.")
}
