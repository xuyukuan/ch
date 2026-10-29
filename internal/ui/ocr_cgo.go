//go:build cgo

package ui

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/otiai10/gosseract/v2"
)

// extractTextFromImage uses Tesseract OCR to extract text from an image
func (t *Terminal) extractTextFromImage(filePath string) (string, error) {
	return t.extractImageText(func(client *gosseract.Client) error { return client.SetImage(filePath) })
}

func (t *Terminal) extractTextFromImageBytes(data []byte) (string, error) {
	return t.extractImageText(func(client *gosseract.Client) error { return client.SetImageFromBytes(data) })
}

func (t *Terminal) extractImageText(setImage func(*gosseract.Client) error) (string, error) {
	// Check if tesseract is installed
	if _, err := exec.LookPath("tesseract"); err != nil {
		return "", fmt.Errorf("tesseract OCR is not installed. Please install it to enable image-to-text extraction")
	}

	client := gosseract.NewClient()
	defer client.Close()

	language := normalizedOCRLanguage(t.config.OCRLanguage)
	if err := client.SetLanguage(language); err != nil {
		return "", fmt.Errorf("failed to load OCR language %q (see tesseract --list-langs): %w", language, err)
	}

	if err := setImage(client); err != nil {
		return "", fmt.Errorf("failed to set image source: %w", err)
	}

	// The whitelist sharpens latin OCR but drops every character outside it,
	// so non-latin languages must run without it.
	if latinOnlyOCR(language) {
		if err := client.SetVariable("tessedit_char_whitelist", "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789.,!?@#$%^&*()_+-={}[]|\\:;\"'<>/~` "); err != nil {
			return "", fmt.Errorf("failed to configure OCR: %w", err)
		}
	}

	// Extract text
	text, err := client.Text()
	if err != nil {
		return "", fmt.Errorf("OCR extraction failed: %w", err)
	}

	// Clean up the extracted text
	cleanedText := strings.TrimSpace(text)

	// Remove excessive whitespace and normalize line breaks
	lines := strings.Split(cleanedText, "\n")
	var cleanedLines []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			cleanedLines = append(cleanedLines, line)
		}
	}

	return strings.Join(cleanedLines, "\n"), nil
}
