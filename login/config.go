package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const defaultBaseURL = "https://app.ourskylight.com"

// profile mirrors the generated CLI's profile shape in config.json.
type profile struct {
	BaseURL  string `json:"base_url,omitempty"`
	Token    string `json:"token,omitempty"`
	AuthType string `json:"auth_type,omitempty"`
}

type config struct {
	DefaultProfile string              `json:"default_profile"`
	Profiles       map[string]*profile `json:"profiles"`
}

func configPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "."
	}
	return filepath.Join(home, ".config", "skylight", "config.json")
}

func refreshTokenPath(profileName string) string {
	return filepath.Join(filepath.Dir(configPath()), profileName+".refresh")
}

func loadConfig() (*config, error) {
	data, err := os.ReadFile(configPath())
	if err != nil {
		if os.IsNotExist(err) {
			return &config{Profiles: map[string]*profile{}}, nil
		}
		return nil, err
	}
	var c config
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, err
	}
	if c.Profiles == nil {
		c.Profiles = map[string]*profile{}
	}
	return &c, nil
}

func saveConfig(c *config) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	dir := filepath.Dir(configPath())
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(configPath(), append(data, '\n'), 0o600)
}

// targetProfileName picks the profile to write: explicit flag, else the
// config's default, else "default".
func targetProfileName(c *config, flagProfile string) string {
	if flagProfile != "" {
		return flagProfile
	}
	if c.DefaultProfile != "" {
		return c.DefaultProfile
	}
	return "default"
}

// resolveBaseURL: profile base_url -> SKYLIGHT_BASE_URL -> default.
func resolveBaseURL(p *profile) string {
	if p != nil && p.BaseURL != "" {
		return p.BaseURL
	}
	if env := os.Getenv("SKYLIGHT_BASE_URL"); env != "" {
		return env
	}
	return defaultBaseURL
}

// persistTokens writes the access token into the named profile (creating it if
// needed, preserving others) and the refresh token to a 0600 sidecar file.
func persistTokens(profileName string, tr *tokenResponse) error {
	c, err := loadConfig()
	if err != nil {
		return err
	}
	p := c.Profiles[profileName]
	if p == nil {
		p = &profile{}
		c.Profiles[profileName] = p
	}
	p.Token = tr.AccessToken
	if c.DefaultProfile == "" {
		c.DefaultProfile = profileName
	}
	if err := saveConfig(c); err != nil {
		return err
	}
	if tr.RefreshToken != "" {
		if err := os.WriteFile(refreshTokenPath(profileName), []byte(tr.RefreshToken), 0o600); err != nil {
			return fmt.Errorf("save refresh token: %w", err)
		}
	}
	return nil
}
