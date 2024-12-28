package main

import "time"

type Broker interface {
	HookStream(channel string, cb func(*HookEvent))

	ClearHook(id string)

	ScheduleHook(channel string, id string, runAfter time.Time)

	UpdateRetryCount(id string, retryCount int)
}
