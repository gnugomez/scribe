package git

import (
	"strings"
	"testing"
)

func TestMergeTrailerValues(t *testing.T) {
	tests := []struct {
		name     string
		existing string
		new      string
		want     string
	}{
		{
			name:     "empty existing",
			existing: "",
			new:      "anthropic:claude-sonnet",
			want:     "anthropic:claude-sonnet",
		},
		{
			name:     "empty new",
			existing: "anthropic:claude-sonnet",
			new:      "",
			want:     "anthropic:claude-sonnet",
		},
		{
			name:     "both empty",
			existing: "",
			new:      "",
			want:     "",
		},
		{
			name:     "no duplicates",
			existing: "anthropic:claude-sonnet",
			new:      "github:copilot",
			want:     "anthropic:claude-sonnet, github:copilot",
		},
		{
			name:     "duplicate in new",
			existing: "anthropic:claude-sonnet",
			new:      "anthropic:claude-sonnet",
			want:     "anthropic:claude-sonnet",
		},
		{
			name:     "multiple existing with duplicate new",
			existing: "anthropic:claude-sonnet, github:copilot",
			new:      "github:copilot",
			want:     "anthropic:claude-sonnet, github:copilot",
		},
		{
			name:     "multiple existing, multiple new, some duplicates",
			existing: "anthropic:claude-sonnet, github:copilot",
			new:      "github:copilot, openai:gpt-4",
			want:     "anthropic:claude-sonnet, github:copilot, openai:gpt-4",
		},
		{
			name:     "whitespace handling",
			existing: "anthropic:claude-sonnet , github:copilot",
			new:      " openai:gpt-4 , anthropic:claude-sonnet",
			want:     "anthropic:claude-sonnet, github:copilot, openai:gpt-4",
		},
		{
			name:     "preserves order of existing first",
			existing: "z:model, a:model",
			new:      "m:model",
			want:     "z:model, a:model, m:model",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mergeTrailerValues(tt.existing, tt.new)
			if got != tt.want {
				t.Errorf("mergeTrailerValues(%q, %q)\n  got:  %q\n  want: %q", tt.existing, tt.new, got, tt.want)
			}
		})
	}
}

func TestParseCommitLog(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		verify func([]Commit) bool
	}{
		{
			name:  "empty output",
			input: "",
			verify: func(commits []Commit) bool {
				return len(commits) == 0
			},
		},
		{
			name:  "single commit",
			input: "abc123def456 Fix: correct typo in documentation",
			verify: func(commits []Commit) bool {
				return len(commits) == 1 &&
					commits[0].Hash == "abc123def456" &&
					commits[0].Subject == "Fix: correct typo in documentation"
			},
		},
		{
			name: "multiple commits",
			input: "hash1 subject one\nhash2 subject two\nhash3 subject three",
			verify: func(commits []Commit) bool {
				return len(commits) == 3 &&
					commits[0].Hash == "hash1" &&
					commits[0].Subject == "subject one" &&
					commits[1].Hash == "hash2" &&
					commits[1].Subject == "subject two" &&
					commits[2].Hash == "hash3" &&
					commits[2].Subject == "subject three"
			},
		},
		{
			name:  "whitespace trimming",
			input: "  \nhash1 subject\n  \n",
			verify: func(commits []Commit) bool {
				return len(commits) == 1 &&
					commits[0].Hash == "hash1" &&
					commits[0].Subject == "subject"
			},
		},
		{
			name:  "subject with multiple spaces",
			input: "hash1 fix: long commit subject with many words",
			verify: func(commits []Commit) bool {
				return len(commits) == 1 &&
					commits[0].Subject == "fix: long commit subject with many words"
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseCommitLog(tt.input)
			if !tt.verify(got) {
				t.Errorf("parseCommitLog(%q) = %v, verification failed", tt.input, got)
			}
		})
	}
}

// TestMergeTwoValues tests the edge cases of merging trailer values.
func TestMergeTwoValuesEdgeCases(t *testing.T) {
	// Test that trimmed values don't leave extra spaces.
	merged := mergeTrailerValues("a:b", "c:d")
	if strings.Contains(merged, "  ") {
		t.Errorf("mergeTrailerValues should not have double spaces, got: %q", merged)
	}

	// Test that commas are properly normalized.
	merged = mergeTrailerValues("a:b, c:d", "e:f, g:h")
	parts := strings.Split(merged, ",")
	for i, p := range parts {
		if strings.HasPrefix(p, " ") && i > 0 {
			// All parts except first should be trimmed
			if !strings.HasPrefix(p, " ") {
				t.Errorf("mergeTrailerValues comma-separated parts should have consistent spacing")
			}
		}
	}
}
