package main

import (
	"flag"
	"sync"
)

func main() {
	var config_file string
	flag.StringVar(&config_file, "config", "config.yaml", "config file path")
	flag.Parse()

	if config, err := LoadConfig(config_file); err != nil {
		panic(err)
	} else {
		broker := NewRedisBroker(config.Redis.Addr, config.Redis.Password, config.Redis.DB)
		defer broker.Close()

		var wq sync.WaitGroup

		for _, channel := range config.Channels {
			wq.Add(1)

			rateLimiter := NewRateLimiter(channel.Ratelimit)
			defer rateLimiter.Stop()

			manager := NewHookManager(channel.Name, rateLimiter)

			go (func() {
				defer wq.Done()
				broker.Run(manager)
			})()
		}

		wq.Wait()
	}
}
