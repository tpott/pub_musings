package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	// Create a temporary config file
	content := `sites:
  - name: test-site
    path: /tmp/test
    path_prefix: test/
    branch: main
    repository: user/repo
    commands:
      - "echo hello"
`
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write temp config: %v", err)
	}

	config, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if len(config.Sites) != 1 {
		t.Errorf("Expected 1 site, got %d", len(config.Sites))
	}

	site := config.Sites[0]
	if site.Name != "test-site" {
		t.Errorf("Expected name 'test-site', got %q", site.Name)
	}
	if site.Branch != "main" {
		t.Errorf("Expected branch 'main', got %q", site.Branch)
	}
	if site.PathPrefix != "test/" {
		t.Errorf("Expected path_prefix 'test/', got %q", site.PathPrefix)
	}
}

func TestLoadConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantErr string
	}{
		{
			name: "missing name",
			content: `sites:
  - path: /tmp/test
    branch: main
    repository: user/repo
    commands: ["echo"]
`,
			wantErr: "name is required",
		},
		{
			name: "missing path",
			content: `sites:
  - name: test
    branch: main
    repository: user/repo
    commands: ["echo"]
`,
			wantErr: "path is required",
		},
		{
			name: "missing branch",
			content: `sites:
  - name: test
    path: /tmp/test
    repository: user/repo
    commands: ["echo"]
`,
			wantErr: "branch is required",
		},
		{
			name: "missing repository",
			content: `sites:
  - name: test
    path: /tmp/test
    branch: main
    commands: ["echo"]
`,
			wantErr: "repository is required",
		},
		{
			name: "missing commands and deploy_script",
			content: `sites:
  - name: test
    path: /tmp/test
    branch: main
    repository: user/repo
`,
			wantErr: "either deploy_script or commands is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			configPath := filepath.Join(tmpDir, "config.yaml")
			if err := os.WriteFile(configPath, []byte(tt.content), 0644); err != nil {
				t.Fatalf("Failed to write temp config: %v", err)
			}

			_, err := LoadConfig(configPath)
			if err == nil {
				t.Fatal("Expected error, got nil")
			}
			if !contains(err.Error(), tt.wantErr) {
				t.Errorf("Expected error containing %q, got %q", tt.wantErr, err.Error())
			}
		})
	}
}

func TestFindMatchingSites(t *testing.T) {
	config := &Config{
		Sites: []SiteConfig{
			{
				Name:       "frontend",
				Path:       "/app/frontend",
				PathPrefix: "frontend/",
				Branch:     "main",
				Repository: "user/repo",
				Commands:   []string{"npm build"},
			},
			{
				Name:       "backend",
				Path:       "/app/backend",
				PathPrefix: "backend/",
				Branch:     "main",
				Repository: "user/repo",
				Commands:   []string{"go build"},
			},
			{
				Name:       "dev-frontend",
				Path:       "/app/frontend",
				PathPrefix: "frontend/",
				Branch:     "develop",
				Repository: "user/repo",
				Commands:   []string{"npm build"},
			},
			{
				Name:       "other-repo",
				Path:       "/other",
				PathPrefix: "",
				Branch:     "main",
				Repository: "other/repo",
				Commands:   []string{"echo"},
			},
		},
	}

	tests := []struct {
		name         string
		branch       string
		repository   string
		changedFiles []string
		wantSites    []string
	}{
		{
			name:         "frontend change on main",
			branch:       "main",
			repository:   "user/repo",
			changedFiles: []string{"frontend/src/app.js"},
			wantSites:    []string{"frontend"},
		},
		{
			name:         "backend change on main",
			branch:       "main",
			repository:   "user/repo",
			changedFiles: []string{"backend/main.go"},
			wantSites:    []string{"backend"},
		},
		{
			name:         "both frontend and backend changes",
			branch:       "main",
			repository:   "user/repo",
			changedFiles: []string{"frontend/src/app.js", "backend/main.go"},
			wantSites:    []string{"frontend", "backend"},
		},
		{
			name:         "frontend change on develop branch",
			branch:       "develop",
			repository:   "user/repo",
			changedFiles: []string{"frontend/src/app.js"},
			wantSites:    []string{"dev-frontend"},
		},
		{
			name:         "no matching path prefix",
			branch:       "main",
			repository:   "user/repo",
			changedFiles: []string{"docs/README.md"},
			wantSites:    []string{},
		},
		{
			name:         "wrong branch",
			branch:       "feature",
			repository:   "user/repo",
			changedFiles: []string{"frontend/src/app.js"},
			wantSites:    []string{},
		},
		{
			name:         "wrong repository",
			branch:       "main",
			repository:   "wrong/repo",
			changedFiles: []string{"frontend/src/app.js"},
			wantSites:    []string{},
		},
		{
			name:         "site with no path_prefix matches any file",
			branch:       "main",
			repository:   "other/repo",
			changedFiles: []string{"anything/file.txt"},
			wantSites:    []string{"other-repo"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matches := config.FindMatchingSites(tt.branch, tt.repository, tt.changedFiles)
			gotNames := make([]string, len(matches))
			for i, m := range matches {
				gotNames[i] = m.Name
			}

			if len(gotNames) != len(tt.wantSites) {
				t.Errorf("Expected %d sites %v, got %d sites %v", len(tt.wantSites), tt.wantSites, len(gotNames), gotNames)
				return
			}

			for i, want := range tt.wantSites {
				if gotNames[i] != want {
					t.Errorf("Expected site[%d] = %q, got %q", i, want, gotNames[i])
				}
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
