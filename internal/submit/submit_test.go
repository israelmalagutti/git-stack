package submit

import "testing"

func TestHumanizeBranchName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"feat/auth-ui", "Auth ui"},
		{"fix/login-bug", "Login bug"},
		{"auth", "Auth"},
		{"my-feature", "My feature"},
		{"feat/a", "A"},
		{"release/v1.0/hotfix", "Hotfix"},
	}

	for _, tt := range tests {
		got := HumanizeBranchName(tt.input)
		if got != tt.want {
			t.Errorf("HumanizeBranchName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}
