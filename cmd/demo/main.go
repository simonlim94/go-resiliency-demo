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

// Debug levels for logging
const (
	INFO  = "INFO"
	WARN  = "WARN"
	ERROR = "ERROR"
	DEBUG = "DEBUG"
)

// ANSI color codes for terminal output
const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorPurple = "\033[35m"
	colorCyan   = "\033[36m"
	colorWhite  = "\033[37m"
	colorBold   = "\033[1m"
)

// getColorForLevel returns the appropriate color code for each log level
func getColorForLevel(level string) string {
	switch level {
	case INFO:
		return colorGreen
	case WARN:
		return colorYellow
	case ERROR:
		return colorRed
	case DEBUG:
		return colorCyan
	default:
		return colorReset
	}
}

// debugLog provides structured logging with timestamp, severity, and color
func debugLog(level string, component string, message string, data ...interface{}) {
	timestamp := time.Now().Format("15:04:05.000")
	color := getColorForLevel(level)

	// Format the component with bold
	formattedComponent := fmt.Sprintf("%s%-10s%s", colorBold, component, colorReset)

	// Format the level with its specific color
	formattedLevel := fmt.Sprintf("%s%s%s", color, level, colorReset)

	prefix := fmt.Sprintf("[%s] %s %s |", timestamp, formattedLevel, formattedComponent)

	if len(data) > 0 {
		fmt.Printf("%s %s: %v\n", prefix, message, data)
	} else {
		fmt.Printf("%s %s\n", prefix, message)
	}
}

// Simulates an external service that sometimes fails
// In a ticketing system, this could represent:
// - Payment processor API calls
// - Venue seating inventory lookup
// - Customer authentication service
func unreliableService(failureRate float64) error {
	serviceName := []string{"PaymentAPI", "InventoryService", "CustomerAuth"}[rand.Intn(3)]
	debugLog(INFO, serviceName, "Service call initiated")

	if rand.Float64() < failureRate {
		errorType := []string{
			"Connection timeout",
			"Rate limit exceeded",
			"Internal server error",
			"Database contention",
		}[rand.Intn(4)]

		debugLog(ERROR, serviceName, errorType)
		return errors.New("service error: " + errorType)
	}

	// Simulate some processing time
	processingTime := time.Duration(rand.Intn(100)) * time.Millisecond
	time.Sleep(processingTime)

	debugLog(INFO, serviceName, "Service call successful",
		fmt.Sprintf("took=%v", processingTime))
	return nil
}

// Demo for Circuit Breaker pattern
// For concert ticketing:
// - Protects system when payment processor is failing
// - Prevents cascading failures during high-volume sales
// - Provides fail-fast response instead of slow failures
func demoCircuitBreaker() {
	fmt.Println("\n=== Circuit Breaker Demo ===")
	debugLog(INFO, "CircuitBreaker", "Starting demo - simulating payment processing during ticket sales")

	// Create a circuit breaker with:
	// - 5 consecutive failures to trip
	// - 1 second wait time before retry
	cb := breaker.New(5, 1, 500*time.Millisecond)

	// Run several operations
	successCount := 0
	failureCount := 0
	tripCount := 0

	for i := 0; i < 30; i++ {
		requestID := fmt.Sprintf("REQ-%04d", i)
		debugLog(INFO, "PaymentProcessor", "Processing payment", requestID)

		startTime := time.Now()
		result := cb.Run(func() error {
			// Simulate a service with 50% failure rate
			return unreliableService(0.5)
		})
		duration := time.Since(startTime)

		if result == nil {
			successCount++
			debugLog(INFO, "PaymentProcessor", "Payment successful",
				fmt.Sprintf("id=%s duration=%v", requestID, duration))
		} else if errors.Is(result, breaker.ErrBreakerOpen) {
			tripCount++
			debugLog(WARN, "CircuitBreaker", "Circuit triggered - payment rejected",
				fmt.Sprintf("id=%s", requestID))
		} else {
			failureCount++
			debugLog(ERROR, "PaymentProcessor", "Payment failed",
				fmt.Sprintf("id=%s error=%v", requestID, result))
		}

		// Small delay between operations
		time.Sleep(100 * time.Millisecond)
	}

	debugLog(INFO, "CircuitBreaker", "Demo completed",
		fmt.Sprintf("success=%d failures=%d trips=%d", successCount, failureCount, tripCount))
}

