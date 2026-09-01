package main

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

// hookClient bounds how long a single delivery may take. The default http
// client has no timeout at all, so a slow endpoint would pile up goroutines
// and sockets without bound.
var hookClient = &http.Client{Timeout: time.Minute}

type HookEvent struct {
	Id             string
	Url            string
	Payload        string
	Secret         string
	Priority       float64
	Run_after_time time.Time
	Expire_time    time.Time
	Retry_count    int
}

func (h *HookEvent) Process() error {
	postData := url.Values{}
	postData.Set("payload", h.Payload)
	postData.Set("secret", h.Secret)

	resp, err := hookClient.PostForm(h.Url, postData)

	if err != nil {
		return err
	}

	defer resp.Body.Close()

	// Drain the body so the connection can be reused for the next hook.
	io.Copy(io.Discard, resp.Body)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("failed to send hook event to %s: %s", h.Url, resp.Status)
	}

	return nil
}
