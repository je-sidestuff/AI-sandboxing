package prpoller

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

// ParsePRURL parses a GitHub PR URL and returns owner, repo, and PR number.
// Supports formats like:
//   - https://github.com/owner/repo/pull/123
//   - github.com/owner/repo/pull/123
//   - owner/repo#123
func ParsePRURL(input string) (owner, repo string, number int, err error) {
	input = strings.TrimSpace(input)

	// Handle short format: owner/repo#123
	shortRe := regexp.MustCompile(`^([^/]+)/([^#]+)#(\d+)$`)
	if matches := shortRe.FindStringSubmatch(input); len(matches) == 4 {
		num, err := strconv.Atoi(matches[3])
		if err != nil {
			return "", "", 0, fmt.Errorf("invalid PR number: %s", matches[3])
		}
		return matches[1], matches[2], num, nil
	}

	// Handle URL format
	if !strings.HasPrefix(input, "http") {
		input = "https://" + input
	}

	parsed, err := url.Parse(input)
	if err != nil {
		return "", "", 0, fmt.Errorf("invalid URL: %w", err)
	}

	// Expected path: /owner/repo/pull/123
	parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
	if len(parts) < 4 || parts[2] != "pull" {
		return "", "", 0, fmt.Errorf("URL does not appear to be a GitHub PR: %s", input)
	}

	num, err := strconv.Atoi(parts[3])
	if err != nil {
		return "", "", 0, fmt.Errorf("invalid PR number: %s", parts[3])
	}

	return parts[0], parts[1], num, nil
}

// FormatPRURL creates a GitHub PR URL from components
func FormatPRURL(owner, repo string, number int) string {
	return fmt.Sprintf("https://github.com/%s/%s/pull/%d", owner, repo, number)
}

// FormatPRShort creates a short PR reference (owner/repo#123)
func FormatPRShort(owner, repo string, number int) string {
	return fmt.Sprintf("%s/%s#%d", owner, repo, number)
}
