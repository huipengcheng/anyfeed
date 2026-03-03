package email

import (
	"regexp"
	"testing"
)

func TestParseMatch(t *testing.T) {
	tests := []struct {
		name    string
		expr    string
		wantErr bool
		rules   int
	}{
		{"simple from", "from:user@example.com", false, 1},
		{"wildcard from", "from:*@example.com", false, 1},
		{"simple to", "to:rss@domain.com", false, 1},
		{"subject contains", "subject:*Newsletter*", false, 1},
		{"multiple rules", "from:*@example.com,subject:*weekly*", false, 2},
		{"with spaces", "from: user@example.com , subject: test", false, 2},
		{"empty", "", true, 0},
		{"no colon", "from-user@example.com", true, 0},
		{"empty pattern", "from:", true, 0},
		{"unknown field", "body:test", true, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rules, err := ParseMatch(tt.expr)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseMatch() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && len(rules) != tt.rules {
				t.Errorf("expected %d rules, got %d", tt.rules, len(rules))
			}
		})
	}
}

func TestMatchEmail(t *testing.T) {
	tests := []struct {
		name    string
		expr    string
		from    string
		to      string
		subject string
		want    bool
	}{
		{
			name:    "exact from match",
			expr:    "from:user@example.com",
			from:    "user@example.com",
			to:      "other@domain.com",
			subject: "Hello",
			want:    true,
		},
		{
			name:    "exact from no match",
			expr:    "from:user@example.com",
			from:    "other@example.com",
			to:      "other@domain.com",
			subject: "Hello",
			want:    false,
		},
		{
			name:    "wildcard from match",
			expr:    "from:*@example.com",
			from:    "anyone@example.com",
			to:      "other@domain.com",
			subject: "Hello",
			want:    true,
		},
		{
			name:    "wildcard from no match",
			expr:    "from:*@example.com",
			from:    "user@other.com",
			to:      "other@domain.com",
			subject: "Hello",
			want:    false,
		},
		{
			name:    "subject contains match",
			expr:    "subject:*Newsletter*",
			from:    "news@example.com",
			to:      "user@domain.com",
			subject: "Weekly Newsletter Edition #5",
			want:    true,
		},
		{
			name:    "subject contains no match",
			expr:    "subject:*Newsletter*",
			from:    "news@example.com",
			to:      "user@domain.com",
			subject: "Weekly Update",
			want:    false,
		},
		{
			name:    "multiple rules all match",
			expr:    "from:*@newsletter.com,subject:*weekly*",
			from:    "news@newsletter.com",
			to:      "user@domain.com",
			subject: "Your Weekly Digest",
			want:    true,
		},
		{
			name:    "multiple rules one fails",
			expr:    "from:*@newsletter.com,subject:*weekly*",
			from:    "news@other.com",
			to:      "user@domain.com",
			subject: "Your Weekly Digest",
			want:    false,
		},
		{
			name:    "case insensitive",
			expr:    "from:USER@EXAMPLE.COM",
			from:    "user@example.com",
			to:      "other@domain.com",
			subject: "Hello",
			want:    true,
		},
		{
			name:    "to field match",
			expr:    "to:rss@mydomain.com",
			from:    "sender@example.com",
			to:      "rss@mydomain.com",
			subject: "Test",
			want:    true,
		},
		{
			name:    "complex pattern",
			expr:    "from:no-reply@*.example.com",
			from:    "no-reply@mail.example.com",
			to:      "user@domain.com",
			subject: "Notification",
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rules, err := ParseMatch(tt.expr)
			if err != nil {
				t.Fatalf("failed to parse match expression: %v", err)
			}

			got := MatchEmail(rules, tt.from, tt.to, tt.subject)
			if got != tt.want {
				t.Errorf("MatchEmail() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGlobToRegex(t *testing.T) {
	tests := []struct {
		glob    string
		input   string
		matches bool
	}{
		{"*", "anything", true},
		{"*@example.com", "user@example.com", true},
		{"*@example.com", "user@other.com", false},
		{"user@*", "user@example.com", true},
		{"user@*", "other@example.com", false},
		{"*.example.com", "mail.example.com", true},
		{"test.txt", "test.txt", true},
		{"test.txt", "test-txt", false},
		{"test?.txt", "test1.txt", true},
		{"test?.txt", "test12.txt", false},
	}

	for _, tt := range tests {
		t.Run(tt.glob+"_"+tt.input, func(t *testing.T) {
			pattern := globToRegex(tt.glob)
			re, err := regexp.Compile("(?i)" + pattern)
			if err != nil {
				t.Fatalf("failed to compile pattern: %v", err)
			}

			if re.MatchString(tt.input) != tt.matches {
				t.Errorf("pattern %s (regex: %s) match %s = %v, want %v",
					tt.glob, pattern, tt.input, !tt.matches, tt.matches)
			}
		})
	}
}
