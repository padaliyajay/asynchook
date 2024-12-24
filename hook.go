package main

import (
	"fmt"
	"net/http"
	"net/url"
)

type HookEvent struct {
	Id        string
	Url       string
	Payload   string
	Timestamp string
	Secret    string
}

func (h *HookEvent) Process() error {
	postData := url.Values{}
	postData.Set("payload", h.Payload)
	postData.Set("timestamp", h.Timestamp)
	postData.Set("secret", h.Secret)

	resp, err := http.PostForm(h.Url, postData)

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
