package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

func main() {
	var config_file string
	flag.StringVar(&config_file, "config", "config.yaml", "config file path")
	flag.Parse()

	if config, err := LoadConfig(config_file); err != nil {
		panic(err)
	} else {
		ctx, cancel := context.WithCancel(context.Background())

		broker := NewRedisBroker(ctx, config.Redis.Addr, config.Redis.Password, config.Redis.DB)
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

		// Handle termination signals
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

		<-sigCh
		cancel()
		wq.Wait()
	}
}
