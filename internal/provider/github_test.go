package provider

import "testing"

func TestGitHubProviderName(t *testing.T) {
	g := NewGitHubProvider("github.com", "user", "repo")
	if g.Name() != "github" {
		t.Errorf("expected 'github', got %q", g.Name())
	}
}

func TestGitHubRepoFlag(t *testing.T) {
	g := NewGitHubProvider("github.com", "myorg", "myrepo")
	if g.repoFlag() != "myorg/myrepo" {
		t.Errorf("expected 'myorg/myrepo', got %q", g.repoFlag())
	}
}

func TestExtractPRNumber(t *testing.T) {
	tests := []struct {
		url  string
		want int
	}{
		{"https://github.com/user/repo/pull/42", 42},
		{"https://github.com/user/repo/pull/1", 1},
		{"https://github.com/user/repo/pull/999", 999},
		{"not-a-url", 0},
		{"", 0},
	}

	for _, tt := range tests {
		got := extractPRNumber(tt.url)
		if got != tt.want {
			t.Errorf("extractPRNumber(%q) = %d, want %d", tt.url, got, tt.want)
		}
	}
}

func TestMapReviewDecision(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"APPROVED", "approved"},
		{"CHANGES_REQUESTED", "changes_requested"},
		{"REVIEW_REQUIRED", "pending"},
		{"", ""},
		{"UNKNOWN", ""},
	}

	for _, tt := range tests {
		got := mapReviewDecision(tt.input)
		if got != tt.want {
			t.Errorf("mapReviewDecision(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestMapCIStatus(t *testing.T) {
	tests := []struct {
		name   string
		checks []ghCheckStatus
		want   string
	}{
		{"empty", nil, ""},
		{"all pass", []ghCheckStatus{{"SUCCESS"}, {"SUCCESS"}}, "pass"},
		{"one fail", []ghCheckStatus{{"SUCCESS"}, {"FAILURE"}}, "fail"},
		{"one error", []ghCheckStatus{{"ERROR"}}, "fail"},
		{"pending", []ghCheckStatus{{"SUCCESS"}, {"PENDING"}}, "pending"},
		{"in progress", []ghCheckStatus{{"IN_PROGRESS"}}, "pending"},
		{"queued", []ghCheckStatus{{"QUEUED"}}, "pending"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapCIStatus(tt.checks)
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGitHubCLIAvailable(t *testing.T) {
	// This test checks that CLIAvailable doesn't panic.
	// Whether gh is installed depends on the environment.
	g := NewGitHubProvider("github.com", "user", "repo")
	_ = g.CLIAvailable() // just ensure no panic
}
