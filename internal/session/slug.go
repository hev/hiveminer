package session

import (
	"regexp"
	"strings"
	"time"
)

var nonAlphaNum = regexp.MustCompile(`[^a-z0-9]+`)

// GenerateSlug creates a session directory name from form title and timestamp
func GenerateSlug(formTitle string) string {
	// Lowercase and replace non-alphanumeric with dashes
	slug := strings.ToLower(formTitle)
	slug = nonAlphaNum.ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")

	// Add timestamp
	timestamp := time.Now().Format("20060102-150405")

	return slug + "-" + timestamp
}

// GenerateSlugFromQuery creates a session directory name from search query
func GenerateSlugFromQuery(query string) string {
	if query == "" {
		return "session-" + time.Now().Format("20060102-150405")
	}

	// Take first few words
	words := strings.Fields(query)
	if len(words) > 4 {
		words = words[:4]
	}
	slug := strings.Join(words, "-")

	// Lowercase and replace non-alphanumeric with dashes
	slug = strings.ToLower(slug)
	slug = nonAlphaNum.ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")

	// Add timestamp
	timestamp := time.Now().Format("20060102-150405")

	return slug + "-" + timestamp
}
