package platform

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/MehmetMHY/ch/pkg/types"
	"github.com/sashabaranov/go-openai"
)

func TestPlatformHeadersReachChatRequests(t *testing.T) {
	var captured http.Header
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = r.Header.Clone()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"1","object":"chat.completion","created":1,"model":"m","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}]}`))
	}))
	defer server.Close()

	t.Setenv("HEADERS_TEST_KEY", "test-key")
	t.Setenv("HOME", t.TempDir())
	cfg := &types.Config{
		CurrentPlatform:   "headerplat",
		ModelsDevEnabled:  false,
		SlowModelPatterns: []string{".*"},
		Platforms: map[string]types.Platform{
			"headerplat": {
				Name:    "headerplat",
				BaseURL: types.BaseURLValue{Single: server.URL + "/v1"},
				EnvName: "HEADERS_TEST_KEY",
				Headers: map[string]string{"x-test-session": "abc-123"},
			},
		},
	}

	m := NewManager(cfg)
	if err := m.Initialize(); err != nil {
		t.Fatalf("Initialize error: %v", err)
	}

	cancelFn := func() {}
	if _, err := m.SendChatRequest([]types.ChatMessage{{Role: "user", Content: "hi"}}, "m", &cancelFn, new(bool)); err != nil {
		t.Fatalf("SendChatRequest error: %v", err)
	}

	if got := captured.Get("x-test-session"); got != "abc-123" {
		t.Fatalf("x-test-session header: got %q, want %q", got, "abc-123")
	}
}

func TestPlatformHeadersKeepAuthorization(t *testing.T) {
	var captured http.Header
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = r.Header.Clone()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"1","object":"chat.completion","created":1,"model":"m","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}]}`))
	}))
	defer server.Close()

	t.Setenv("HEADERS_TEST_KEY", "test-key")
	t.Setenv("HOME", t.TempDir())
	cfg := &types.Config{
		CurrentPlatform:   "headerplat",
		ModelsDevEnabled:  false,
		SlowModelPatterns: []string{".*"},
		Platforms: map[string]types.Platform{
			"headerplat": {
				Name:    "headerplat",
				BaseURL: types.BaseURLValue{Single: server.URL + "/v1"},
				EnvName: "HEADERS_TEST_KEY",
				Headers: map[string]string{"x-test-session": "abc-123"},
			},
		},
	}

	m := NewManager(cfg)
	if err := m.Initialize(); err != nil {
		t.Fatalf("Initialize error: %v", err)
	}

	cancelFn := func() {}
	if _, err := m.SendChatRequest([]types.ChatMessage{{Role: "user", Content: "hi"}}, "m", &cancelFn, new(bool)); err != nil {
		t.Fatalf("SendChatRequest error: %v", err)
	}

	if got := captured.Get("Authorization"); got != "Bearer test-key" {
		t.Fatalf("Authorization header: got %q, want %q", got, "Bearer test-key")
	}
}

func TestWithPlatformHeadersSkipsEmpty(t *testing.T) {
	cfg := openai.DefaultConfig("test-key")
	if got := withPlatformHeaders(cfg, nil); got.HTTPClient != cfg.HTTPClient {
		t.Fatal("expected unchanged client when no headers are configured")
	}
}

func TestSetPlatformHeaders(t *testing.T) {
	req, err := http.NewRequest("GET", "https://example.com/v1/models", nil)
	if err != nil {
		t.Fatalf("NewRequest error: %v", err)
	}
	req.Header.Set("Authorization", "Bearer test-key")

	setPlatformHeaders(req, map[string]string{"x-test-session": "abc-123"})

	if got := req.Header.Get("x-test-session"); got != "abc-123" {
		t.Fatalf("x-test-session header: got %q, want %q", got, "abc-123")
	}
	if got := req.Header.Get("Authorization"); got != "Bearer test-key" {
		t.Fatalf("Authorization header: got %q, want %q", got, "Bearer test-key")
	}
}
