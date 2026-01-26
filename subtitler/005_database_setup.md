# 005_database_setup.md

**Implements Task 5: Database setup**

## Objective

Set up SQLite database with tables for users and jobs, define file storage directory structure, and create database migrations that work on fresh install.

## Current State

- Backend is in Go with `internal/storage/` package for file validation
- No database code exists yet
- No file encryption exists yet
- File uploads currently go to `./uploads/` (temporary, not persisted to DB)
- Transcription creates temp files in `./tmp/transcribe/` (cleaned up immediately)

## Design Decisions

### Database Library
Use `modernc.org/sqlite` (pure Go SQLite driver, no CGo required):
- No C compiler dependency for builds
- Cross-platform compatibility
- Used by many production Go projects

### Migration Strategy
Simple SQL migration files with version tracking:
- Migration files: `backend/internal/db/migrations/001_initial_schema.sql`, etc.
- Version table: `schema_migrations` (tracks applied migrations)
- No external migration tool needed (implement in Go)
- Migrations run automatically on server startup

### Database Schema

**users table:**
```sql
CREATE TABLE users (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    email TEXT UNIQUE NOT NULL,
    password_hash TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);
CREATE INDEX idx_users_email ON users(email);
```

**jobs table:**
```sql
CREATE TABLE jobs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id INTEGER NOT NULL,
    status TEXT NOT NULL CHECK(status IN ('pending', 'processing', 'completed', 'failed')),
    original_filename TEXT NOT NULL,
    file_path TEXT NOT NULL,
    file_size INTEGER NOT NULL,
    output_format TEXT NOT NULL CHECK(output_format IN ('srt', 'vtt', 'embedded')),
    transcript_path TEXT,
    error_message TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    completed_at DATETIME,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);
CREATE INDEX idx_jobs_user_id ON jobs(user_id);
CREATE INDEX idx_jobs_status ON jobs(status);
CREATE INDEX idx_jobs_created_at ON jobs(created_at);
```

### File Storage Structure

**Directory layout:**
```
data/
├── db/
│   └── subtitler.db          # SQLite database
└── files/
    ├── uploads/               # User-uploaded files (encrypted with age)
    │   └── {user_id}/
    │       └── {job_id}/
    │           └── {filename}.encrypted
    └── results/               # Transcription results (encrypted with age)
        └── {user_id}/
            └── {job_id}/
                └── {filename}.{srt|vtt|mp4}.encrypted
```

**Encryption:**
- All user files encrypted at rest using `age` encryption
- Encryption key stored separately from environment vars (as required)
- Key file location: `./data/keys/file_encryption.key`
- Key generated automatically on first run if missing
- Implementation deferred to Task 6+ (Task 5 defines structure only)

**Note:** Task 5 only creates the directory structure and documents the encryption plan. Actual encryption will be implemented when user authentication (Task 6) and background jobs (Task 8) are built.

### Configuration

Add to `internal/config/config.go`:
```go
type Config struct {
    // ... existing fields
    DatabasePath string  // Path to SQLite database file
    DataDir      string  // Base directory for all data storage
}
```

Defaults:
- `DATABASE_PATH`: `./data/db/subtitler.db`
- `DATA_DIR`: `./data`

## Implementation Plan

### Step 1: Add SQLite dependency
- Run `go get modernc.org/sqlite`
- Update `go.mod` and `go.sum`

### Step 2: Create database package
- `backend/internal/db/db.go` - Database connection, initialization
- `backend/internal/db/migrate.go` - Migration runner
- `backend/internal/db/migrations/001_initial_schema.sql` - Users and jobs tables

### Step 3: Update config package
- Add `DatabasePath` and `DataDir` to `Config` struct
- Add environment variable support: `DATABASE_PATH`, `DATA_DIR`
- Add defaults to `Load()` function

### Step 4: Initialize database in main.go
- Call database initialization on startup
- Run migrations automatically
- Handle errors gracefully

### Step 5: Create file storage structure
- Create utility functions to generate storage paths
- Document encryption strategy (implementation in future tasks)
- Create directories on first run

