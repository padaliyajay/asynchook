package main

import (
	"fmt"
	"net/http"
	"net/url"
	"time"
)

type HookEvent struct {
	Id             string
	Url            string
	Payload        string
	Secret         string
	Run_after_time time.Time
	Expire_time    time.Time
	Retry_count    int
}

func (h *HookEvent) Process() error {
	postData := url.Values{}
	postData.Set("payload", h.Payload)
	postData.Set("secret", h.Secret)

	resp, err := http.PostForm(h.Url, postData)

	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("failed to send hook event to %s: %s", h.Url, resp.Status)
	}

	return nil
}
