// Package api provides HTTP handlers for the Peekaboo backend.
package api

import (
	"os"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

// ConnectionTracker tracks the number of active WebSocket connections.
type ConnectionTracker struct {
	count atomic.Int32
	max   int32
}

// NewConnectionTracker creates a new connection tracker with the given max limit.
func NewConnectionTracker(maxConnections int) *ConnectionTracker {
	return &ConnectionTracker{
		max: int32(maxConnections),
	}
}

// TryAcquire attempts to acquire a connection slot.
// Returns true if successful, false if at capacity.
func (ct *ConnectionTracker) TryAcquire() bool {
	for {
		current := ct.count.Load()
		if current >= ct.max {
			return false
		}
		if ct.count.CompareAndSwap(current, current+1) {
			return true
		}
		// CAS failed, another goroutine changed the value; retry
	}
}

// Release releases a connection slot.
func (ct *ConnectionTracker) Release() {
	ct.count.Add(-1)
}

// Count returns the current number of active connections.
func (ct *ConnectionTracker) Count() int {
	return int(ct.count.Load())
}

// Max returns the maximum number of allowed connections.
func (ct *ConnectionTracker) Max() int {
	return int(ct.max)
}

// getIdleTimeout returns the WebSocket idle timeout from WEBSOCKET_IDLE_TIMEOUT_SECS env var.
// Defaults to 300 seconds (5 minutes) if not set or invalid.
func getIdleTimeout() time.Duration {
	val := os.Getenv("WEBSOCKET_IDLE_TIMEOUT_SECS")
	if val == "" {
		return 5 * time.Minute
	}
	secs, err := strconv.Atoi(val)
	if err != nil || secs <= 0 {
		return 5 * time.Minute
	}
	return time.Duration(secs) * time.Second
}

// defaultMaxConnections is the default maximum number of concurrent WebSocket connections.
const defaultMaxConnections = 100

// getMaxConnections returns the max WebSocket connections from WEBSOCKET_MAX_CONNECTIONS env var.
// Defaults to 100 if not set or invalid.
func getMaxConnections() int {
	val := os.Getenv("WEBSOCKET_MAX_CONNECTIONS")
	if val == "" {
		return defaultMaxConnections
	}
	n, err := strconv.Atoi(val)
	if err != nil || n <= 0 {
		return defaultMaxConnections
	}
	return n
}

// WSAuthLimits defines the connection and interaction limits for WebSocket users.
type WSAuthLimits struct {
	AnonMaxConcurrent       int           // Max concurrent WS connections for anonymous users (per IP)
	AnonMaxInteractionsHr   int           // Max interactions per hour for anonymous users (per IP)
	RegisteredMaxConcurrent int           // Max concurrent WS connections for registered users (per user)
	InteractionWindow       time.Duration // Sliding window for interaction counting
}

// DefaultWSAuthLimits returns the default WS auth limits.
func DefaultWSAuthLimits() WSAuthLimits {
	return WSAuthLimits{
		AnonMaxConcurrent:       1,
		AnonMaxInteractionsHr:   30,
		RegisteredMaxConcurrent: 3,
		InteractionWindow:       time.Hour,
	}
}

// WSAuthTracker tracks per-user and per-IP WebSocket connections and interactions.
type WSAuthTracker struct {
	mu     sync.Mutex
	limits WSAuthLimits

	// anonConns tracks concurrent connections per IP for anonymous users.
	anonConns map[string]int

	// userConns tracks concurrent connections per userID for registered users.
	userConns map[string]int

	// anonInteractions tracks interaction timestamps per IP for anonymous users.
	anonInteractions map[string][]time.Time
}

// NewWSAuthTracker creates a new WSAuthTracker with the given limits.
func NewWSAuthTracker(limits WSAuthLimits) *WSAuthTracker {
	return &WSAuthTracker{
		limits:           limits,
		anonConns:        make(map[string]int),
		userConns:        make(map[string]int),
		anonInteractions: make(map[string][]time.Time),
	}
}

// TryAcquireAnon attempts to acquire a connection slot for an anonymous user (by IP).
// Returns true if the connection is allowed.
func (t *WSAuthTracker) TryAcquireAnon(ip string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.anonConns[ip] >= t.limits.AnonMaxConcurrent {
		return false
	}
	t.anonConns[ip]++
	return true
}

// ReleaseAnon releases a connection slot for an anonymous user.
func (t *WSAuthTracker) ReleaseAnon(ip string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.anonConns[ip]--
	if t.anonConns[ip] <= 0 {
		delete(t.anonConns, ip)
	}
}

// TryAcquireUser attempts to acquire a connection slot for a registered user.
// Returns true if the connection is allowed.
func (t *WSAuthTracker) TryAcquireUser(userID string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.userConns[userID] >= t.limits.RegisteredMaxConcurrent {
		return false
	}
	t.userConns[userID]++
	return true
}

// ReleaseUser releases a connection slot for a registered user.
func (t *WSAuthTracker) ReleaseUser(userID string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.userConns[userID]--
	if t.userConns[userID] <= 0 {
		delete(t.userConns, userID)
	}
}

// AllowAnonInteraction checks if an anonymous user (by IP) is within the interaction limit.
// Returns true if the interaction is allowed. Also records the interaction.
func (t *WSAuthTracker) AllowAnonInteraction(ip string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-t.limits.InteractionWindow)

	// Prune old interactions
	timestamps := t.anonInteractions[ip]
	pruned := timestamps[:0]
	for _, ts := range timestamps {
		if ts.After(cutoff) {
			pruned = append(pruned, ts)
		}
	}

	if len(pruned) >= t.limits.AnonMaxInteractionsHr {
		t.anonInteractions[ip] = pruned
		return false
	}

	t.anonInteractions[ip] = append(pruned, now)
	return true
}

// CleanupStaleEntries removes anonInteractions entries where all timestamps
// have expired (older than the interaction window). This prevents the map
// from growing indefinitely as new IPs connect and disconnect.
func (t *WSAuthTracker) CleanupStaleEntries() int {
	t.mu.Lock()
	defer t.mu.Unlock()

	cutoff := time.Now().Add(-t.limits.InteractionWindow)
	removed := 0
	for ip, timestamps := range t.anonInteractions {
		hasRecent := false
		for _, ts := range timestamps {
			if ts.After(cutoff) {
				hasRecent = true
				break
			}
		}
		if !hasRecent {
			delete(t.anonInteractions, ip)
			removed++
		}
	}
	return removed
}

// AnonConnCount returns the current concurrent connection count for an IP.
func (t *WSAuthTracker) AnonConnCount(ip string) int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.anonConns[ip]
}

// UserConnCount returns the current concurrent connection count for a user.
func (t *WSAuthTracker) UserConnCount(userID string) int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.userConns[userID]
}
