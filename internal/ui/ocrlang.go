package ui

import (
	"strings"

	"github.com/MehmetMHY/ch/pkg/types"
)

// normalizedOCRLanguage resolves the configured OCR language, falling back to
// the default when unset.
func normalizedOCRLanguage(language string) string {
	trimmed := strings.TrimSpace(language)
	if trimmed == "" {
		return types.DefaultOCRLanguage
	}
	return trimmed
}

// latinOnlyOCR reports whether the Tesseract character whitelist applies. The
// whitelist only lists latin letters, digits, and punctuation, so enabling it
// for any other script makes the engine drop every recognized character.
func latinOnlyOCR(language string) bool {
	for _, lang := range strings.Split(normalizedOCRLanguage(language), "+") {
		if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(lang)), "eng") {
			return false
		}
	}
	return true
}
