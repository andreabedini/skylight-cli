package main

import (
	"os"
	"testing"
)

func TestPersistTokensRoundTrip(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	if err := persistTokens("default", &tokenResponse{AccessToken: "AT", RefreshToken: "RT"}); err != nil {
		t.Fatal(err)
	}

	c, err := loadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if c.Profiles["default"].Token != "AT" {
		t.Fatalf("token = %q", c.Profiles["default"].Token)
	}
	if c.DefaultProfile != "default" {
		t.Fatalf("default profile = %q", c.DefaultProfile)
	}
	// auth_type must stay empty so the generated CLI applies its "bearer" default.
	if c.Profiles["default"].AuthType != "" {
		t.Fatalf("auth_type = %q, want empty", c.Profiles["default"].AuthType)
	}

	rt, err := os.ReadFile(refreshTokenPath("default"))
	if err != nil {
		t.Fatal(err)
	}
	if string(rt) != "RT" {
		t.Fatalf("refresh = %q", string(rt))
	}

	info, err := os.Stat(configPath())
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("config mode = %v, want 0600", info.Mode().Perm())
	}

	// The refresh sidecar holds a secret; it must be owner-only too.
	rtInfo, err := os.Stat(refreshTokenPath("default"))
	if err != nil {
		t.Fatal(err)
	}
	if rtInfo.Mode().Perm() != 0o600 {
		t.Fatalf("refresh sidecar mode = %v, want 0600", rtInfo.Mode().Perm())
	}
}

func TestPersistPreservesOtherProfiles(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	seed := &config{DefaultProfile: "work", Profiles: map[string]*profile{
		"work": {Token: "existing", BaseURL: "https://example.test"},
	}}
	if err := saveConfig(seed); err != nil {
		t.Fatal(err)
	}

	if err := persistTokens("home", &tokenResponse{AccessToken: "NEW"}); err != nil {
		t.Fatal(err)
	}

	got, err := loadConfig()
	if err != nil {
		t.Fatal(err)
	}
	if got.Profiles["work"].Token != "existing" || got.Profiles["work"].BaseURL != "https://example.test" {
		t.Fatalf("clobbered existing profile: %+v", got.Profiles["work"])
	}
	if got.Profiles["home"].Token != "NEW" {
		t.Fatalf("new token not written: %+v", got.Profiles["home"])
	}
	if got.DefaultProfile != "work" {
		t.Fatalf("default profile changed to %q", got.DefaultProfile)
	}
}

func TestResolveBaseURL(t *testing.T) {
	t.Setenv("SKYLIGHT_BASE_URL", "")
	if got := resolveBaseURL(&profile{BaseURL: "https://p.test"}); got != "https://p.test" {
		t.Fatalf("profile base = %q", got)
	}
	if got := resolveBaseURL(nil); got != "https://app.ourskylight.com" {
		t.Fatalf("default base = %q", got)
	}
	t.Setenv("SKYLIGHT_BASE_URL", "https://env.test")
	if got := resolveBaseURL(&profile{}); got != "https://env.test" {
		t.Fatalf("env base = %q", got)
	}
}
