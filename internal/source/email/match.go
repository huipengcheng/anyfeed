package email

import (
	"fmt"
	"regexp"
	"strings"
)

// MatchRule represents an email matching rule.
type MatchRule struct {
	Field   string         // from, to, subject
	Pattern *regexp.Regexp // Compiled pattern
}

// ParseMatch parses match expression like "from:*@example.com".
// Supported formats:
//   - from:user@example.com
//   - from:*@example.com (wildcard matching)
//   - to:rss@yourdomain.com
//   - subject:*Newsletter* (contains matching)
//   - Multiple rules separated by comma: from:*@example.com,subject:*weekly*
func ParseMatch(expr string) ([]MatchRule, error) {
	if expr == "" {
		return nil, fmt.Errorf("match expression is empty")
	}

	var rules []MatchRule
	parts := strings.Split(expr, ",")

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		// Split by first colon
		idx := strings.Index(part, ":")
		if idx == -1 {
			return nil, fmt.Errorf("invalid match expression: %s (expected field:pattern)", part)
		}

		field := strings.ToLower(strings.TrimSpace(part[:idx]))
		pattern := strings.TrimSpace(part[idx+1:])

		if pattern == "" {
			return nil, fmt.Errorf("empty pattern for field '%s'", field)
		}

		// Validate field
		switch field {
		case "from", "to", "subject":
			// Valid fields
		default:
			return nil, fmt.Errorf("unsupported field '%s' (use from, to, or subject)", field)
		}

		// Convert glob pattern to regex
		regexPattern := globToRegex(pattern)
		re, err := regexp.Compile("(?i)" + regexPattern) // Case insensitive
		if err != nil {
			return nil, fmt.Errorf("invalid pattern '%s': %w", pattern, err)
		}

		rules = append(rules, MatchRule{
			Field:   field,
			Pattern: re,
		})
	}

	if len(rules) == 0 {
		return nil, fmt.Errorf("no valid match rules found")
	}

	return rules, nil
}

// globToRegex converts a simple glob pattern to a regex pattern.
// Supports * as wildcard for any characters.
func globToRegex(glob string) string {
	var result strings.Builder
	result.WriteString("^")

	for i := 0; i < len(glob); i++ {
		c := glob[i]
		switch c {
		case '*':
			result.WriteString(".*")
		case '?':
			result.WriteString(".")
		case '.', '+', '^', '$', '[', ']', '(', ')', '{', '}', '|', '\\':
			result.WriteByte('\\')
			result.WriteByte(c)
		default:
			result.WriteByte(c)
		}
	}

	result.WriteString("$")
	return result.String()
}

// MatchEmail checks if an email matches the given rules.
// All rules must match (AND logic).
func MatchEmail(rules []MatchRule, from, to, subject string) bool {
	for _, rule := range rules {
		var value string
		switch rule.Field {
		case "from":
			value = from
		case "to":
			value = to
		case "subject":
			value = subject
		default:
			return false
		}

		if !rule.Pattern.MatchString(value) {
			return false
		}
	}
	return true
}
