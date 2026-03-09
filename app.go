// Package main — BigBanFan GUI application backend.
//
// App is the Wails application struct. All exported methods are callable
// from the Svelte frontend via the auto-generated wailsjs bindings.
package main

import (
	"context"
	"os"
	"path/filepath"

	"github.com/BurnRoberts/BigBanFan-Gui/connection"
	"github.com/BurnRoberts/BigBanFan-Gui/mgmt"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App is the Wails application struct.
type App struct {
	ctx      context.Context
	profiles *connection.Manager
	client   *mgmt.Client
}

// NewApp creates a new App.
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts. The context is saved so we can
// emit events and use other Wails runtime functions.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	// Profiles stored in OS-standard app data dir (XDG on Linux).
	dataDir := appDataDir()
	var err error
	a.profiles, err = connection.NewManager(dataDir)
	if err != nil {
		runtime.LogErrorf(ctx, "profiles init: %v", err)
	}

	a.client = mgmt.NewClient(ctx)
}

// shutdown is called when the app is about to quit.
// Use Close() (not Disconnect) to avoid EventsEmit after WebView is destroyed.
func (a *App) shutdown(ctx context.Context) {
	if a.client != nil {
		a.client.Close()
	}
}

// ── Connection profiles ───────────────────────────────────────────────────────

// ListProfiles returns all saved connection profiles.
func (a *App) ListProfiles() []connection.Profile {
	return a.profiles.List()
}

// SaveProfile creates or updates a named connection profile.
func (a *App) SaveProfile(profile connection.Profile) error {
	return a.profiles.Save(profile)
}

// DeleteProfile removes a saved profile by name.
func (a *App) DeleteProfile(name string) error {
	return a.profiles.Delete(name)
}

// ── Connection lifecycle ──────────────────────────────────────────────────────

// Connect opens a persistent management connection to a node.
// profile.KeyHex must be the 64-char hex client_key from config.yaml.
func (a *App) Connect(profile connection.Profile) error {
	return a.client.Connect(profile)
}

// Disconnect closes the active management connection.
func (a *App) Disconnect() {
	a.client.Disconnect()
}

// IsConnected returns true if the management connection is active.
func (a *App) IsConnected() bool {
	return a.client.IsConnected()
}

// ── Queries (request → response) ─────────────────────────────────────────────

// GetStatus requests node identity and health summary.
func (a *App) GetStatus() (*mgmt.StatusResult, error) {
	return a.client.GetStatus()
}

// GetPeers requests the peer list with connection state.
func (a *App) GetPeers() (*mgmt.PeersResult, error) {
	return a.client.GetPeers()
}

// GetStats requests session-only counters from the node.
func (a *App) GetStats() (*mgmt.StatsResult, error) {
	return a.client.GetStats()
}

// GetBans requests a paginated, filtered ban list from the node.
// search is forwarded as-is; the node enforces a 3-char minimum for LIKE queries.
func (a *App) GetBans(page, pageSize int, search, filterSource string, activeOnly bool) (*mgmt.BansResult, error) {
	return a.client.GetBans(page, pageSize, search, filterSource, activeOnly)
}

// ── Ban actions ───────────────────────────────────────────────────────────────

// BanIP submits an IP or CIDR for cluster-wide banning.
// reason is optional; pass an empty string to omit it from the wire message.
func (a *App) BanIP(ip, reason string) error {
	return a.client.BanIP(ip, reason)
}

// UnbanIP lifts an existing ban on an IP or CIDR.
func (a *App) UnbanIP(ip string) error {
	return a.client.UnbanIP(ip)
}

// ── Log stream ────────────────────────────────────────────────────────────────

// GetLogs retrieves the last 100 log lines buffered by the node since startup.
// Call this immediately after connecting to pre-populate the log view.
func (a *App) GetLogs() ([]string, error) {
	return a.client.GetLogs()
}

// SubscribeLogs starts streaming log lines from the node.
// level is "info", "warn", or "error".
func (a *App) SubscribeLogs(level string) error {
	return a.client.SubscribeLogs(level)
}

// UnsubscribeLogs stops the log stream.
func (a *App) UnsubscribeLogs() error {
	return a.client.UnsubscribeLogs()
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// appDataDir returns the platform-standard app data directory.
func appDataDir() string {
	base, err := os.UserConfigDir()
	if err != nil {
		base = filepath.Join(os.Getenv("HOME"), ".config")
	}
	return filepath.Join(base, "bigbanfan-gui")
}
