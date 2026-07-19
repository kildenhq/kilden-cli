package config

import (
	"errors"
	"testing"
	"time"
)

func TestSaveLoadClear(t *testing.T) {
	// Redirect the config dir to a temp location on every platform.
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())

	if _, err := Load(); !errors.Is(err, ErrNotLoggedIn) {
		t.Fatalf("expected ErrNotLoggedIn before any save, got %v", err)
	}

	want := &Credentials{
		Host:         "https://panel.example",
		ClientID:     "cid",
		AccessToken:  "at",
		RefreshToken: "rt",
		TokenType:    "Bearer",
		Expiry:       time.Now().Add(time.Hour).Round(time.Second),
		Scopes:       "account:read",
	}
	if err := want.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got.AccessToken != want.AccessToken || got.Host != want.Host || !got.Expiry.Equal(want.Expiry) {
		t.Fatalf("roundtrip mismatch:\n got %+v\nwant %+v", got, want)
	}

	if err := Clear(); err != nil {
		t.Fatalf("Clear: %v", err)
	}
	if _, err := Load(); !errors.Is(err, ErrNotLoggedIn) {
		t.Fatalf("expected ErrNotLoggedIn after clear, got %v", err)
	}
	// Clear is idempotent.
	if err := Clear(); err != nil {
		t.Fatalf("second Clear should be a no-op: %v", err)
	}
}
