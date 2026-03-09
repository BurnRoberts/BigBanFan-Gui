// Package mgmt implements the BigBanFan management protocol client.
//
// Client maintains a persistent encrypted connection to a BigBanFan node's
// management port (default :7779). It uses the same AES-256-GCM + HMAC-SHA256
// framing as the server-side client.go, keeping the connection alive for
// bidirectional request/response and real-time push events.
package mgmt

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	cryptotls "crypto/tls"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/BurnRoberts/BigBanFan-Gui/connection"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// ── Result types (returned to JS frontend) ────────────────────────────────────

// BanRecord mirrors proto.BanRecord for the Svelte frontend.
type BanRecord struct {
	ID        int64  `json:"id"`
	IP        string `json:"ip"`
	DedupeID  string `json:"dedupe_id"`
	BannedAt  int64  `json:"banned_at"`
	ExpiresAt int64  `json:"expires_at"`
	Source    string `json:"source"`
	Reason    string `json:"reason,omitempty"`
}

// PeerRecord mirrors proto.PeerRecord.
type PeerRecord struct {
	NodeID    string `json:"node_id"`
	Addr      string `json:"addr"`
	Connected bool   `json:"connected"`
	LastSeen  int64  `json:"last_seen"`
	Direction string `json:"direction"`
}

// StatusResult holds node health info.
type StatusResult struct {
	NodeID    string `json:"node_id"`
	Version   string `json:"version"`
	UptimeSec int64  `json:"uptime_sec"`
	PeerCount int    `json:"peer_count"`
	BanCount  int    `json:"ban_count"`
}

// StatsResult holds session counters.
type StatsResult struct {
	BansThisSession        int64 `json:"bans_this_session"`
	UnbansThisSession      int64 `json:"unbans_this_session"`
	ScanDetectsThisSession int64 `json:"scan_detects_this_session"`
	ConnectionsAccepted    int64 `json:"connections_accepted"`
}

// BansResult holds a paginated ban list.
type BansResult struct {
	Total int         `json:"total"`
	Page  int         `json:"page"`
	Bans  []BanRecord `json:"bans"`
}

// PeersResult holds the peer list.
type PeersResult struct {
	Peers []PeerRecord `json:"peers"`
}

// ── Wire message (matches bigbanfan proto.Message JSON) ───────────────────────

type wireMsg struct {
	Type             string        `json:"type"`
	NodeID           string        `json:"node_id,omitempty"`
	IP               string        `json:"ip,omitempty"`
	Reason           string        `json:"reason,omitempty"`
	DedupeID         string        `json:"dedupe_id,omitempty"`
	Ts               int64         `json:"ts"`
	Page             int           `json:"page,omitempty"`
	PageSize         int           `json:"page_size,omitempty"`
	Search           string        `json:"search,omitempty"`
	FilterSource     string        `json:"filter_source,omitempty"`
	FilterActiveOnly bool          `json:"filter_active_only,omitempty"`
	LogLevel         string        `json:"log_level,omitempty"`
	Total            int           `json:"total,omitempty"`
	Bans             []BanRecord   `json:"bans,omitempty"`
	Peers            []PeerRecord  `json:"peers,omitempty"`
	Status           *StatusResult `json:"status,omitempty"`
	Stats            *StatsResult  `json:"stats,omitempty"`
	Line             string        `json:"line,omitempty"`
	LogLines         []string      `json:"log_lines,omitempty"`
	ErrorMsg         string        `json:"error,omitempty"`
}

// ── Client ────────────────────────────────────────────────────────────────────

// Client manages one persistent management connection.
type Client struct {
	ctx context.Context
	mu  sync.Mutex // protects conn, key
	// sendMu serializes frame writes; kept separate from mu so Connect/Disconnect
	// are never blocked by a slow or hung conn.Write call.
	sendMu    sync.Mutex
	conn      net.Conn
	key       []byte
	connected atomic.Bool
	closing   atomic.Bool  // true during app shutdown — suppresses EventsEmit
	connID    atomic.Int64 // incremented on each Connect(); readLoop checks it to avoid stale events

	// pending holds one-shot response channels keyed by request type.
	// Only one outstanding request per type at a time (adequate for GUI use).
	pendingMu sync.Mutex
	pending   map[string]chan *wireMsg
}

// NewClient creates an idle Client.
func NewClient(ctx context.Context) *Client {
	return &Client{
		ctx:     ctx,
		pending: make(map[string]chan *wireMsg),
	}
}