## File Changes

### New Files
1. `backend/internal/db/db.go` - Database connection and initialization
2. `backend/internal/db/migrate.go` - Migration runner logic
3. `backend/internal/db/migrations/001_initial_schema.sql` - Initial schema
4. `backend/internal/db/db_test.go` - Database tests
5. `backend/internal/storage/paths.go` - File path generation utilities
6. `backend/internal/storage/paths_test.go` - Path utilities tests

### Modified Files
1. `backend/internal/config/config.go` - Add database configuration
2. `backend/internal/config/config_test.go` - Test new config fields
3. `backend/cmd/server/main.go` - Initialize database on startup
4. `backend/go.mod` - Add SQLite dependency

### Documentation
1. `README.md` - Add database setup instructions
2. `INTEGRATION_TESTS.md` - Add database integration tests
3. `CLAUDE.md` - Document database conventions

## Testing

### Unit Tests
1. **Config tests** (`backend/internal/config/config_test.go`):
   - Verify database path defaults
   - Verify environment variable overrides

2. **Path generation tests** (`backend/internal/storage/paths_test.go`):
   - Test user upload path generation
   - Test result path generation
   - Test path sanitization

3. **Migration tests** (`backend/internal/db/db_test.go`):
   - Create in-memory database
   - Run migrations
   - Verify tables exist
   - Verify indexes exist
   - Test migration idempotency (running twice doesn't error)

### Integration Tests
1. **Database initialization** (in `INTEGRATION_TESTS.md`):
   ```bash
   # Start server and verify database is created
   cd backend && /home/trevor/go/bin/go run ./cmd/server &
   sleep 2
   test -f ./data/db/subtitler.db
   kill %1
   ```

2. **Schema verification**:
   ```bash
   # Verify tables exist after server startup
   cd backend && /home/trevor/go/bin/go run ./cmd/server &
   sleep 2
   sqlite3 ./data/db/subtitler.db ".tables" | grep -q "users"
   sqlite3 ./data/db/subtitler.db ".tables" | grep -q "jobs"
   kill %1
   ```

3. **Directory structure**:
   ```bash
   # Verify directory structure is created
   cd backend && /home/trevor/go/bin/go run ./cmd/server &
   sleep 2
   test -d ./data/files/uploads
   test -d ./data/files/results
   kill %1
   ```

### Manual Testing
1. Start server: `cd backend && /home/trevor/go/bin/go run ./cmd/server`
2. Verify database file created: `ls -la data/db/subtitler.db`
3. Verify schema: `sqlite3 data/db/subtitler.db ".schema"`
4. Verify directories: `ls -la data/files/`

## Acceptance Criteria Verification

Task 5 acceptance criteria:
> SQLite database initialized with tables for users and jobs; file storage directory structure defined; database migrations work on fresh install

Verification steps:
1. ✅ Run server from clean state
2. ✅ Verify `data/db/subtitler.db` exists
3. ✅ Verify `users` table exists with correct schema
4. ✅ Verify `jobs` table exists with correct schema
5. ✅ Verify indexes are created
6. ✅ Verify `data/files/uploads/` and `data/files/results/` directories exist
7. ✅ Run server again and verify no migration errors (idempotency)
8. ✅ All unit tests pass
9. ✅ All integration tests pass

## Dependencies

- Task 1 (Initialize project structure) ✅ Complete
- No blockers

## Notes

- Encryption implementation deferred to when files are actually persisted (Task 6+)
- This task sets up the foundation; actual usage will be in Task 6 (auth) and Task 8 (jobs)
- Database path should be configurable for testing (in-memory, temporary paths)
- Migration system is simple but sufficient for current needs
- If migration needs grow, consider tools like `goose` or `migrate` later

## Future Considerations

- Database backups (after deployment, Task 12)
- Database size monitoring (after analytics, Task 16)
- Migration rollback support (if needed)
- Connection pooling (SQLite default is single-writer, multiple-reader)
- WAL mode for better concurrency (can enable later if needed)
