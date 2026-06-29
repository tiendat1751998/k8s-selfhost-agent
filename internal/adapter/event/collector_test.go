package event

import "testing"

func TestMatchesSelector_Match(t *testing.T) {
	podLabels := map[string]string{
		"app":     "nginx",
		"version": "v1",
		"env":     "prod",
	}
	selector := map[string]string{
		"app":     "nginx",
		"version": "v1",
	}

	if !matchesSelector(podLabels, selector) {
		t.Error("expected pod labels to match selector")
	}
}

func TestMatchesSelector_NoMatch(t *testing.T) {
	podLabels := map[string]string{
		"app":     "nginx",
		"version": "v1",
	}
	selector := map[string]string{
		"app":     "redis",
		"version": "v1",
	}

	if matchesSelector(podLabels, selector) {
		t.Error("expected pod labels to NOT match selector")
	}
}

func TestMatchesSelector_EmptySelector(t *testing.T) {
	podLabels := map[string]string{
		"app": "nginx",
	}

	if matchesSelector(podLabels, map[string]string{}) {
		t.Error("expected empty selector to not match")
	}
}

func TestMatchesSelector_NilSelector(t *testing.T) {
	podLabels := map[string]string{
		"app": "nginx",
	}

	if matchesSelector(podLabels, nil) {
		t.Error("expected nil selector to not match")
	}
}

func TestMatchesSelector_MissingLabel(t *testing.T) {
	podLabels := map[string]string{
		"app": "nginx",
	}
	selector := map[string]string{
		"app":     "nginx",
		"version": "v1",
	}

	if matchesSelector(podLabels, selector) {
		t.Error("expected missing label to cause no match")
	}
}
