package platform

import (
	"encoding/json"
	"testing"

	"github.com/MehmetMHY/ch/pkg/types"
)

func TestNormalizeThinking(t *testing.T) {
	cases := []struct {
		in   string
		want string
		ok   bool
	}{
		{"", "", true},
		{"default", "", true},
		{"provider", "", true},
		{" on ", ThinkingOn, true},
		{"TRUE", ThinkingOn, true},
		{"enabled", ThinkingOn, true},
		{"off", ThinkingOff, true},
		{"disabled", ThinkingOff, true},
		{"maybe", "", false},
	}
	for _, c := range cases {
		got, ok := NormalizeThinking(c.in)
		if got != c.want || ok != c.ok {
			t.Errorf("NormalizeThinking(%q) = (%q, %v), want (%q, %v)", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestGetSetThinking(t *testing.T) {
	m := NewManager(&types.Config{})
	if m.GetThinking() != "" {
		t.Errorf("expected empty default, got %q", m.GetThinking())
	}
	m.SetThinking(ThinkingOff)
	if m.GetThinking() != ThinkingOff {
		t.Errorf("expected thinking=off, got %q", m.GetThinking())
	}
}

func thinkingPayload(t *testing.T, body string) map[string]interface{} {
	t.Helper()
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		t.Fatalf("failed to parse request body: %v", err)
	}
	return payload
}

func TestNonStreamingRequestIncludesThinkingOff(t *testing.T) {
	t.Setenv("TESTPLAT_API_KEY", "test-key")
	server, captured := captureRequestPayload(t, false)
	defer server.Close()

	cfg := &types.Config{
		CurrentPlatform:   "testplat",
		SlowModelPatterns: []string{".*"},
		Platforms: map[string]types.Platform{
			"testplat": {Name: "testplat", BaseURL: types.BaseURLValue{Single: server.URL + "/v1"}, EnvName: "TESTPLAT_API_KEY"},
		},
	}

	m := NewManager(cfg)
	m.SetThinking(ThinkingOff)
	if err := m.Initialize(); err != nil {
		t.Fatalf("Initialize error: %v", err)
	}

	cancelFn := func() {}
	messages := []types.ChatMessage{{Role: "user", Content: "test"}}
	if _, err := m.SendChatRequest(messages, "test-model", &cancelFn, new(bool)); err != nil {
		t.Fatalf("SendChatRequest error: %v", err)
	}

	payload := thinkingPayload(t, *captured)
	kwargs, ok := payload["chat_template_kwargs"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected chat_template_kwargs in request body, got %v", payload)
	}
	if kwargs["enable_thinking"] != false {
		t.Errorf("expected enable_thinking=false, got %v", kwargs["enable_thinking"])
	}
}

func TestStreamingRequestIncludesThinkingOn(t *testing.T) {
	t.Setenv("TESTPLAT_API_KEY", "test-key")
	server, captured := captureRequestPayload(t, true)
	defer server.Close()

	cfg := &types.Config{
		CurrentPlatform: "testplat",
		Platforms: map[string]types.Platform{
			"testplat": {Name: "testplat", BaseURL: types.BaseURLValue{Single: server.URL + "/v1"}, EnvName: "TESTPLAT_API_KEY"},
		},
	}

	m := NewManager(cfg)
	m.SetThinking(ThinkingOn)
	if err := m.Initialize(); err != nil {
		t.Fatalf("Initialize error: %v", err)
	}

	cancelFn := func() {}
	messages := []types.ChatMessage{{Role: "user", Content: "test"}}
	if _, err := m.SendChatRequest(messages, "stream-model", &cancelFn, new(bool)); err != nil {
		t.Fatalf("SendChatRequest error: %v", err)
	}

	payload := thinkingPayload(t, *captured)
	kwargs, ok := payload["chat_template_kwargs"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected chat_template_kwargs in streaming request body, got %v", payload)
	}
	if kwargs["enable_thinking"] != true {
		t.Errorf("expected enable_thinking=true, got %v", kwargs["enable_thinking"])
	}
}

func TestRequestOmitsThinkingWhenUnset(t *testing.T) {
	t.Setenv("TESTPLAT_API_KEY", "test-key")
	server, captured := captureRequestPayload(t, false)
	defer server.Close()

	cfg := &types.Config{
		CurrentPlatform:   "testplat",
		SlowModelPatterns: []string{".*"},
		Platforms: map[string]types.Platform{
			"testplat": {Name: "testplat", BaseURL: types.BaseURLValue{Single: server.URL + "/v1"}, EnvName: "TESTPLAT_API_KEY"},
		},
	}

	m := NewManager(cfg)
	if err := m.Initialize(); err != nil {
		t.Fatalf("Initialize error: %v", err)
	}

	cancelFn := func() {}
	messages := []types.ChatMessage{{Role: "user", Content: "test"}}
	if _, err := m.SendChatRequest(messages, "test-model", &cancelFn, new(bool)); err != nil {
		t.Fatalf("SendChatRequest error: %v", err)
	}

	payload := thinkingPayload(t, *captured)
	if _, ok := payload["chat_template_kwargs"]; ok {
		t.Error("expected chat_template_kwargs to be omitted when thinking is unset")
	}
}

func TestSilentRequestOmitsThinking(t *testing.T) {
	t.Setenv("TESTPLAT_API_KEY", "test-key")
	server, captured := captureRequestPayload(t, false)
	defer server.Close()

	cfg := &types.Config{
		CurrentPlatform:   "testplat",
		SlowModelPatterns: []string{".*"},
		Platforms: map[string]types.Platform{
			"testplat": {Name: "testplat", BaseURL: types.BaseURLValue{Single: server.URL + "/v1"}, EnvName: "TESTPLAT_API_KEY"},
		},
	}

	m := NewManager(cfg)
	m.SetThinking(ThinkingOff)
	if err := m.Initialize(); err != nil {
		t.Fatalf("Initialize error: %v", err)
	}

	cancelFn := func() {}
	messages := []types.ChatMessage{{Role: "user", Content: "generate filenames"}}
	if _, err := m.SendSilentChatRequest(messages, "test-model", &cancelFn, new(bool)); err != nil {
		t.Fatalf("SendSilentChatRequest error: %v", err)
	}

	payload := thinkingPayload(t, *captured)
	if _, ok := payload["chat_template_kwargs"]; ok {
		t.Error("expected chat_template_kwargs to be omitted in silent (auxiliary) request")
	}
	if m.GetThinking() != ThinkingOff {
		t.Errorf("expected thinking to be restored to off, got %q", m.GetThinking())
	}
}
