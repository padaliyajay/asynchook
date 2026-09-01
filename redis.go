package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// defaultPriority matches the score used in the README's ZADD example, and is
// the fallback when a scheduled hook has no recorded priority.
const defaultPriority = 1

// opTimeout bounds the bookkeeping writes that must still run during shutdown.
const opTimeout = 5 * time.Second

// popTimeout is how long a single blocking pop waits before looping. go-redis
// does not abort an in-flight blocking read when the context is cancelled, so
// this also bounds how long shutdown waits for the consumer to notice.
const popTimeout = 5 * time.Second

// errInvalidHook marks a hook whose stored fields cannot be used. Only these
// are safe to discard; a transport error says nothing about the hook itself.
var errInvalidHook = errors.New("invalid hook")

type RedisBroker struct {
	client *redis.Client
	ctx    context.Context
}

func NewRedisBroker(ctx context.Context, addr string, password string, db int) *RedisBroker {
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	return &RedisBroker{
		client: client,
		ctx:    ctx,
	}
}

// opCtx is for writes that record the outcome of a hook. They must survive
// cancellation, otherwise a hook delivered during shutdown is never cleared
// and is left orphaned in redis.
func (b *RedisBroker) opCtx() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.WithoutCancel(b.ctx), opTimeout)
}

// sleep waits for d, or returns early once the broker is shutting down.
func (b *RedisBroker) sleep(d time.Duration) {
	select {
	case <-b.ctx.Done():
	case <-time.After(d):
	}
}

// HookStream blocks until the broker's context is cancelled so the caller can
// wait for the channel to stop consuming before it drains.
func (b *RedisBroker) HookStream(channel string, cb func(*HookEvent)) {
	var wg sync.WaitGroup

	wg.Add(2)
	go func() {
		defer wg.Done()
		b.consumeQueue(channel, cb)
	}()
	go func() {
		defer wg.Done()
		b.promoteScheduled(channel)
	}()

	wg.Wait()
}

// consumeQueue pops due hooks and hands them to cb one at a time.
func (b *RedisBroker) consumeQueue(channel string, cb func(*HookEvent)) {
	key := "asynchooks:" + channel

	for {
		if b.ctx.Err() != nil {
			return
		}

		result, err := b.client.BZPopMin(b.ctx, popTimeout, key).Result()
		if err != nil {
			if errors.Is(err, redis.Nil) || b.ctx.Err() != nil {
				continue // the blocking pop timed out, or we are shutting down
			}
			// Redis is unreachable. Back off instead of spinning on the error,
			// which previously burned a core for the length of the outage.
			log.Println("failed to read", key, err)
			b.sleep(time.Second)
			continue
		}

		if result == nil {
			continue
		}

		id, ok := result.Member.(string)
		if !ok {
			log.Println("unexpected member type in", key)
			continue
		}

		hook, err := b.getHook(id)
		if err != nil {
			log.Println(err)
			if errors.Is(err, errInvalidHook) {
				b.deleteRawHook(id)
			} else {
				// A read failure says nothing about the hook. It has already
				// been popped, so put it back rather than destroying it.
				b.requeueHook(key, id, result.Score)
			}
			continue
		}
		hook.Priority = result.Score

		cb(hook)
	}
}

// promoteScheduled moves hooks whose delay has elapsed back onto the queue.
func (b *RedisBroker) promoteScheduled(channel string) {
	scheduledKey := "asynchooks-scheduled:" + channel
	queueKey := "asynchooks:" + channel

	for {
		if b.ctx.Err() != nil {
			return
		}

		ids, err := b.client.ZRangeByScore(b.ctx, scheduledKey, &redis.ZRangeBy{
			Min:    "-inf",
			Max:    strconv.FormatInt(time.Now().Unix(), 10),
			Offset: 0,
			Count:  100,
		}).Result()
		if err != nil {
			if b.ctx.Err() == nil {
				log.Println("failed to read", scheduledKey, err)
			}
			b.sleep(time.Second)
			continue
		}

		for _, id := range ids {
			priority := b.hookPriority(id)

			// Remove first: only the caller that actually removed the id may
			// requeue it, so the hook cannot be queued twice.
			removed, err := b.client.ZRem(b.ctx, scheduledKey, id).Result()
			if err != nil || removed == 0 {
				continue
			}

			// Restore the original priority so retried and scheduled hooks do
			// not preempt everything already waiting in the queue.
			if err := b.client.ZAdd(b.ctx, queueKey, redis.Z{Score: priority, Member: id}).Err(); err != nil {
				log.Println("failed to requeue hook", id, err)
			}
		}

		b.sleep(time.Second)
	}
}

