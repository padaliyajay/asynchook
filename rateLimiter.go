package main

import (
	"log"
	"strconv"
	"strings"
	"sync"
	"time"
)

type RateLimiter struct {
	tokens    uint64
	maxTokens uint64
	tokenRate time.Duration
	mu        sync.Mutex
	cond      *sync.Cond
	stopCh    chan struct{}
}

func NewRateLimiter(rateLimit string) *RateLimiter {
	parts := strings.Split(rateLimit, "/")
	if len(parts) != 2 {
		log.Panic("invalid rate limit. Must be in the format of <limit>/<time>. Ex. 20/m 30/s 300/h")
	}

	limit, err := strconv.ParseUint(parts[0], 10, 64)
	if err != nil {
		log.Panic("invalid rate limit. Must be in the format of <limit>/<time>. Ex. 20/m 30/s 300/h")
	}

	var duration time.Duration
	switch parts[1] {
	case "s":
		duration = time.Second
	case "m":
		duration = time.Minute
	case "h":
		duration = time.Hour
	default:
		log.Panic("invalid rate limit. Must be in the format of <limit>/<time>. Ex. 20/m 30/s 300/h")
	}

	rl := &RateLimiter{
		tokens:    limit,
		maxTokens: limit,
		tokenRate: duration / time.Duration(limit),
		stopCh:    make(chan struct{}),
	}
	rl.cond = sync.NewCond(&rl.mu)

	go rl.refillTokens()

	return rl
}

func (rl *RateLimiter) refillTokens() {
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
		case <-rl.stopCh:
			return
		}
	}
}

func (rl *RateLimiter) Acquire() {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	for rl.tokens == 0 {
		rl.cond.Wait() // Wait until a token is available
	}

	rl.tokens--
}

func (rl *RateLimiter) Stop() {
	close(rl.stopCh)
}
