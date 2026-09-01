package main

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
)

// maxBurst caps how many tokens may accumulate while a channel is idle.
// Seeding the bucket with a full window's worth of tokens (and letting it
// refill that far) lets an idle channel fire an entire window of hooks
// back-to-back before throttling engages at all.
const maxBurst = 1

type RateLimiter struct {
	tokens    uint64
	maxTokens uint64
	tokenRate time.Duration
	mu        sync.Mutex
	cond      *sync.Cond
	stopCh    chan struct{}
	stopOnce  sync.Once
	stopped   bool
}

func NewRateLimiter(ctx context.Context, rateLimit string) (*RateLimiter, error) {
	tokenRate, err := parseRateLimit(rateLimit)
	if err != nil {
		return nil, err
	}

	rl := &RateLimiter{
		tokens:    0,
		maxTokens: maxBurst,
		tokenRate: tokenRate,
		stopCh:    make(chan struct{}),
	}
	rl.cond = sync.NewCond(&rl.mu)

	go rl.refillTokens(ctx)

	return rl, nil
}

// parseRateLimit turns "<limit>/<s|m|h>" into the interval between two tokens.
func parseRateLimit(rateLimit string) (time.Duration, error) {
	invalid := fmt.Errorf("invalid rate limit %q. Must be in the format of <limit>/<time>. Ex. 20/m 30/s 300/h", rateLimit)

	parts := strings.Split(strings.TrimSpace(rateLimit), "/")
	if len(parts) != 2 {
		return 0, invalid
	}

	limit, err := strconv.ParseUint(strings.TrimSpace(parts[0]), 10, 64)
	if err != nil || limit == 0 {
		return 0, invalid
	}

	var duration time.Duration
	switch strings.TrimSpace(parts[1]) {
	case "s":
		duration = time.Second
	case "m":
		duration = time.Minute
	case "h":
		duration = time.Hour
	default:
		return 0, invalid
	}

	// Guard the division: limit == 0 would panic, and a limit large enough to
	// truncate the interval to zero would panic inside time.NewTicker.
	tokenRate := duration / time.Duration(limit)
	if tokenRate <= 0 {
		return 0, fmt.Errorf("rate limit %q is too high to pace over a %v window", rateLimit, duration)
	}

	return tokenRate, nil
}

func (rl *RateLimiter) refillTokens(ctx context.Context) {
	ticker := time.NewTicker(rl.tokenRate)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			rl.mu.Lock()
			if rl.tokens < rl.maxTokens {
				rl.tokens++
				rl.cond.Signal() // Notify a waiting goroutine that a token is available
			}
			rl.mu.Unlock()
		case <-ctx.Done():
			rl.markStopped()
			return
		case <-rl.stopCh:
			rl.markStopped()
			return
		}
	}
}

// markStopped releases every waiter so callers can unwind during shutdown
// instead of blocking forever on a bucket that will never be refilled.
func (rl *RateLimiter) markStopped() {
	rl.mu.Lock()
	rl.stopped = true
	rl.mu.Unlock()
	rl.cond.Broadcast()
}

// Acquire blocks until a token is available. It reports false if the limiter
// was stopped while waiting, in which case no token was consumed.
func (rl *RateLimiter) Acquire() bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	for rl.tokens == 0 && !rl.stopped {
		rl.cond.Wait() // Wait until a token is available
	}

	if rl.tokens == 0 {
		return false
	}

	rl.tokens--
	return true
}

func (rl *RateLimiter) Stop() {
	rl.stopOnce.Do(func() { close(rl.stopCh) })
}
