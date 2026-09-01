package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
)

func main() {
	var config_file string
	flag.StringVar(&config_file, "config", "config.yaml", "config file path")
	flag.Parse()

	config, err := LoadConfig(config_file)
	if err != nil {
		log.Fatal(err)
	}

	// Create log file if specified and set log output to it
	if config.LogFile != "" {
		f, err := os.OpenFile(config.LogFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()
		log.SetOutput(f)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	broker := NewRedisBroker(ctx, config.Redis.Addr, config.Redis.Password, config.Redis.DB)
	defer broker.Close()

	// Build every manager before starting any of them so an invalid rate limit
	// is reported up front instead of taking down a running process.
	managers := make([]*HookManager, 0, len(config.Channels))
	for _, channel := range config.Channels {
		hookManager, err := NewHookManager(ctx, broker, channel.Name, channel.Ratelimit)
		if err != nil {
			log.Fatal(err)
		}
		managers = append(managers, hookManager)
	}

	var wg sync.WaitGroup
	for _, hookManager := range managers {
		wg.Add(1)
		go func(hm *HookManager) {
			defer wg.Done()
			hm.Run()
		}(hookManager)
	}

	// Handle termination signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	<-sigCh
	log.Println("shutting down")

	cancel()
	wg.Wait() // every channel has stopped consuming new hooks

	// Drain in-flight deliveries before the redis client closes under them.
	for _, hookManager := range managers {
		hookManager.Stop()
	}
}
