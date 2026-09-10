package jed

import "testing"

func TestRenderSkipsIntegration(t *testing.T) {
	// This same-package test exercises the unexported render path directly.
	// Public Spec paths validate Integration before rendering, so they cannot carry
	// template-looking integration values far enough to prove expand:"exclude" behavior.
	svc := Service{
		Name:        "app",
		Integration: &Integration{Name: "infra", Versions: []string{"v1.2.3+{{BUILD}}"}},
		Image:       "app:{{TAG}}",
	}

	spec, err := render(svc, Env{Name: "app", Vars: map[string]string{}}, map[string]string{"BUILD": "7", "TAG": "v1"})
	if err != nil {
		t.Fatalf("render failed: %v", err)
	}
	if spec.Service.Image != "app:v1" {
		t.Fatalf("image = %q, want %q", spec.Service.Image, "app:v1")
	}
	if spec.Service.Integration == nil {
		t.Fatal("integration is nil")
	}
	if got := spec.Service.Integration.Versions; len(got) != 1 || got[0] != "v1.2.3+{{BUILD}}" {
		t.Fatalf("integration versions = %q, want unrendered template", got)
	}
}
