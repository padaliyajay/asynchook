package main

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"
)

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

func (b *RedisBroker) HookStream(channel string, cb func(*HookEvent)) {
	// Start a goroutine to listen for new hooks
	go func() {
		for {
			select {
			case <-b.ctx.Done():
				return
			default:
				result, _ := b.client.BZPopMin(b.ctx, time.Minute, "asynchooks:"+channel).Result()

				if result != nil {
					id := result.Member.(string)

					hook, err := b.getHook(id)
					if err != nil {
						b.deleteRawHook(id)
						log.Println(err)
						continue
					}

					cb(hook)
				}
			}
		}
	}()

	// Start a goroutine to check for scheduled hooks
	go func() {
		for {
			select {
			case <-b.ctx.Done():
				return
			default:
				results, _ := b.client.ZRangeByScore(b.ctx, "asynchooks-scheduled:"+channel, &redis.ZRangeBy{
					Min:    "-inf",
					Max:    strconv.FormatInt(time.Now().Unix(), 10),
					Offset: 0,
					Count:  100,
				}).Result()

				for _, id := range results {
					b.client.ZRem(b.ctx, "asynchooks-scheduled:"+channel, id)
					b.client.ZAdd(b.ctx, "asynchooks:"+channel, redis.Z{Score: 0, Member: id})
				}

				time.Sleep(time.Second)
			}
		}
	}()
}

func (b *RedisBroker) ScheduleHook(channel string, id string, runAfter time.Time) {
	_, err := b.client.ZAdd(b.ctx, "asynchooks-scheduled:"+channel, redis.Z{Score: float64(runAfter.Unix()), Member: id}).Result()
	if err != nil {
		log.Println(err)
	}
}

func (b *RedisBroker) UpdateRetryCount(id string, retryCount int) {
	_, err := b.client.HSet(b.ctx, "asynchook:"+id, "retry_count", retryCount).Result()
	if err != nil {
		log.Println(err)
	}
}

func (b *RedisBroker) getHook(id string) (*HookEvent, error) {
	result, err := b.client.HMGet(b.ctx, "asynchook:"+id, "url", "payload", "secret", "run_after_time", "expire_time", "retry_count").Result()

	if err != nil {
		return nil, err
	}

	if result[0] == nil {
		return nil, fmt.Errorf("missing url in hook %s", id)
	}

	hook := &HookEvent{
		Id:          id,
		Url:         result[0].(string),
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
			return nil, fmt.Errorf("error parsing run_after_time: %v", err)
		}
		hook.Run_after_time = time.Unix(unixTime, 0)
	}
	if result[4] != nil { // expire_time
		unixTime, err := strconv.ParseInt(result[4].(string), 10, 64)
		if err != nil {
			return nil, fmt.Errorf("error parsing expire_time: %v", err)
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
	_, err := b.client.Del(b.ctx, "asynchook:"+id).Result()

	return err
}

func (b *RedisBroker) ClearHook(id string) {
	b.deleteRawHook(id)
}

func (b *RedisBroker) Close() {
	b.client.Close()
}
