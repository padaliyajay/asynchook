package main

import "time"

type Broker interface {
	// HookStream consumes the channel until the broker's context is cancelled.
	HookStream(channel string, cb func(*HookEvent))

	ClearHook(id string)

	// ScheduleHook defers a hook until runAfter, preserving the queue priority
	// it had so it does not jump ahead of work already queued.
	ScheduleHook(channel string, id string, priority float64, runAfter time.Time)

	UpdateRetryCount(id string, retryCount int)
}
