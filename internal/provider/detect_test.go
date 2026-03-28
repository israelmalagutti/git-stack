package provider

import "testing"

func TestParseRemoteURL(t *testing.T) {
	tests := []struct {
		name      string
		url       string
		wantHost  string
		wantOwner string
		wantRepo  string
		wantErr   bool
	}{
		{"HTTPS", "https://github.com/user/repo.git", "github.com", "user", "repo", false},
		{"HTTPS no .git", "https://github.com/user/repo", "github.com", "user", "repo", false},
		{"SSH shorthand", "git@github.com:user/repo.git", "github.com", "user", "repo", false},
		{"SSH shorthand no .git", "git@github.com:user/repo", "github.com", "user", "repo", false},
		{"SSH scheme", "ssh://git@github.com/user/repo.git", "github.com", "user", "repo", false},
		{"GitLab HTTPS", "https://gitlab.com/org/project.git", "gitlab.com", "org", "project", false},
		{"GHE HTTPS", "https://github.mycompany.com/team/service.git", "github.mycompany.com", "team", "service", false},
		{"GHE SSH", "git@github.corp.example.com:team/service.git", "github.corp.example.com", "team", "service", false},
		{"Bitbucket", "https://bitbucket.org/user/repo.git", "bitbucket.org", "user", "repo", false},
		{"empty", "", "", "", "", true},
		{"no path", "https://github.com", "", "", "", true},
		{"single segment", "https://github.com/user", "", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host, owner, repo, err := ParseRemoteURL(tt.url)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error for %q", tt.url)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if host != tt.wantHost {
				t.Errorf("host: got %q, want %q", host, tt.wantHost)
			}
			if owner != tt.wantOwner {
				t.Errorf("owner: got %q, want %q", owner, tt.wantOwner)
			}
			if repo != tt.wantRepo {
				t.Errorf("repo: got %q, want %q", repo, tt.wantRepo)
			}
		})
	}
}

func TestDetectFromRemoteURL(t *testing.T) {
	tests := []struct {
		name         string
		url          string
		wantProvider string
		wantErr      bool
	}{
		{"GitHub HTTPS", "https://github.com/user/repo", "github", false},
		{"GitHub SSH", "git@github.com:user/repo.git", "github", false},
		{"GHE", "https://github.mycompany.com/team/svc.git", "github", false},
		{"GitLab", "https://gitlab.com/org/project.git", "generic", false},
		{"Bitbucket", "https://bitbucket.org/user/repo.git", "generic", false},
		{"Self-hosted", "https://git.internal.com/team/repo.git", "generic", false},
		{"empty URL", "", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := DetectFromRemoteURL(tt.url)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if p.Name() != tt.wantProvider {
				t.Errorf("provider: got %q, want %q", p.Name(), tt.wantProvider)
			}
		})
	}
}