// Connect opens a persistent TLS 1.3 + AES-256-GCM connection to a BigBanFan node.
func (c *Client) Connect(profile connection.Profile) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn != nil {
		// Increment connID BEFORE closing the old connection. The old readLoop
		// goroutine will see a stale ID and skip the connection:down event,
		// preventing a spurious logout when switching nodes.
		c.connID.Add(1)
		c.conn.Close()
		c.conn = nil
	}

	keyBytes, err := hex.DecodeString(profile.KeyHex)
	if err != nil || len(keyBytes) != 32 {
		return fmt.Errorf("invalid client key: must be 64 hex chars (32 bytes)")
	}
	c.key = keyBytes

	host := profile.Host
	// Wrap bare IPv6 addresses in brackets so net.Dial parses them correctly.
	// e.g. "2604:4300::1" → "[2604:4300::1]"
	if strings.Contains(host, ":") && !strings.HasPrefix(host, "[") {
		host = "[" + host + "]"
	}
	addr := fmt.Sprintf("%s:%d", host, profile.Port)

	// TLS 1.3 — skip cert verify (self-signed); HMAC on every frame provides mutual auth.
	tlsCfg := &cryptotls.Config{
		InsecureSkipVerify: true, //nolint:gosec — intentional, HMAC is the auth layer
		MinVersion:         cryptotls.VersionTLS13,
	}
	dialer := &net.Dialer{Timeout: 10 * time.Second}
	conn, err := cryptotls.DialWithDialer(dialer, "tcp", addr, tlsCfg)
	if err != nil {
		return fmt.Errorf("connect %s: %w", addr, err)
	}
	c.conn = conn
	c.closing.Store(false)
	c.connected.Store(true)
	myID := c.connID.Add(1) // unique ID for this connection's readLoop

	go c.readLoop(myID)
	go c.keepalive(myID)

	runtime.EventsEmit(c.ctx, "connection:up", profile.Host)
	return nil
}

// Disconnect closes the active connection (user-initiated — emits connection:down).
func (c *Client) Disconnect() {
	c.mu.Lock()
	// Increment connID first so readLoop's defer sees a stale ID and skips
	// emitting its own connection:down — preventing a duplicate event.
	c.connID.Add(1)
	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}
	c.mu.Unlock()
	c.connected.Store(false)
	runtime.EventsEmit(c.ctx, "connection:down", nil)
}

// Close is called during app shutdown. It closes the connection but does NOT
// emit any Wails events — the WebView may already be destroyed at this point.
func (c *Client) Close() {
	c.closing.Store(true)
	c.mu.Lock()
	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}
	c.mu.Unlock()
	c.connected.Store(false)
}

// IsConnected returns true if the connection is active.
func (c *Client) IsConnected() bool {
	return c.connected.Load()
}

// ── Request helpers ───────────────────────────────────────────────────────────

// request sends a message and waits for a response matching responseType.
func (c *Client) request(req *wireMsg, responseType string) (*wireMsg, error) {
	ch := make(chan *wireMsg, 1)
	c.pendingMu.Lock()
	c.pending[responseType] = ch
	c.pendingMu.Unlock()

	if err := c.send(req); err != nil {
		c.pendingMu.Lock()
		delete(c.pending, responseType)
		c.pendingMu.Unlock()
		return nil, err
	}

	timer := time.NewTimer(10 * time.Second)
	defer timer.Stop()

	select {
	case msg := <-ch:
		if msg.Type == "ERROR" {
			return nil, fmt.Errorf("node error: %s", msg.ErrorMsg)
		}
		return msg, nil
	case <-timer.C:
		c.pendingMu.Lock()
		delete(c.pending, responseType)
		c.pendingMu.Unlock()
		return nil, fmt.Errorf("request timed out")
	case <-c.ctx.Done():
		// Connection is being torn down (node switch or app shutdown).
		c.pendingMu.Lock()
		delete(c.pending, responseType)
		c.pendingMu.Unlock()
		return nil, fmt.Errorf("request cancelled: %w", c.ctx.Err())
	}
}

// GetStatus requests STATUS from the node.
func (c *Client) GetStatus() (*StatusResult, error) {
	resp, err := c.request(&wireMsg{Type: "STATUS", Ts: now()}, "STATUS_REPLY")
	if err != nil {
		return nil, err
	}
	return resp.Status, nil
}

// GetPeers requests LIST_PEERS from the node.
func (c *Client) GetPeers() (*PeersResult, error) {
	resp, err := c.request(&wireMsg{Type: "LIST_PEERS", Ts: now()}, "PEERS_LIST")
	if err != nil {
		return nil, err
	}
	return &PeersResult{Peers: resp.Peers}, nil
}

// GetStats requests STATS from the node.
func (c *Client) GetStats() (*StatsResult, error) {
	resp, err := c.request(&wireMsg{Type: "STATS", Ts: now()}, "STATS_REPLY")
	if err != nil {
		return nil, err
	}
	return resp.Stats, nil
}

