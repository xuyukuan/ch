package platform

import (
	"net/http"

	"github.com/sashabaranov/go-openai"
)

// headerTransport attaches the platform's configured headers to every request
// the OpenAI client sends, so gateways that require custom routing or auth
// headers (such as x-opencode-session) can be used through config alone.
type headerTransport struct {
	base    http.RoundTripper
	headers map[string]string
}

func (t *headerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	outbound := req.Clone(req.Context())
	for name, value := range t.headers {
		outbound.Header.Set(name, value)
	}
	return t.base.RoundTrip(outbound)
}

// withPlatformHeaders returns cfg with the platform headers applied to its HTTP
// client. A nil or empty header map leaves cfg untouched.
func withPlatformHeaders(cfg openai.ClientConfig, headers map[string]string) openai.ClientConfig {
	if len(headers) == 0 {
		return cfg
	}

	client, ok := cfg.HTTPClient.(*http.Client)
	if !ok || client == nil {
		client = &http.Client{}
	}

	base := client.Transport
	if base == nil {
		base = http.DefaultTransport
	}

	cloned := *client
	cloned.Transport = &headerTransport{base: base, headers: headers}
	cfg.HTTPClient = &cloned
	return cfg
}

// setPlatformHeaders applies the platform headers to a model-listing request.
// Values given here take precedence over the headers set for the provider.
func setPlatformHeaders(req *http.Request, headers map[string]string) {
	for name, value := range headers {
		req.Header.Set(name, value)
	}
}
