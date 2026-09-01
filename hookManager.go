package main

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

const (
	maxRetries = 5

	// Retries back off exponentially rather than by a flat minute, giving a
	// failing endpoint room to recover: 5m, 15m, 45m, 2h15m, 6h45m.
	retryBaseDelay   = 5 * time.Minute
	retryDelayFactor = 3
)

// retryDelay is the gap before the given 1-based retry attempt.
func retryDelay(attempt int) time.Duration {
	delay := retryBaseDelay
	for i := 1; i < attempt; i++ {
		delay *= retryDelayFactor
	}

	return delay
}

type HookManager struct {
	Broker      Broker
	Channel     string
	rateLimiter *RateLimiter
	inFlight    sync.WaitGroup
}

func NewHookManager(ctx context.Context, broker Broker, channel string, rateLimit string) (*HookManager, error) {
	rateLimiter, err := NewRateLimiter(ctx, rateLimit)
	if err != nil {
		return nil, fmt.Errorf("channel %q: %w", channel, err)
	}

	return &HookManager{Broker: broker, Channel: channel, rateLimiter: rateLimiter}, nil
}

func (hm *HookManager) Run() {
	hm.Broker.HookStream(hm.Channel, hm.process)
}

func (hm *HookManager) process(hook *HookEvent) {
	// Expiry is checked first: an expired hook must be dropped even when it
	// also carries a future run_after_time, otherwise the two checks bounce it
	// between the queue and the scheduled set forever.
	if !hook.Expire_time.IsZero() && hook.Expire_time.Before(time.Now()) {
		log.Println("hook event expired", hook.Url)
		hm.Broker.ClearHook(hook.Id)
		return
	}

	if !hook.Run_after_time.IsZero() && hook.Run_after_time.After(time.Now()) {
		log.Println("hook event scheduled to run", hook.Url)
		hm.Broker.ScheduleHook(hm.Channel, hook.Id, hook.Priority, hook.Run_after_time)
		return
	}

	if !hm.rateLimiter.Acquire() {
		// Shutting down. The hook has already been popped, so hand it back to
		// the scheduled set rather than dropping it.
		hm.Broker.ScheduleHook(hm.Channel, hook.Id, hook.Priority, time.Now())
		return
	}

	hm.inFlight.Add(1)
	go func() {
		defer hm.inFlight.Done()

		err := hook.Process()
		if err != nil {
			log.Println(err)
			if hook.Retry_count < maxRetries {
				// Attempts are 1-based: a zero-based delay meant the first
				// retry fired immediately.
				retry := hook.Retry_count + 1
				hm.Broker.UpdateRetryCount(hook.Id, retry)
				hm.Broker.ScheduleHook(hm.Channel, hook.Id, hook.Priority, time.Now().Add(retryDelay(retry)))
			} else {
				hm.Broker.ClearHook(hook.Id)
				log.Println("hook event failed", hook.Url)
			}
		} else {
			hm.Broker.ClearHook(hook.Id)
			log.Println("hook event sent to", hook.Url)
		}
	}()
}

// Stop releases the rate limiter and waits for in-flight deliveries to settle
// so their results are recorded in redis before the broker closes.
func (hm *HookManager) Stop() {
	hm.rateLimiter.Stop()
	hm.inFlight.Wait()
}