// requeueHook returns a popped hook to the queue at the score it came from.
func (b *RedisBroker) requeueHook(queueKey string, id string, priority float64) {
	ctx, cancel := b.opCtx()
	defer cancel()

	if err := b.client.ZAdd(ctx, queueKey, redis.Z{Score: priority, Member: id}).Err(); err != nil {
		log.Println("failed to requeue hook", id, err)
	}
}

// hookPriority reads the queue score recorded when the hook was deferred.
func (b *RedisBroker) hookPriority(id string) float64 {
	value, err := b.client.HGet(b.ctx, "asynchook:"+id, "priority").Result()
	if err != nil {
		return defaultPriority
	}

	priority, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return defaultPriority
	}

	return priority
}

func (b *RedisBroker) ScheduleHook(channel string, id string, priority float64, runAfter time.Time) {
	ctx, cancel := b.opCtx()
	defer cancel()

	// Remember the priority so promoteScheduled can put the hook back at the
	// position it came from.
	if err := b.client.HSet(ctx, "asynchook:"+id, "priority", priority).Err(); err != nil {
		log.Println(err)
	}

	if err := b.client.ZAdd(ctx, "asynchooks-scheduled:"+channel, redis.Z{Score: float64(runAfter.Unix()), Member: id}).Err(); err != nil {
		log.Println(err)
	}
}

func (b *RedisBroker) UpdateRetryCount(id string, retryCount int) {
	ctx, cancel := b.opCtx()
	defer cancel()

	if err := b.client.HSet(ctx, "asynchook:"+id, "retry_count", retryCount).Err(); err != nil {
		log.Println(err)
	}
}

func (b *RedisBroker) getHook(id string) (*HookEvent, error) {
	result, err := b.client.HMGet(b.ctx, "asynchook:"+id, "url", "payload", "secret", "run_after_time", "expire_time", "retry_count").Result()

	if err != nil {
		return nil, fmt.Errorf("failed to read hook %s: %w", id, err)
	}

	if result[0] == nil {
		return nil, fmt.Errorf("%w: missing url in hook %s", errInvalidHook, id)
	}

	hook := &HookEvent{
		Id:          id,
		Url:         result[0].(string),
		Priority:    defaultPriority,
		Retry_count: 0,
	}
	if result[1] != nil { // payload
		hook.Payload = result[1].(string)
	}
	if result[2] != nil { // secret
		hook.Secret = result[2].(string)
	}
	if result[3] != nil { // run_after_time
		unixTime, err := strconv.ParseInt(result[3].(string), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("%w: error parsing run_after_time for %s: %v", errInvalidHook, id, err)
		}
		hook.Run_after_time = time.Unix(unixTime, 0)
	}
	if result[4] != nil { // expire_time
		unixTime, err := strconv.ParseInt(result[4].(string), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("%w: error parsing expire_time for %s: %v", errInvalidHook, id, err)
		}
		hook.Expire_time = time.Unix(unixTime, 0)
	}
	if result[5] != nil { // retry_count
		retry_count, _ := strconv.Atoi(result[5].(string))
		hook.Retry_count = retry_count
	}

	return hook, nil
}

func (b *RedisBroker) deleteRawHook(id string) error {
	ctx, cancel := b.opCtx()
	defer cancel()

	return b.client.Del(ctx, "asynchook:"+id).Err()
}

func (b *RedisBroker) ClearHook(id string) {
	if err := b.deleteRawHook(id); err != nil {
		log.Println(err)
	}
}

func (b *RedisBroker) Close() {
	b.client.Close()
}
