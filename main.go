package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

// RateLimitRetryRoundTripper is an http.RoundTripper that retries requests when rate limited.
type RateLimitRetryRoundTripper struct {
	Transport  http.RoundTripper
	MaxRetries int
	SleepFunc  func(time.Duration)
}

// RoundTrip implements the http.RoundTripper interface.
func (rt *RateLimitRetryRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	transport := rt.Transport
	if transport == nil {
		transport = http.DefaultTransport
	}

	maxRetries := rt.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 3
	}

	sleepFunc := rt.SleepFunc

	var resp *http.Response
	var err error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		clonedReq := req.Clone(req.Context())

		resp, err = transport.RoundTrip(clonedReq)
		if err != nil {
			return nil, err
		}

		isRateLimit := resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests
		if isRateLimit {
			remainingHeader := resp.Header.Get("X-RateLimit-Remaining")
			resetHeader := resp.Header.Get("X-RateLimit-Reset")

			if remainingHeader == "0" || resp.StatusCode == http.StatusTooManyRequests {
				if resetHeader != "" {
					resetUnix, parseErr := strconv.ParseInt(resetHeader, 10, 64)
					if parseErr == nil {
						resetTime := time.Unix(resetUnix, 0)
						resetDuration := time.Until(resetTime)

						var sleepDuration time.Duration
						if resetDuration <= 0 {
							sleepDuration = 1 * time.Second
						} else {
							sleepDuration = resetDuration + 1 * time.Second
						}

						if attempt >= maxRetries {
							return resp, nil
						}

						if sleepFunc != nil {
							sleepFunc(sleepDuration)
						} else {
							select {
							case <-clonedReq.Context().Done():
								if resp.Body != nil {
									io.Copy(io.Discard, resp.Body)
									resp.Body.Close()
								}
								return nil, clonedReq.Context().Err()
							case <-time.After(sleepDuration):
							}
						}

						if resp.Body != nil {
							io.Copy(io.Discard, resp.Body)
							resp.Body.Close()
						}
						continue
					}
				}
			}
		}

		return resp, nil
	}

	return resp, err
}

func main() {
	fmt.Println("Hello, Bounty Hunter!")
}