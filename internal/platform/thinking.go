package platform

import "strings"

// Thinking states sent as chat_template_kwargs.enable_thinking. The empty
// state omits chat_template_kwargs so the provider default applies.
const (
	ThinkingOn  = "on"
	ThinkingOff = "off"
)

// ThinkingOptions lists the picker values for the interactive thinking switch.
func ThinkingOptions() []string {
	return []string{"default", ThinkingOn, ThinkingOff}
}

// NormalizeThinking maps user input to a thinking state. The bool reports
// whether the input was recognized.
func NormalizeThinking(arg string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(arg)) {
	case "", "default", "provider":
		return "", true
	case ThinkingOn, "true", "enable", "enabled":
		return ThinkingOn, true
	case ThinkingOff, "false", "disable", "disabled":
		return ThinkingOff, true
	}
	return "", false
}

// DescribeThinkingState renders the status text for a thinking state.
func DescribeThinkingState(thinking string) string {
	switch thinking {
	case ThinkingOn, ThinkingOff:
		return thinking
	}
	return "provider default"
}
