package config

import "testing"

func TestDefaultSettingsAndThemes(t *testing.T) {
	store := NewStore("")
	cfg := store.Settings()
	if cfg.LayoutMode != "tabbed" {
		t.Fatalf("expected default layoutMode 'tabbed', got %q", cfg.LayoutMode)
	}
	if !cfg.BlurEnabled || cfg.BlurRadius <= 0 {
		t.Fatalf("expected blur enabled with positive blurRadius")
	}
	if !cfg.BorderGlow || cfg.GlowIntensity <= 0 {
		t.Fatalf("expected borderGlow enabled with positive glowIntensity")
	}
	themes := store.Themes()
	if len(themes) != 4 {
		t.Fatalf("expected 4 anime themes, got %d", len(themes))
	}
	expected := []string{"neon-tokyo", "sakura-lofi", "crimson-shonen", "eva-midnight"}
	for i, id := range expected {
		if themes[i].ID != id {
			t.Fatalf("expected theme[%d]=%q, got %q", i, id, themes[i].ID)
		}
	}
}
