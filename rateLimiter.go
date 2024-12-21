package main

import (
	"sync"
	"time"
)

type RateLimiter struct {
	tokens    int
	maxTokens int
	tokenRate time.Duration
	mu        sync.Mutex
	cond      *sync.Cond
	stopCh    chan struct{}
}

func NewRateLimiter(maxTokens int, refillRate time.Duration) *RateLimiter {
	rl := &RateLimiter{
		tokens:    maxTokens,
		maxTokens: maxTokens,
		tokenRate: refillRate,
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
