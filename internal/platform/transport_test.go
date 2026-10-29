package platform

import (
	"encoding/json"
	"io"
	"net/http"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/MehmetMHY/ch/pkg/types"
)

func TestThinkingParamsByTransport(t *testing.T) {
	cases := []struct {
		name      string
		transport string
		state     string
		budget    int
		want      map[string]any
	}{
		{"unset state sends nothing", ThinkingTransportChatTemplate, "", 0, nil},
		{"none transport sends nothing", ThinkingTransportNone, ThinkingOff, 0, nil},
		{"chat_template_kwargs off", ThinkingTransportChatTemplate, ThinkingOff, 0,
			map[string]any{"chat_template_kwargs": map[string]any{"enable_thinking": false}}},
		{"chat_template_kwargs on", ThinkingTransportChatTemplate, ThinkingOn, 0,
			map[string]any{"chat_template_kwargs": map[string]any{"enable_thinking": true}}},
		{"dashscope off", ThinkingTransportDashScope, ThinkingOff, 0,
			map[string]any{"enable_thinking": false}},
		{"dashscope on", ThinkingTransportDashScope, ThinkingOn, 0,
			map[string]any{"enable_thinking": true}},
		{"ollama off", ThinkingTransportOllama, ThinkingOff, 0,
			map[string]any{"think": false}},
		{"anthropic off", ThinkingTransportAnthropic, ThinkingOff, 0,
			map[string]any{"thinking": map[string]any{"type": "disabled"}}},
		{"anthropic on uses default budget", ThinkingTransportAnthropic, ThinkingOn, 0,
			map[string]any{"thinking": map[string]any{"type": "enabled", "budget_tokens": DefaultThinkingBudget}}},
		{"anthropic on honors platform budget", ThinkingTransportAnthropic, ThinkingOn, 1234,
			map[string]any{"thinking": map[string]any{"type": "enabled", "budget_tokens": 1234}}},
	}

	for _, c := range cases {
		got := thinkingParams(c.transport, c.state, c.budget)
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

func TestEffortParamsByTransport(t *testing.T) {
	cases := []struct {
		name      string
		transport string
		effort    string
		want      map[string]any
	}{
		{"unset effort sends nothing", EffortTransportRoot, "", nil},
		{"none transport sends nothing", EffortTransportNone, "medium", nil},
		{"root", EffortTransportRoot, "medium", map[string]any{"reasoning_effort": "medium"}},
		{"openrouter", EffortTransportOpenRouter, "xhigh", map[string]any{"reasoning": map[string]any{"effort": "xhigh"}}},
	}

	for _, c := range cases {
		got := effortParams(c.transport, c.effort)
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

func TestResolveTransports(t *testing.T) {
	got, err := resolveThinkingTransport(types.Platform{Name: "p"})
	if err != nil || got != ThinkingTransportChatTemplate {
		t.Errorf("default thinking transport: got (%q, %v)", got, err)
	}
	got, err = resolveEffortTransport(types.Platform{Name: "p"})
	if err != nil || got != EffortTransportRoot {
		t.Errorf("default effort transport: got (%q, %v)", got, err)
	}
	if _, err := resolveThinkingTransport(types.Platform{Name: "p", ThinkingTransport: "bogus"}); err == nil {
		t.Error("expected an error for an unknown thinking transport")
	}
	if _, err := resolveEffortTransport(types.Platform{Name: "p", ReasoningEffortTransport: "bogus"}); err == nil {
		t.Error("expected an error for an unknown effort transport")
	}
}

func TestInitializeRejectsUnknownTransport(t *testing.T) {
	t.Setenv("TESTPLAT_API_KEY", "test-key")
	cfg := &types.Config{
		CurrentPlatform: "testplat",
		Platforms: map[string]types.Platform{
			"testplat": {
				Name:              "testplat",
				BaseURL:           types.BaseURLValue{Single: "https://example.invalid/v1"},
				EnvName:           "TESTPLAT_API_KEY",
				ThinkingTransport: "bogus",
			},
		},
	}

	err := NewManager(cfg).Initialize()
	if err == nil || !strings.Contains(err.Error(), "thinking_transport") {
		t.Fatalf("expected a thinking_transport error at Initialize, got %v", err)
	}
}

func TestThinkingTransportReachesRequestBody(t *testing.T) {
	cases := []struct {
		transport string
		assert    func(t *testing.T, payload map[string]any)
	}{
		{ThinkingTransportChatTemplate, func(t *testing.T, payload map[string]any) {
			kwargs, ok := payload["chat_template_kwargs"].(map[string]any)
			if !ok || kwargs["enable_thinking"] != false {
				t.Fatalf("expected nested chat_template_kwargs, got %v", payload)
			}
		}},
		{ThinkingTransportDashScope, func(t *testing.T, payload map[string]any) {
			if payload["enable_thinking"] != false {
				t.Fatalf("expected root enable_thinking, got %v", payload)
			}
		}},
		{ThinkingTransportOllama, func(t *testing.T, payload map[string]any) {
			if payload["think"] != false {
				t.Fatalf("expected root think, got %v", payload)
			}
		}},
	}

	for _, c := range cases {
		t.Run(c.transport, func(t *testing.T) {
			t.Setenv("TESTPLAT_API_KEY", "test-key")
			server, captured := captureRequestPayload(t, false)
			defer server.Close()

			cfg := &types.Config{
				CurrentPlatform:   "testplat",
				SlowModelPatterns: []string{".*"},
				Platforms: map[string]types.Platform{
					"testplat": {
						Name:              "testplat",
						BaseURL:           types.BaseURLValue{Single: server.URL + "/v1"},
						EnvName:           "TESTPLAT_API_KEY",
						ThinkingTransport: c.transport,
					},
				},
			}

			m := NewManager(cfg)
			m.SetThinking(ThinkingOff)
			if err := m.Initialize(); err != nil {
				t.Fatalf("Initialize error: %v", err)
			}

			cancelFn := func() {}
			messages := []types.ChatMessage{{Role: "user", Content: "hi"}}
			if _, err := m.SendChatRequest(messages, "test-model", &cancelFn, new(bool)); err != nil {
				t.Fatalf("SendChatRequest error: %v", err)
			}

			var payload map[string]any
			if err := json.Unmarshal([]byte(*captured), &payload); err != nil {
				t.Fatalf("failed to parse request body: %v", err)
			}
			c.assert(t, payload)
		})
	}
}

func TestEffortTransportOpenRouterReachesRequestBody(t *testing.T) {
	t.Setenv("TESTPLAT_API_KEY", "test-key")
	server, captured := captureRequestPayload(t, false)
	defer server.Close()

	cfg := &types.Config{
		CurrentPlatform:   "testplat",
		SlowModelPatterns: []string{".*"},
		Platforms: map[string]types.Platform{
			"testplat": {
				Name:                     "testplat",
				BaseURL:                  types.BaseURLValue{Single: server.URL + "/v1"},
				EnvName:                  "TESTPLAT_API_KEY",
				ReasoningEffortTransport: EffortTransportOpenRouter,
			},
		},
	}

	m := NewManager(cfg)
	m.SetReasoningEffort("xhigh")
	if err := m.Initialize(); err != nil {
		t.Fatalf("Initialize error: %v", err)
	}

	cancelFn := func() {}
	messages := []types.ChatMessage{{Role: "user", Content: "hi"}}
	if _, err := m.SendChatRequest(messages, "test-model", &cancelFn, new(bool)); err != nil {
		t.Fatalf("SendChatRequest error: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(*captured), &payload); err != nil {
		t.Fatalf("failed to parse request body: %v", err)
	}
	reasoning, ok := payload["reasoning"].(map[string]any)
	if !ok || reasoning["effort"] != "xhigh" {
		t.Fatalf("expected nested reasoning.effort, got %v", payload)
	}
	if _, ok := payload["reasoning_effort"]; ok {
		t.Fatalf("expected root reasoning_effort to be absent, got %v", payload)
	}
}

type recordingTransport struct {
	mu     sync.Mutex
	bodies []string
}

func (r *recordingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	var body string
	if req.Body != nil {
		raw, err := io.ReadAll(req.Body)
		if err != nil {
			return nil, err
		}
		body = string(raw)
	}
	r.mu.Lock()
	r.bodies = append(r.bodies, body)
	r.mu.Unlock()
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader("{}")),
		Request:    req,
	}, nil
}

func TestParamsTransportMergesNestedAndIsolatesConcurrentRequests(t *testing.T) {
	recorder := &recordingTransport{}
	transport := &paramsTransport{base: recorder}

	newRequest := func(params map[string]any) *http.Request {
		body := "{\"model\":\"m\",\"chat_template_kwargs\":{\"seed\":1}}"
		req, err := http.NewRequest(http.MethodPost, "http://example.invalid/v1/chat/completions", strings.NewReader(body))
		if err != nil {
			t.Fatalf("failed to build request: %v", err)
		}
		return req.WithContext(withRequestParams(req.Context(), params))
	}

	withParams := map[string]any{"chat_template_kwargs": map[string]any{"enable_thinking": false}}

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			if _, err := transport.RoundTrip(newRequest(withParams)); err != nil {
				t.Errorf("RoundTrip with params: %v", err)
			}
		}()
		go func() {
			defer wg.Done()
			if _, err := transport.RoundTrip(newRequest(nil)); err != nil {
				t.Errorf("RoundTrip without params: %v", err)
			}
		}()
	}
	wg.Wait()

	for _, body := range recorder.bodies {
		var payload map[string]any
		if err := json.Unmarshal([]byte(body), &payload); err != nil {
			t.Fatalf("failed to parse %q: %v", body, err)
		}
		kwargs, ok := payload["chat_template_kwargs"].(map[string]any)
		if !ok {
			t.Fatalf("expected chat_template_kwargs in %q", body)
		}
		if _, ok := kwargs["enable_thinking"]; ok {
			if kwargs["enable_thinking"] != false {
				t.Fatalf("unexpected enable_thinking value in %q", body)
			}
		}
		if kwargs["seed"] != float64(1) {
			t.Fatalf("nested merge dropped existing keys: %q", body)
		}
	}
}
