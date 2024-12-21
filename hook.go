package main

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

type HookEvent struct {
	Id        string
	Url       string
	Payload   string
	Timestamp uint64
	Secret    string
}

func (h *HookEvent) Process() error {
	data := url.Values{}
	data.Set("payload", h.Payload)
	data.Set("timestamp", strconv.FormatUint(h.Timestamp, 10))
	data.Set("secret", h.Secret)

	resp, err := http.PostForm(h.Url, data)

	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("failed to send hook event to %s: %s", h.Url, resp.Status)
	}

	fmt.Println("hook event sent to", h.Url)

	return nil
}

type HookManager struct {
	Channel string
	rl      *RateLimiter
}

func NewHookManager(channel string, rl *RateLimiter) *HookManager {
	return &HookManager{channel, rl}
}

func (s *HookManager) Process(hook *HookEvent) error {
	s.rl.Acquire()
	err := hook.Process()
	if err != nil {
		fmt.Println(err)
	}

	return err
}