// GetBans requests LIST_BANS with server-side search and pagination.
func (c *Client) GetBans(page, pageSize int, search, filterSource string, activeOnly bool) (*BansResult, error) {
	resp, err := c.request(&wireMsg{
		Type:             "LIST_BANS",
		Page:             page,
		PageSize:         pageSize,
		Search:           search,
		FilterSource:     filterSource,
		FilterActiveOnly: activeOnly,
		Ts:               now(),
	}, "BANS_LIST")
	if err != nil {
		return nil, err
	}
	return &BansResult{Total: resp.Total, Page: resp.Page, Bans: resp.Bans}, nil
}

// BanIP submits an IP/CIDR for banning across the cluster.
// reason is optional; pass an empty string to omit it.
// Reason is silently truncated to 1024 chars, matching the server's own limit.
func (c *Client) BanIP(ip, reason string) error {
	const maxReasonLen = 1024
	if len([]rune(reason)) > maxReasonLen {
		reason = string([]rune(reason)[:maxReasonLen])
	}
	return c.send(&wireMsg{Type: "BAN", IP: ip, Reason: reason, Ts: now()})
}

// UnbanIP lifts an existing ban.
func (c *Client) UnbanIP(ip string) error {
	return c.send(&wireMsg{Type: "UNBAN", IP: ip, Ts: now()})
}

// GetLogs retrieves the last 100 log lines buffered since daemon startup.
func (c *Client) GetLogs() ([]string, error) {
	resp, err := c.request(&wireMsg{Type: "GET_LOGS", Ts: now()}, "LOGS_REPLY")
	if err != nil {
		return nil, err
	}
	return resp.LogLines, nil
}

// SubscribeLogs starts streaming log lines from the node.
func (c *Client) SubscribeLogs(level string) error {
	return c.send(&wireMsg{Type: "LOG_SUBSCRIBE", LogLevel: level, Ts: now()})
}

// UnsubscribeLogs stops the log stream.
func (c *Client) UnsubscribeLogs() error {
	return c.send(&wireMsg{Type: "LOG_UNSUBSCRIBE", Ts: now()})
}

// ── Keepalive ─────────────────────────────────────────────────────────────────

// keepalive sends a STATUS request every 60 seconds while the connection with
// the given connID is active. This prevents the 120s read deadline in readLoop
// from firing during idle periods (e.g. quiet log streams with no push events).
// The goroutine exits automatically when the connection is replaced or closed.
func (c *Client) keepalive(myConnID int64) {
	ticker := time.NewTicker(60 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			// Stop if this connection is no longer the active one.
			if c.connID.Load() != myConnID {
				return
			}
			// Best-effort STATUS ping — ignore result; the response goes
			// through readLoop normally which refreshes the read deadline.
			if _, err := c.GetStatus(); err != nil {
				// Connection is dead; readLoop will handle cleanup.
				return
			}
		case <-c.ctx.Done():
			return
		}
	}
}

// ── Read loop (push event router) ────────────────────────────────────────────

// readLoop continuously reads and dispatches incoming frames.
func (c *Client) readLoop(myConnID int64) {
	defer func() {
		c.connected.Store(false)
		// Only emit connection:down if:
		// - This is still the active connection (same connID), AND
		// - App is not shutting down (closing flag not set)
		if c.connID.Load() == myConnID && !c.closing.Load() {
			runtime.EventsEmit(c.ctx, "connection:down", nil)
		}
	}()

	for {
		c.mu.Lock()
		conn := c.conn
		c.mu.Unlock()
		if conn == nil {
			return
		}

		// Refresh read deadline before each frame. If the server drops the
		// connection silently (TCP half-open), we detect it within 120s instead
		// of hanging the goroutine indefinitely. The keepalive goroutine sends
		// STATUS every 60s so the deadline is always refreshed during idle periods.
		conn.SetReadDeadline(time.Now().Add(120 * time.Second)) //nolint:errcheck
		raw, err := readFrame(conn, c.key)
		if err != nil {
			if c.connected.Load() {
				runtime.LogWarningf(c.ctx, "mgmt read: %v", err)
			}
			return
		}

		var msg wireMsg
		if err := json.Unmarshal(raw, &msg); err != nil {
			runtime.LogWarningf(c.ctx, "mgmt decode: %v", err)
			continue
		}

		c.dispatch(&msg)
	}
}

