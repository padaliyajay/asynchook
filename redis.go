package main

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisBroker struct {
	client *redis.Client
	ctx    context.Context
}

func (b *RedisBroker) getHook(id string) (*HookEvent, error) {
	result, err := b.client.HMGet(b.ctx, "asynchook:"+id, "url", "payload", "timestamp", "secret").Result()

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
		hook.Timestamp = result[2].(string)
	}
	if result[3] != nil { // secret
		hook.Secret = result[3].(string)
	}

	return hook, nil
}

func (b *RedisBroker) deleteRawHook(id string) error {
	_, err := b.client.Del(b.ctx, "asynchook:"+id).Result()

	return err
}

func (b *RedisBroker) Run(manager *HookManager) {
	for {
		select {
		case <-b.ctx.Done():
			return
		default:
			result, _ := b.client.BZPopMin(b.ctx, time.Second*10, "asynchooks:"+manager.Channel).Result()

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
}

func (b *RedisBroker) Close() {
	b.client.Close()
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
