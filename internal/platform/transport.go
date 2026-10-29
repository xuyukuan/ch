package platform

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/MehmetMHY/ch/pkg/types"
	"github.com/sashabaranov/go-openai"
)

// Thinking and effort transports name where those controls are written in the
// chat-completions body. Gateways disagree on the field and its shape, and an
// unsupported field is silently ignored by most of them, so the transport is
// declared per platform instead of guessed at runtime.
const (
	ThinkingTransportChatTemplate = "chat_template_kwargs"
	ThinkingTransportDashScope    = "dashscope"
	ThinkingTransportAnthropic    = "anthropic"
	ThinkingTransportOllama       = "ollama"
	ThinkingTransportNone         = "none"

	EffortTransportRoot       = "root"
	EffortTransportOpenRouter = "openrouter"
	EffortTransportNone       = "none"

	DefaultThinkingBudget = 4096
)

// resolveThinkingTransport validates the platform's thinking transport and
// falls back to the chat_template_kwargs default.
func resolveThinkingTransport(p types.Platform) (string, error) {
	switch p.ThinkingTransport {
	case "":
		return ThinkingTransportChatTemplate, nil
	case ThinkingTransportChatTemplate, ThinkingTransportDashScope, ThinkingTransportAnthropic, ThinkingTransportOllama, ThinkingTransportNone:
		return p.ThinkingTransport, nil
	}
	return "", fmt.Errorf("unknown thinking_transport %q for platform %s (supported: %s, %s, %s, %s, %s)",
		p.ThinkingTransport, p.Name,
		ThinkingTransportChatTemplate, ThinkingTransportDashScope, ThinkingTransportAnthropic, ThinkingTransportOllama, ThinkingTransportNone)
}

// resolveEffortTransport validates the platform's reasoning-effort transport.
func resolveEffortTransport(p types.Platform) (string, error) {
	switch p.ReasoningEffortTransport {
	case "":
		return EffortTransportRoot, nil
	case EffortTransportRoot, EffortTransportOpenRouter, EffortTransportNone:
		return p.ReasoningEffortTransport, nil
	}
	return "", fmt.Errorf("unknown reasoning_effort_transport %q for platform %s (supported: %s, %s, %s)",
		p.ReasoningEffortTransport, p.Name, EffortTransportRoot, EffortTransportOpenRouter, EffortTransportNone)
}

// thinkingParams renders the body fragment carrying the thinking state. An
// empty state or the none transport yields nil so nothing is sent.
func thinkingParams(transport, state string, budget int) map[string]any {
	if state == "" || transport == ThinkingTransportNone {
		return nil
	}
	enabled := state == ThinkingOn

	switch transport {
	case ThinkingTransportChatTemplate:
		return map[string]any{"chat_template_kwargs": map[string]any{"enable_thinking": enabled}}
	case ThinkingTransportDashScope:
		return map[string]any{"enable_thinking": enabled}
	case ThinkingTransportOllama:
		return map[string]any{"think": enabled}
	case ThinkingTransportAnthropic:
		if !enabled {
			return map[string]any{"thinking": map[string]any{"type": "disabled"}}
		}
		if budget <= 0 {
			budget = DefaultThinkingBudget
		}
		return map[string]any{"thinking": map[string]any{"type": "enabled", "budget_tokens": budget}}
	}
	return nil
}

// effortParams renders the body fragment carrying the reasoning effort.
func effortParams(transport, effort string) map[string]any {
	if effort == "" || transport == EffortTransportNone {
		return nil
	}
	switch transport {
	case EffortTransportRoot:
		return map[string]any{"reasoning_effort": effort}
	case EffortTransportOpenRouter:
		return map[string]any{"reasoning": map[string]any{"effort": effort}}
	}
	return nil
}

type requestParamsKey struct{}

// withRequestParams attaches per-request body parameters. Carrying them on the
// context keeps concurrent requests isolated, which matters for the parallel
// compression workers that must never inherit chat-level controls.
func withRequestParams(ctx context.Context, params map[string]any) context.Context {
	if len(params) == 0 {
		return ctx
	}
	return context.WithValue(ctx, requestParamsKey{}, params)
}

// paramsTransport merges per-request parameters into the JSON request body.
// go-openai cannot express arbitrary provider fields, so they are injected
// here instead of widening the request struct.
type paramsTransport struct {
	base http.RoundTripper
}

func (t *paramsTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	params, _ := req.Context().Value(requestParamsKey{}).(map[string]any)
	if len(params) == 0 || req.Body == nil {
		return t.base.RoundTrip(req)
	}

	raw, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, fmt.Errorf("read request body: %w", err)
	}
	if err := req.Body.Close(); err != nil {
		return nil, fmt.Errorf("close request body: %w", err)
	}

	merged, err := mergeRequestParams(raw, params)
	if err != nil {
		return nil, err
	}

	req.Body = io.NopCloser(bytes.NewReader(merged))
	req.ContentLength = int64(len(merged))
	req.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(merged)), nil
	}
	return t.base.RoundTrip(req)
}

// mergeRequestParams deep-merges params into a JSON object body. Nested objects
// merge key by key; every other value replaces.
func mergeRequestParams(raw []byte, params map[string]any) ([]byte, error) {
	var body map[string]any
	if err := json.Unmarshal(raw, &body); err != nil {
		return nil, fmt.Errorf("merge request params: %w", err)
	}
	mergeInto(body, params)
	return json.Marshal(body)
}

func mergeInto(dst, src map[string]any) {
	for key, value := range src {
		if srcMap, ok := value.(map[string]any); ok {
			if dstMap, ok := dst[key].(map[string]any); ok {
				mergeInto(dstMap, srcMap)
				continue
			}
		}
		dst[key] = value
	}
}

// withParamsTransport wraps the client transport so declared parameters reach
// the request body of every chat-completions call.
func withParamsTransport(cfg openai.ClientConfig) openai.ClientConfig {
	client, ok := cfg.HTTPClient.(*http.Client)
	if !ok || client == nil {
		client = &http.Client{}
	}

	base := client.Transport
	if base == nil {
		base = http.DefaultTransport
	}

	cloned := *client
	cloned.Transport = &paramsTransport{base: base}
	cfg.HTTPClient = &cloned
	return cfg
}
