package main

import (
	"fmt"
	"log"
	"time"
)

type HookManager struct {
	Broker      Broker
	Channel     string
	rateLimiter *RateLimiter
}

func NewHookManager(broker Broker, channel string, rateLimit string) *HookManager {
	rateLimiter := NewRateLimiter(rateLimit)

	return &HookManager{broker, channel, rateLimiter}
}

func (hm *HookManager) Run() {
	hm.Broker.HookStream(hm.Channel, hm.process)
}

func (hm *HookManager) process(hook *HookEvent) {
	if !hook.Run_after_time.IsZero() && hook.Run_after_time.After(time.Now()) {
		fmt.Println("hook event scheduled to run", hook.Url)
		hm.Broker.ScheduleHook(hm.Channel, hook.Id, hook.Run_after_time)
		return
	}

	if !hook.Expire_time.IsZero() && hook.Expire_time.Before(time.Now()) {
		fmt.Println("hook event expired", hook.Url)
		hm.Broker.ClearHook(hook.Id)
		return
	}

	hm.rateLimiter.Acquire()

	go func() {
		err := hook.Process()
		if err != nil {
			log.Println(err)
			if hook.Retry_count < 3 {
				hm.Broker.UpdateRetryCount(hook.Id, hook.Retry_count+1)
				hm.Broker.ScheduleHook(hm.Channel, hook.Id, time.Now().Add(time.Minute*time.Duration(hook.Retry_count)))
			} else {
				hm.Broker.ClearHook(hook.Id)
				log.Println("hook event failed", hook.Url)
			}
		} else {
			hm.Broker.ClearHook(hook.Id)
			fmt.Println("hook event sent to", hook.Url)
		}
	}()
}

func (hm *HookManager) Stop() {
	hm.rateLimiter.Stop()
}
