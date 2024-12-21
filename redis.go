package main

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisBroker struct {
	client *redis.Client
}

func (b *RedisBroker) getHook(id string) (*HookEvent, error) {
	ctx := context.Background()

	result, err := b.client.HMGet(ctx, "asynchook:"+id, "url", "payload", "timestamp", "secret").Result()

	if err != nil {
		return nil, err
	}

	if result[0] == nil {
		return nil, fmt.Errorf("missing url in hook %s", id)
	}

	hook := &HookEvent{
		Id:  id,
		Url: result[0].(string),
	}
	if result[1] != nil { // payload
		hook.Payload = result[1].(string)
	}
	if result[2] != nil { // timestamp
		hook.Timestamp = result[2].(uint64)
	}
	if result[3] != nil { // secret
		hook.Secret = result[3].(string)
	}

	return hook, nil
}

func (b *RedisBroker) deleteRawHook(id string) error {
	ctx := context.Background()

	_, err := b.client.Del(ctx, "asynchook:"+id).Result()

	return err
}

func (b *RedisBroker) Run(manager *HookManager) {
	ctx := context.Background()

	for {
		result, _ := b.client.BZPopMin(ctx, time.Minute, "asynchooks:"+manager.Channel).Result()

		if result != nil {
			id := result.Member.(string)

			hook, err := b.getHook(id)
			if err != nil {
				fmt.Println(err)
				continue
			}

			b.deleteRawHook(id)

			manager.Process(hook)
		}
	}
}

func (b *RedisBroker) Close() {
	b.client.Close()
}

func NewRedisBroker(url string) *RedisBroker {
	opt, err := redis.ParseURL(url)
	if err != nil {
		panic(err)
	}

	client := redis.NewClient(opt)

	return &RedisBroker{
		client: client,
	}
}