// Demo for Semaphore pattern
// For concert ticketing:
// - Controls number of concurrent ticket reservation requests
// - Prevents system overload during popular concert on-sales
// - Maintains fair access to limited ticket inventory
func demoSemaphore() {
	fmt.Println("\n=== Concurrency Control Demo ===")
	debugLog(INFO, "ConcurrencyControl", "Starting demo - simulating ticket reservation requests")

	// Create a semaphore with 3 tickets capacity
	// For ticket sales, this could limit concurrent database connections
	// or number of simultaneous payment processing operations
	sem := semaphore.New(3, 0) // No timeout for simplicity

	var wg sync.WaitGroup
	concurrencyStats := struct {
		rejected  int
		completed int
		inFlight  int
		mu        sync.Mutex
	}{}

	// Launch 10 concurrent operations
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			customerID := fmt.Sprintf("CUST-%04d", id)
			debugLog(INFO, "TicketReservation", "Customer attempting to reserve tickets", customerID)

			err := sem.Acquire()
			if err != nil {
				concurrencyStats.mu.Lock()
				concurrencyStats.rejected++
				concurrencyStats.mu.Unlock()

				debugLog(WARN, "TicketReservation", "Reservation rejected - system at capacity", customerID)
				return
			}

			concurrencyStats.mu.Lock()
			concurrencyStats.inFlight++
			currentInFlight := concurrencyStats.inFlight
			concurrencyStats.mu.Unlock()

			debugLog(INFO, "TicketReservation", "Processing reservation",
				fmt.Sprintf("customer=%s concurrent=%d", customerID, currentInFlight))

			// Simulate work
			processingTime := time.Duration(500+rand.Intn(500)) * time.Millisecond
			time.Sleep(processingTime)

			concurrencyStats.mu.Lock()
			concurrencyStats.inFlight--
			concurrencyStats.completed++
			concurrencyStats.mu.Unlock()

			debugLog(INFO, "TicketReservation", "Reservation completed",
				fmt.Sprintf("customer=%s duration=%v", customerID, processingTime))

			sem.Release()
		}(i)
	}

	wg.Wait()
	debugLog(INFO, "ConcurrencyControl", "Demo completed",
		fmt.Sprintf("completed=%d rejected=%d", concurrencyStats.completed, concurrencyStats.rejected))
}

// Demo for Timeout pattern
// For concert ticketing:
// - Enforces time limits on ticket reservation holds
// - Prevents users from blocking inventory indefinitely
// - Ensures responsive UX even when backend services are slow
func demoDeadline() {
	fmt.Println("\n=== Timeout Demo ===")
	debugLog(INFO, "ReservationTimeout", "Starting demo - enforcing reservation time limits")

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

	timeoutCount := 0
	successCount := 0

	for i, duration := range durations {
		reservationID := fmt.Sprintf("RESV-%04d", i)
		debugLog(INFO, "TicketReservation", "Customer started checkout process",
			fmt.Sprintf("id=%s timeout=500ms", reservationID))

		startTime := time.Now()
		result := d.Run(func(cancelCh <-chan struct{}) error {
			select {
			case <-cancelCh:
				debugLog(WARN, "TicketReservation", "Checkout timed out", reservationID)
				return nil
			case <-time.After(duration):
				debugLog(INFO, "TicketReservation", "Checkout completed",
					fmt.Sprintf("id=%s duration=%v", reservationID, duration))
				return nil
			}
		})
		actualDuration := time.Since(startTime)

		if result == nil {
			successCount++
			debugLog(INFO, "ReservationTimeout", "Reservation confirmed",
				fmt.Sprintf("id=%s processTime=%v", reservationID, actualDuration))
		} else if errors.Is(result, deadline.ErrTimedOut) {
			timeoutCount++
			debugLog(WARN, "ReservationTimeout", "Reservation expired - seats released",
				fmt.Sprintf("id=%s processTime=%v", reservationID, actualDuration))
		} else {
			debugLog(ERROR, "ReservationTimeout", "Unexpected error",
				fmt.Sprintf("id=%s error=%v", reservationID, result))
		}
	}

	debugLog(INFO, "ReservationTimeout", "Demo completed",
		fmt.Sprintf("successful=%d timeouts=%d", successCount, timeoutCount))
}

