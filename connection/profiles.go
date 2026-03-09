// Package connection manages saved BigBanFan node connection profiles.
//
// Profiles are stored as JSON in the Wails app data directory so they
// persist across sessions (like saved logins in a DB client).
package connection

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// Profile holds the credentials and address needed to connect to one
// BigBanFan management port.
type Profile struct {
	Name   string `json:"name"`
	Host   string `json:"host"`
	Port   int    `json:"port"`
	KeyHex string `json:"key_hex"` // 64-char hex client_key
}

// Manager persists and retrieves connection profiles.
type Manager struct {
	mu       sync.RWMutex
	profiles map[string]Profile
	dataDir  string
}

// NewManager creates a Manager that persists profiles to dataDir/profiles.json.
func NewManager(dataDir string) (*Manager, error) {
	if err := os.MkdirAll(dataDir, 0700); err != nil {
		return nil, fmt.Errorf("profiles: mkdir %s: %w", dataDir, err)
	}
	m := &Manager{
		profiles: make(map[string]Profile),
		dataDir:  dataDir,
	}
	_ = m.load() // OK if file doesn't exist yet
	return m, nil
}

func (m *Manager) filePath() string {
	return filepath.Join(m.dataDir, "profiles.json")
}

func (m *Manager) load() error {
	data, err := os.ReadFile(m.filePath())
	if err != nil {
		return err
	}
	var profiles []Profile
	if err := json.Unmarshal(data, &profiles); err != nil {
		return err
	}
	for _, p := range profiles {
		m.profiles[p.Name] = p
	}
	return nil
}

func (m *Manager) save() error {
	profiles := make([]Profile, 0, len(m.profiles))
	for _, p := range m.profiles {
		profiles = append(profiles, p)
	}
	data, err := json.MarshalIndent(profiles, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.filePath(), data, 0600)
}

// Save creates or updates a named profile.
func (m *Manager) Save(p Profile) error {
	if p.Name == "" {
		return fmt.Errorf("profile name is required")
	}
	if p.Host == "" {
		return fmt.Errorf("host is required")
	}
	if p.Port <= 0 || p.Port > 65535 {
		return fmt.Errorf("port must be 1–65535")
	}
	if len(p.KeyHex) != 64 {
		return fmt.Errorf("client key must be 64 hex chars (32 bytes)")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.profiles[p.Name] = p
	return m.save()
}

// Delete removes a named profile.
func (m *Manager) Delete(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.profiles, name)
	return m.save()
}

// List returns all saved profiles, including KeyHex.
// KeyHex exposure is acceptable here because this is a local desktop app with
// no network-accessible API — the key never leaves the process except when
// sent over the already-encrypted management connection.
func (m *Manager) List() []Profile {
	m.mu.RLock()
	defer m.mu.RUnlock()
	profiles := make([]Profile, 0, len(m.profiles))
	for _, p := range m.profiles {
		profiles = append(profiles, p)
	}
	return profiles
}

// Get retrieves a single profile by name.
func (m *Manager) Get(name string) (Profile, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	p, ok := m.profiles[name]
	return p, ok
}
