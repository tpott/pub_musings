package db

import "fmt"

// Vacuum runs SQLite VACUUM to reclaim disk space and defragment the database.
// This should be run periodically (e.g., daily) during low-usage periods.
// Note: VACUUM requires exclusive access and may take time for large databases.
func (db *DB) Vacuum() error {
	ctx, cancel := db.queryContext()
	defer cancel()

	_, err := db.conn.ExecContext(ctx, "VACUUM")
	return err
}

// Analyze runs SQLite ANALYZE to update query planner statistics.
// This should be run after significant data changes to improve query performance.
func (db *DB) Analyze() error {
	ctx, cancel := db.queryContext()
	defer cancel()

	_, err := db.conn.ExecContext(ctx, "ANALYZE")
	return err
}

// Maintenance runs both VACUUM and ANALYZE operations.
// Returns error from the first operation that fails, or nil if both succeed.
func (db *DB) Maintenance() error {
	if err := db.Vacuum(); err != nil {
		return fmt.Errorf("vacuum failed: %w", err)
	}
	if err := db.Analyze(); err != nil {
		return fmt.Errorf("analyze failed: %w", err)
	}
	return nil
}
