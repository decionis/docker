// Package store keeps the daemon's connection settings and org API key at
// rest inside the backend's private data directory, per
// rules/security.rules.md Rule 2.5: owner-only permissions, atomic writes,
// and the key in its own file so settings can be read without touching it.
package store

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Connection is the non-secret part of the stored settings.
type Connection struct {
	BaseURL string `json:"base_url"`
	OrgID   string `json:"org_id"`
}

// Store persists connection state under a single directory.
type Store struct {
	dir string
}

// New returns a store rooted at dir (created 0700 on first save).
func New(dir string) *Store {
	return &Store{dir: dir}
}

func (s *Store) connectionPath() string { return filepath.Join(s.dir, "connection.json") }
func (s *Store) apiKeyPath() string     { return filepath.Join(s.dir, "api-key") }

func writeFileAtomic(path string, data []byte) error {
	temp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	tempName := temp.Name()
	defer os.Remove(tempName)
	if err := temp.Chmod(0o600); err != nil {
		temp.Close()
		return err
	}
	if _, err := temp.Write(data); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(tempName, path)
}

// Save persists the connection settings and API key with 0600 permissions.
func (s *Store) Save(connection Connection, apiKey string) error {
	if strings.TrimSpace(apiKey) == "" {
		return errors.New("store: refusing to save an empty api key")
	}
	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return fmt.Errorf("store: create data dir: %w", err)
	}
	settings, err := json.Marshal(connection)
	if err != nil {
		return fmt.Errorf("store: encode settings: %w", err)
	}
	if err := writeFileAtomic(s.connectionPath(), settings); err != nil {
		return fmt.Errorf("store: write settings: %w", err)
	}
	if err := writeFileAtomic(s.apiKeyPath(), []byte(apiKey)); err != nil {
		return fmt.Errorf("store: write api key: %w", err)
	}
	return nil
}

// Load returns the stored connection and key. ok is false when no connection
// has been saved.
func (s *Store) Load() (connection Connection, apiKey string, ok bool, err error) {
	settings, readErr := os.ReadFile(s.connectionPath())
	if readErr != nil {
		if os.IsNotExist(readErr) {
			return Connection{}, "", false, nil
		}
		return Connection{}, "", false, fmt.Errorf("store: read settings: %w", readErr)
	}
	if err := json.Unmarshal(settings, &connection); err != nil {
		return Connection{}, "", false, errors.New("store: settings file is corrupt")
	}
	keyBytes, readErr := os.ReadFile(s.apiKeyPath())
	if readErr != nil {
		if os.IsNotExist(readErr) {
			return Connection{}, "", false, nil
		}
		return Connection{}, "", false, fmt.Errorf("store: read api key: %w", readErr)
	}
	apiKey = strings.TrimSpace(string(keyBytes))
	if apiKey == "" {
		return Connection{}, "", false, nil
	}
	return connection, apiKey, true, nil
}

// Clear removes stored settings and the API key.
func (s *Store) Clear() error {
	var firstErr error
	for _, path := range []string{s.apiKeyPath(), s.connectionPath()} {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