// Demo for Retrier pattern
// For concert ticketing:
// - Handles transient payment processing failures
// - Retries seat allocation when database contention occurs
// - Improves success rate of ticket fulfillment operations
func demoRetrier() {
	fmt.Println("\n=== Retrier Demo ===")
	debugLog(INFO, "PaymentRetry", "Starting demo - handling transient payment failures")

	// Create a retrier with 5 attempts and exponential backoff
	// Critical for payment processing or database write operations
	r := retrier.New(retrier.ExponentialBackoff(5, 100*time.Millisecond), nil)

	// Test with a service that has a high failure rate
	attempt := 0
	paymentID := fmt.Sprintf("PMT-%04d", rand.Intn(1000))
	debugLog(INFO, "PaymentProcessor", "Processing payment", paymentID)

	startTime := time.Now()
	err := r.Run(func() error {
		attempt++
		debugLog(INFO, "PaymentProcessor", "Payment attempt",
			fmt.Sprintf("id=%s attempt=%d", paymentID, attempt))

		// 80% failure rate for first few attempts
		var failRate float64
		if attempt < 3 {
			failRate = 0.8
		} else {
			failRate = 0.3 // Better chance of success later
		}

		err := unreliableService(failRate)
		if err != nil {
			debugLog(WARN, "PaymentProcessor", "Payment attempt failed",
				fmt.Sprintf("id=%s attempt=%d error=%v", paymentID, attempt, err))
		}
		return err
	})
	duration := time.Since(startTime)

	if err != nil {
		debugLog(ERROR, "PaymentProcessor", "Payment ultimately failed after multiple attempts",
			fmt.Sprintf("id=%s attempts=%d duration=%v", paymentID, attempt, duration))
	} else {
		debugLog(INFO, "PaymentProcessor", "Payment eventually succeeded",
			fmt.Sprintf("id=%s attempts=%d duration=%v", paymentID, attempt, duration))
	}
}

// Demo for Batcher pattern
// For concert ticketing:
// - Batches multiple ticket reservations into single inventory updates
// - Reduces database load by consolidating operations
// - Improves throughput during high-volume on-sales
func demoBatcher() {
	fmt.Println("\n=== Batcher Demo ===")
	debugLog(INFO, "BatchProcessor", "Starting demo - batching ticket operations")

	batchesProcessed := 0
	var batchesMutex sync.Mutex

	// Create a batcher with:
	// - Max delay of 1 second
	// - Process function
	b := batcher.New(1*time.Second, func(items []interface{}) error {
		batchID := fmt.Sprintf("BATCH-%04d", batchesProcessed)
		debugLog(INFO, "InventoryUpdate", "Processing batch of reservations",
			fmt.Sprintf("id=%s count=%d", batchID, len(items)))

		// In ticketing, this could batch multiple ticket reservations
		// or multiple payment confirmations into a single database operation
		time.Sleep(200 * time.Millisecond)

		batchesMutex.Lock()
		batchesProcessed++
		batchesMutex.Unlock()

		debugLog(INFO, "InventoryUpdate", "Batch completed",
			fmt.Sprintf("id=%s itemsProcessed=%v", batchID, items))
		return nil
	})

	// Submit items
	var wg sync.WaitGroup
	totalItems := 0

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			reservationID := fmt.Sprintf("RESV-%04d", id)
			debugLog(INFO, "TicketReservation", "Submitting reservation for processing", reservationID)

			err := b.Run(reservationID)
			if err != nil {
				debugLog(ERROR, "TicketReservation", "Error submitting reservation",
					fmt.Sprintf("id=%s error=%v", reservationID, err))
			} else {
				debugLog(INFO, "TicketReservation", "Reservation queued in batch", reservationID)

				batchesMutex.Lock()
				totalItems++
				batchesMutex.Unlock()
			}

			// Random delay between submissions
			time.Sleep(time.Duration(rand.Intn(300)) * time.Millisecond)
		}(i)
	}

	wg.Wait()
	// Wait a bit for the last batch to process
	time.Sleep(1500 * time.Millisecond)

	debugLog(INFO, "BatchProcessor", "Demo completed",
		fmt.Sprintf("itemsSubmitted=%d batchesProcessed=%d", totalItems, batchesProcessed))
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
	debugLog(INFO, "TicketSystem", "Starting concert ticketing system resilience demo")

	// Run each demo
	// demoCircuitBreaker()
	// demoSemaphore()
	// demoDeadline()
	// demoRetrier()
	demoBatcher()

	// // Integrated demo
	// demoHttpService()

	// Give HTTP server time to serve requests
	debugLog(INFO, "TicketSystem", "Demo complete! Press Ctrl+C to exit.")
	fmt.Println("\nDemo complete! Press Ctrl+C to exit.")
}
