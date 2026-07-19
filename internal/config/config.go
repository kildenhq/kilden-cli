// Package config stores the CLI's credentials and endpoint on disk.
//
// Everything the CLI needs to make an authenticated request — the panel host,
// the OAuth client id and the tokens the device flow returned — lives in a
// single credentials.json under the user's config dir, written 0600.
package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

const (
	// DefaultHost is the hosted Kilden panel. Override at login with --host
	// (self-hosters) or the KILDEN_HOST environment variable.
	DefaultHost = "https://app.kilden.io"

	// DefaultClientID is the well-known PUBLIC OAuth client the panel seeds
	// with `php artisan passport:cli-client`. A public client carries no
	// secret, so shipping its id in the binary is expected, not a leak.
	DefaultClientID = "9c3f6a10-1b2c-4d5e-8f90-a1b2c3d4e5f6"
)

// ErrNotLoggedIn is returned when no credentials are stored yet.
var ErrNotLoggedIn = errors.New("not signed in — run `kd login`")

// Credentials is the persisted session.
type Credentials struct {
	Host         string    `json:"host"`
	ClientID     string    `json:"client_id"`
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	TokenType    string    `json:"token_type"`
	Expiry       time.Time `json:"expiry"`
	Scopes       string    `json:"scopes,omitempty"`
}

// Dir is the CLI's config directory (e.g. ~/.config/kilden on Linux).
func Dir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "kilden"), nil
}

func credentialsPath() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "credentials.json"), nil
}

// Load reads the stored credentials, or ErrNotLoggedIn if none exist.
func Load() (*Credentials, error) {
	path, err := credentialsPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, ErrNotLoggedIn
		}
		return nil, err
	}
	var c Credentials
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, err
	}
	return &c, nil
}

// Save writes the credentials back, creating the config dir if needed.
func (c *Credentials) Save() error {
	dir, err := Dir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "credentials.json"), data, 0o600)
}

// Clear removes the stored credentials (idempotent).
func Clear() error {
	path, err := credentialsPath()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}
