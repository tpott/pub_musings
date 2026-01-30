package db

import (
	"os"
	"testing"
	"time"
)

func TestNewProfilerDisabledByDefault(t *testing.T) {
	// Ensure env vars are not set
	os.Unsetenv("LOG_SLOW_QUERIES")
	os.Unsetenv("SLOW_QUERY_THRESHOLD_MS")

	p := NewProfiler()
	if p.IsEnabled() {
		t.Error("Profiler should be disabled by default")
	}
}

func TestNewProfilerEnabledViaEnv(t *testing.T) {
	os.Setenv("LOG_SLOW_QUERIES", "true")
	defer os.Unsetenv("LOG_SLOW_QUERIES")

	p := NewProfiler()
	if !p.IsEnabled() {
		t.Error("Profiler should be enabled when LOG_SLOW_QUERIES=true")
	}
}

func TestNewProfilerEnabledVia1(t *testing.T) {
	os.Setenv("LOG_SLOW_QUERIES", "1")
	defer os.Unsetenv("LOG_SLOW_QUERIES")

	p := NewProfiler()
	if !p.IsEnabled() {
		t.Error("Profiler should be enabled when LOG_SLOW_QUERIES=1")
	}
}

func TestProfilerCustomThreshold(t *testing.T) {
	os.Setenv("LOG_SLOW_QUERIES", "true")
	os.Setenv("SLOW_QUERY_THRESHOLD_MS", "50")
	defer os.Unsetenv("LOG_SLOW_QUERIES")
	defer os.Unsetenv("SLOW_QUERY_THRESHOLD_MS")

	p := NewProfiler()
	if p.slowQueryMs != 50 {
		t.Errorf("Expected threshold 50ms, got %dms", p.slowQueryMs)
	}
}

func TestProfilerInvalidThreshold(t *testing.T) {
	os.Setenv("LOG_SLOW_QUERIES", "true")
	os.Setenv("SLOW_QUERY_THRESHOLD_MS", "invalid")
	defer os.Unsetenv("LOG_SLOW_QUERIES")
	defer os.Unsetenv("SLOW_QUERY_THRESHOLD_MS")

	p := NewProfiler()
	if p.slowQueryMs != defaultSlowQueryThreshold {
		t.Errorf("Expected default threshold %dms for invalid value, got %dms", defaultSlowQueryThreshold, p.slowQueryMs)
	}
}

func TestStartQueryDisabled(t *testing.T) {
	os.Unsetenv("LOG_SLOW_QUERIES")

	p := NewProfiler()
	done := p.StartQuery("SELECT", "users")
	time.Sleep(1 * time.Millisecond)
	done()

	total, slow := p.Stats()
	if total != 0 || slow != 0 {
		t.Errorf("Disabled profiler should not record stats, got total=%d slow=%d", total, slow)
	}
}

func TestStartQueryEnabled(t *testing.T) {
	os.Setenv("LOG_SLOW_QUERIES", "true")
	os.Setenv("SLOW_QUERY_THRESHOLD_MS", "1000") // high threshold so query won't be slow
	defer os.Unsetenv("LOG_SLOW_QUERIES")
	defer os.Unsetenv("SLOW_QUERY_THRESHOLD_MS")

	p := NewProfiler()
	done := p.StartQuery("SELECT", "users")
	time.Sleep(1 * time.Millisecond)
	done()

	total, slow := p.Stats()
	if total != 1 {
		t.Errorf("Expected 1 total query, got %d", total)
	}
	if slow != 0 {
		t.Errorf("Expected 0 slow queries, got %d", slow)
	}
}

func TestSlowQueryDetection(t *testing.T) {
	os.Setenv("LOG_SLOW_QUERIES", "true")
	os.Setenv("SLOW_QUERY_THRESHOLD_MS", "1") // 1ms threshold
	defer os.Unsetenv("LOG_SLOW_QUERIES")
	defer os.Unsetenv("SLOW_QUERY_THRESHOLD_MS")

	p := NewProfiler()
	done := p.StartQuery("SELECT", "videos")
	time.Sleep(10 * time.Millisecond) // exceed threshold
	done()

	total, slow := p.Stats()
	if total != 1 {
		t.Errorf("Expected 1 total query, got %d", total)
	}
	if slow != 1 {
		t.Errorf("Expected 1 slow query, got %d", slow)
	}
}

func TestStartQueryWithRows(t *testing.T) {
	os.Setenv("LOG_SLOW_QUERIES", "true")
	os.Setenv("SLOW_QUERY_THRESHOLD_MS", "1000")
	defer os.Unsetenv("LOG_SLOW_QUERIES")
	defer os.Unsetenv("SLOW_QUERY_THRESHOLD_MS")

	p := NewProfiler()
	done := p.StartQueryWithRows("UPDATE", "users")
	time.Sleep(1 * time.Millisecond)
	done(5, nil)

	total, _ := p.Stats()
	if total != 1 {
		t.Errorf("Expected 1 total query, got %d", total)
	}
}

func TestReset(t *testing.T) {
	os.Setenv("LOG_SLOW_QUERIES", "true")
	defer os.Unsetenv("LOG_SLOW_QUERIES")

	p := NewProfiler()
	done := p.StartQuery("SELECT", "users")
	done()

	total, _ := p.Stats()
	if total != 1 {
		t.Errorf("Expected 1 total query before reset, got %d", total)
	}

	p.Reset()

	total, slow := p.Stats()
	if total != 0 || slow != 0 {
		t.Errorf("Expected 0 stats after reset, got total=%d slow=%d", total, slow)
	}
}

func TestGlobalProfiler(t *testing.T) {
	// The global profiler should exist
	p := GetProfiler()
	if p == nil {
		t.Error("Global profiler should not be nil")
	}
}

func TestProfileConvenienceFunction(t *testing.T) {
	os.Setenv("LOG_SLOW_QUERIES", "true")
	os.Setenv("SLOW_QUERY_THRESHOLD_MS", "1000")
	defer os.Unsetenv("LOG_SLOW_QUERIES")
	defer os.Unsetenv("SLOW_QUERY_THRESHOLD_MS")

	// Reset global profiler
	globalProfiler = NewProfiler()

	// Use convenience function
	done := Profile("SELECT", "sessions")
	time.Sleep(1 * time.Millisecond)
	done()

	total, _ := globalProfiler.Stats()
	if total != 1 {
		t.Errorf("Expected 1 query via convenience function, got %d", total)
	}
}
