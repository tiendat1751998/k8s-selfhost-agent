package github

import "testing"

func TestParseRepo_FullURL(t *testing.T) {
	owner, repo := parseRepo("https://github.com/datdt/k8sselfhost")
	if owner != "datdt" {
		t.Errorf("expected owner 'datdt', got '%s'", owner)
	}
	if repo != "k8sselfhost" {
		t.Errorf("expected repo 'k8sselfhost', got '%s'", repo)
	}
}

func TestParseRepo_WithGitSuffix(t *testing.T) {
	owner, repo := parseRepo("https://github.com/datdt/k8sselfhost.git")
	if owner != "datdt" || repo != "k8sselfhost" {
		t.Errorf("unexpected: %s/%s", owner, repo)
	}
}

func TestParseRepo_ShortForm(t *testing.T) {
	owner, repo := parseRepo("datdt/k8sselfhost")
	if owner != "datdt" || repo != "k8sselfhost" {
		t.Errorf("unexpected: %s/%s", owner, repo)
	}
}

func TestParseRepo_TrailingSlash(t *testing.T) {
	owner, repo := parseRepo("https://github.com/datdt/k8sselfhost/")
	if owner != "datdt" || repo != "k8sselfhost" {
		t.Errorf("unexpected: %s/%s", owner, repo)
	}
}

func TestEncodeBase64(t *testing.T) {
	result := encodeBase64("hello world")
	expected := "aGVsbG8gd29ybGQ="
	if result != expected {
		t.Errorf("expected '%s', got '%s'", expected, result)
	}
}

func TestEncodeBase64_Empty(t *testing.T) {
	result := encodeBase64("")
	if result != "" {
		t.Errorf("expected empty, got '%s'", result)
	}
}

func TestNewProvider(t *testing.T) {
	p := NewProvider("test-token")
	if p.Name() != "github" {
		t.Errorf("expected name 'github', got '%s'", p.Name())
	}
	if p.apiURL != "https://api.github.com" {
		t.Errorf("unexpected API URL: %s", p.apiURL)
	}
}

func TestNewProviderWithURL(t *testing.T) {
	p := NewProviderWithURL("token", "https://github.company.com/api/v3/")
	if p.apiURL != "https://github.company.com/api/v3" {
		t.Errorf("unexpected API URL: %s", p.apiURL)
	}
}
