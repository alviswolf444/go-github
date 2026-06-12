package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"
)

func TestRateLimitRetryRoundTripper_ClockDrift(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.Header().Set("X-RateLimit-Remaining", "0")
		resetTime := time.Now().Add(-5 * time.Second).Unix()
		w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(resetTime, 10))
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()

	var sleepDurations []time.Duration
	rt := &RateLimitRetryRoundTripper{
		Transport:  http.DefaultTransport,
		MaxRetries: 3,
		SleepFunc: func(d time.Duration) {
			sleepDurations = append(sleepDurations, d)
		},
	}

	client := &http.Client{Transport: rt}
	req, err := http.NewRequestWithContext(context.Background(), "GET", server.URL, nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	if attempts != 4 {
		t.Errorf("Expected 4 attempts, got %d", attempts)
	}

	if len(sleepDurations) != 3 {
		t.Errorf("Expected 3 sleep durations, got %d", len(sleepDurations))
	}

	for i, d := range sleepDurations {
		if d != 1*time.Second {
			t.Errorf("Sleep duration %d: expected 1s (fallback), got %v", i, d)
		}
	}

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("Expected final status code 403, got %d", resp.StatusCode)
	}
}

func TestRateLimitRetryRoundTripper_StandardBackoff(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts == 1 {
			w.Header().Set("X-RateLimit-Remaining", "0")
			resetTime := time.Now().Add(2 * time.Second).Unix()
			w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(resetTime, 10))
			w.WriteHeader(http.StatusForbidden)
		} else {
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer server.Close()

	var sleepDurations []time.Duration
	rt := &RateLimitRetryRoundTripper{
		Transport:  http.DefaultTransport,
		MaxRetries: 3,
		SleepFunc: func(d time.Duration) {
			sleepDurations = append(sleepDurations, d)
		},
	}

	client := &http.Client{Transport: rt}
	req, err := http.NewRequestWithContext(context.Background(), "GET", server.URL, nil)
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Request failed: %v", err)
	}
	defer resp.Body.Close()

	if attempts != 2 {
		t.Errorf("Expected 2 attempts, got %d", attempts)
	}

	if len(sleepDurations) != 1 {
		t.Errorf("Expected 1 sleep duration, got %d", len(sleepDurations))
	}

	d := sleepDurations[0]
	if d < 2*time.Second || d > 4*time.Second {
		t.Errorf("Expected sleep duration around 3s, got %v", d)
	}

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected final status code 200, got %d", resp.StatusCode)
	}
}