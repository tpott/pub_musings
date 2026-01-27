package db

import (
	"context"
	"os"
	"strconv"
	"time"

	"github.com/trevor/subtitler/backend/logging"
)

// QueryProfile holds information about a profiled query
type QueryProfile struct {
	Operation  string        // e.g., "SELECT", "INSERT", "UPDATE", "DELETE"
	Table      string        // target table name
	Duration   time.Duration // execution time
	RowsAffect int64         // rows affected (for writes)
	Error      error         // any error that occurred
}

// Profiler provides database query timing and logging
type Profiler struct {
	enabled      bool
	slowQueryMs  int64 // threshold in milliseconds
	totalQueries int64 // count of queries executed
	slowQueries  int64 // count of slow queries
}

// Default slow query threshold: 100ms
const defaultSlowQueryThreshold = 100

// NewProfiler creates a new database profiler based on environment configuration.
// Set LOG_SLOW_QUERIES=true to enable, optionally set SLOW_QUERY_THRESHOLD_MS.
func NewProfiler() *Profiler {
	enabled := os.Getenv("LOG_SLOW_QUERIES") == "true" || os.Getenv("LOG_SLOW_QUERIES") == "1"

	threshold := int64(defaultSlowQueryThreshold)
	if thresholdStr := os.Getenv("SLOW_QUERY_THRESHOLD_MS"); thresholdStr != "" {
		if parsed, err := strconv.ParseInt(thresholdStr, 10, 64); err == nil && parsed > 0 {
			threshold = parsed
		}
	}

	return &Profiler{
		enabled:     enabled,
		slowQueryMs: threshold,
	}
}

// IsEnabled returns whether profiling is enabled
func (p *Profiler) IsEnabled() bool {
	return p.enabled
}

// StartQuery begins timing a query and returns a function to call when done.
// Usage: done := profiler.StartQuery("SELECT", "users"); defer done()
func (p *Profiler) StartQuery(operation, table string) func() {
	if !p.enabled {
		return func() {} // no-op
	}

	start := time.Now()
	return func() {
		duration := time.Since(start)
		p.recordQuery(operation, table, duration, 0, nil)
	}
}

// StartQueryWithRows begins timing a query that returns affected rows.
// Usage: done := profiler.StartQueryWithRows("UPDATE", "videos"); defer done(rowsAffected, err)
func (p *Profiler) StartQueryWithRows(operation, table string) func(int64, error) {
	if !p.enabled {
		return func(int64, error) {} // no-op
	}

	start := time.Now()
	return func(rowsAffected int64, err error) {
		duration := time.Since(start)
		p.recordQuery(operation, table, duration, rowsAffected, err)
	}
}

// recordQuery logs query metrics and tracks statistics
func (p *Profiler) recordQuery(operation, table string, duration time.Duration, rowsAffected int64, err error) {
	p.totalQueries++

	durationMs := duration.Milliseconds()
	if durationMs >= p.slowQueryMs {
		p.slowQueries++

		// Log slow query as warning
		ctx := context.Background()
		if err != nil {
			logging.WarnContext(ctx, "Slow query with error",
				"operation", operation,
				"table", table,
				"duration_ms", durationMs,
				"rows_affected", rowsAffected,
				"error", err.Error(),
			)
		} else {
			logging.WarnContext(ctx, "Slow query detected",
				"operation", operation,
				"table", table,
				"duration_ms", durationMs,
				"rows_affected", rowsAffected,
			)
		}
	}
}

// Stats returns profiler statistics
func (p *Profiler) Stats() (totalQueries, slowQueries int64) {
	return p.totalQueries, p.slowQueries
}

// Reset clears profiler statistics (useful for testing)
func (p *Profiler) Reset() {
	p.totalQueries = 0
	p.slowQueries = 0
}

// Global profiler instance
var globalProfiler = NewProfiler()

// GetProfiler returns the global profiler instance
func GetProfiler() *Profiler {
	return globalProfiler
}

// Profile is a convenience function to profile a query using the global profiler.
// Usage: defer db.Profile("SELECT", "users")()
func Profile(operation, table string) func() {
	return globalProfiler.StartQuery(operation, table)
}

// ProfileWithRows is a convenience function to profile a write query using the global profiler.
// Usage: done := db.ProfileWithRows("INSERT", "users"); defer done(result.RowsAffected())
func ProfileWithRows(operation, table string) func(int64, error) {
	return globalProfiler.StartQueryWithRows(operation, table)
}