// dispatch routes an incoming message to either a pending request channel
// or a Wails frontend event.
func (c *Client) dispatch(msg *wireMsg) {
	switch msg.Type {
	// Response to a pending request — deliver to the waiting goroutine.
	case "BANS_LIST", "STATUS_REPLY", "PEERS_LIST", "STATS_REPLY", "LOGS_REPLY", "ERROR":
		c.pendingMu.Lock()
		ch, ok := c.pending[msg.Type]
		if !ok && msg.Type == "ERROR" {
			// Server sent ERROR but the request was registered under its reply type
			// (e.g. "STATUS_REPLY"). Fan it out to whichever channel is waiting.
			// Only one request can be in-flight per type, so the first match wins.
			for k, waitCh := range c.pending {
				ch = waitCh
				delete(c.pending, k)
				ok = true
				break
			}
		} else if ok {
			delete(c.pending, msg.Type)
		}
		c.pendingMu.Unlock()
		if ok {
			ch <- msg
		}

	// "ERROR" may also be a push if no pending request — emit to frontend.
	// Push events — emit to Svelte frontend via Wails EventsEmit.
	case "BAN_EVENT":
		runtime.EventsEmit(c.ctx, "ban:new", map[string]any{
			"ip":        msg.IP,
			"dedupe_id": msg.DedupeID,
			"ts":        msg.Ts,
			"source":    msg.NodeID,
			"reason":    msg.Reason,
		})
	case "UNBAN_EVENT":
		runtime.EventsEmit(c.ctx, "ban:removed", map[string]any{
			"ip": msg.IP,
			"ts": msg.Ts,
		})
	case "PEER_UP":
		runtime.EventsEmit(c.ctx, "peer:up", map[string]any{
			"node_id": msg.NodeID,
			"addr":    msg.IP,
			"ts":      msg.Ts,
		})
	case "PEER_DOWN":
		runtime.EventsEmit(c.ctx, "peer:down", map[string]any{
			"node_id": msg.NodeID,
			"addr":    msg.IP,
			"ts":      msg.Ts,
		})
	case "LOG_LINE":
		runtime.EventsEmit(c.ctx, "log:line", msg.Line)
	}
}

// ── Frame codec (mirrors bigbanfan server-side framing) ───────────────────────
//
// Frame layout:
//   [4-byte big-endian length][32-byte HMAC-SHA256][AES-256-GCM ciphertext]
//
// The HMAC signs the ciphertext only.

const hmacSize = 32

func (c *Client) send(msg *wireMsg) error {
	// Encode first — pure CPU, no shared state needed.
	plain, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	frame, err := encodeFrame(c.key, plain)
	if err != nil {
		return err
	}

	// Serialise writes with sendMu (not c.mu) so Connect/Disconnect are never
	// blocked by a slow or hung Write.
	c.sendMu.Lock()
	defer c.sendMu.Unlock()

	c.mu.Lock()
	conn := c.conn
	c.mu.Unlock()
	if conn == nil {
		return fmt.Errorf("not connected")
	}

	// Bound the write so a stuck server frees sendMu within 10 s.
	conn.SetWriteDeadline(time.Now().Add(10 * time.Second)) //nolint:errcheck
	_, err = conn.Write(frame)
	conn.SetWriteDeadline(time.Time{}) //nolint:errcheck — clear deadline
	return err
}

func encodeFrame(key, plaintext []byte) ([]byte, error) {
	// AES-256-GCM encrypt.
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)

	// HMAC the ciphertext.
	mac := hmac.New(sha256.New, key)
	mac.Write(ciphertext)
	sig := mac.Sum(nil)

	// Build: [4-byte len][32-byte hmac][ciphertext]
	payload := append(sig, ciphertext...)
	lenBuf := make([]byte, 4)
	binary.BigEndian.PutUint32(lenBuf, uint32(len(payload)))
	return append(lenBuf, payload...), nil
}

func readFrame(conn net.Conn, key []byte) ([]byte, error) {
	// Read length prefix.
	lenBuf := make([]byte, 4)
	if _, err := io.ReadFull(conn, lenBuf); err != nil {
		return nil, err
	}
	length := binary.BigEndian.Uint32(lenBuf)
	if length < hmacSize || length > 64*1024 {
		return nil, fmt.Errorf("invalid frame length %d", length)
	}

	// Read payload.
	payload := make([]byte, length)
	if _, err := io.ReadFull(conn, payload); err != nil {
		return nil, err
	}

	sig := payload[:hmacSize]
	ciphertext := payload[hmacSize:]

	// Verify HMAC.
	mac := hmac.New(sha256.New, key)
	mac.Write(ciphertext)
	expected := mac.Sum(nil)
	if !hmac.Equal(sig, expected) {
		return nil, fmt.Errorf("HMAC verification failed")
	}

	// Decrypt AES-256-GCM.
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}
	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	return gcm.Open(nil, nonce, ciphertext, nil)
}

func now() int64 { return time.Now().Unix() }
