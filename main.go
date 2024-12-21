package main

import "time"

func main() {
	broker := NewRedisBroker("redis://localhost:6379/0")
	defer broker.Close()

	manager := NewHookManager("default", NewRateLimiter(20, time.Minute/20))

	broker.Run(manager)
}
