package api

import (
	"os"
	"testing"
	"time"

	"nofx/config"
)

func TestIsLoopbackClient(t *testing.T) {
	if !isLoopbackClient("127.0.0.1") {
		t.Fatal("expected 127.0.0.1 to be treated as loopback")
	}
	if !isLoopbackClient("::1") {
		t.Fatal("expected ::1 to be treated as loopback")
	}
	if isLoopbackClient("8.8.8.8") {
		t.Fatal("did not expect public IP to be treated as loopback")
	}
}

func TestIsAllowedOrigin(t *testing.T) {
	original := os.Getenv("CORS_ALLOWED_ORIGINS")
	defer os.Setenv("CORS_ALLOWED_ORIGINS", original)

	if err := os.Setenv("CORS_ALLOWED_ORIGINS", "https://app.example.com,http://127.0.0.1:3000"); err != nil {
		t.Fatalf("failed to set env: %v", err)
	}
	config.Init()

	if !isAllowedOrigin("https://app.example.com") {
		t.Fatal("expected configured origin to be allowed")
	}
	if isAllowedOrigin("https://evil.example.com") {
		t.Fatal("did not expect unconfigured origin to be allowed")
	}
}

func TestLoginRateLimiter(t *testing.T) {
	limiter := newLoginRateLimiter()
	ip := "203.0.113.10"

	for i := 0; i < 5; i++ {
		if !limiter.allow(ip) {
			t.Fatalf("expected attempt %d to be allowed before lockout", i+1)
		}
		limiter.registerFailure(ip)
	}

	if limiter.allow(ip) {
		t.Fatal("expected IP to be rate-limited after repeated failures")
	}

	limiter.mu.Lock()
	entry := limiter.entries[ip]
	entry.BlockedUntil = time.Now().Add(-time.Minute)
	entry.WindowStart = time.Now().Add(-16 * time.Minute)
	limiter.entries[ip] = entry
	limiter.mu.Unlock()

	if !limiter.allow(ip) {
		t.Fatal("expected limiter to recover after cooldown")
	}

	limiter.registerSuccess(ip)
	if !limiter.allow(ip) {
		t.Fatal("expected successful login to clear limiter state")
	}
}
