// ref: https://dotat.at/@/2024-09-02-ewma.html
package ratelimit

import (
	"math"
	"sync"
	"time"
)

const (
	cost             = 1.0
	cleanupThreshold = 3 // multiplier for period to determine staleness
)

// Result represents the outcome of a rate limit check
type Result struct {
	Allowed  bool
	NextTime time.Time // When request will be allowed if currently denied
}

// Client represents a rate-limited client with its current state
type Client struct {
	time time.Time // Last update time
	rate float64   // Current rate
}

// RateLimiter manages rate limiting for multiple clients
type RateLimiter struct {
	mu      sync.Mutex
	clients map[string]*Client
	limit   float64       // Rate limit threshold for all clients
	period  time.Duration // Time period for rate calculation
	stop    chan struct{} // Channel to signal stopping the cleanup goroutine
	strict  bool          // Whether to update rate for denied requests
}

// NewRateLimiter creates a new rate limiter with a specified limit and period
func NewRateLimiter(limit float64, period time.Duration, strict bool) *RateLimiter {
	rl := &RateLimiter{
		clients: make(map[string]*Client),
		limit:   limit,
		period:  period,
		stop:    make(chan struct{}),
		strict:  strict,
	}

	// Start cleanup goroutine
	go rl.cleanupLoop()

	return rl
}

// Stop gracefully shuts down the rate limiter and its cleanup goroutine
func (rl *RateLimiter) Stop() {
	close(rl.stop)
}

// cleanupLoop runs the cleanup function periodically
func (rl *RateLimiter) cleanupLoop() {
	ticker := time.NewTicker(rl.period)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			rl.cleanup()
		case <-rl.stop:
			return
		}
	}
}

// cleanup removes stale clients
func (rl *RateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	threshold := time.Now().Add(-rl.period * cleanupThreshold)

	for id, client := range rl.clients {
		if client.time.Before(threshold) {
			delete(rl.clients, id)
		}
	}
}

// calculateNextAllowedTime calculates when the next request would be allowed
func (rl *RateLimiter) calculateNextAllowedTime(now time.Time, rate float64) time.Time {
	// t_next = t_now + period * ln(r_now / limit)
	waitPeriods := math.Log(rate / rl.limit)
	waitDuration := time.Duration(float64(rl.period) * waitPeriods)
	return now.Add(waitDuration)
}

// Check determines if a request can proceed under rate limits
func (rl *RateLimiter) Check(clientID string) Result {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()

	// Get or create client
	client, exists := rl.clients[clientID]
	if !exists {
		client = &Client{
			time: now,
			rate: 0,
		}
		rl.clients[clientID] = client
	}

	// Convert time difference to period units
	interval := now.Sub(client.time).Seconds() / rl.period.Seconds()

	// Clamp interval to avoid division by zero
	interval = math.Max(interval, 1.0e-10)

	// Calculate exponential smoothing weight
	alpha := math.Exp(-interval)

	// Calculate instantaneous rate (cost per period)
	rInst := cost / interval

	// Update average rate using exponential smoothing
	rNow := (1-alpha)*rInst + alpha*client.rate

	// Ensure rare requests are counted in full
	rNow = math.Max(rNow, cost)

	// Check if rate exceeds limit
	if rNow > rl.limit {
		// In strict mode, update client state even for denied requests
		if rl.strict {
			client.time = now
			client.rate = rNow
		}
		return Result{
			Allowed:  false,
			NextTime: rl.calculateNextAllowedTime(now, rNow),
		}
	}

	// Update client state for allowed requests
	client.time = now
	client.rate = rNow

	return Result{
		Allowed:  true,
		NextTime: now,
	}
}

// GetRate returns the current rate for a client
func (rl *RateLimiter) GetRate(clientID string) (float64, bool) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	client, exists := rl.clients[clientID]
	if !exists {
		return 0, false
	}
	return client.rate, true
}

// GetLastUpdate returns the last update time for a client
func (rl *RateLimiter) GetLastUpdate(clientID string) (time.Time, bool) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	client, exists := rl.clients[clientID]
	if !exists {
		return time.Time{}, false
	}
	return client.time, true
}

// RemoveClient removes a client from the rate limiter
func (rl *RateLimiter) RemoveClient(clientID string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	delete(rl.clients, clientID)
}
