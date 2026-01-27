# Encryption Key Rotation

This document describes the encryption key rotation feature for the subtitler project.

## Overview

Key rotation allows administrators to securely transition from an old encryption key to a new one. This is important for:

1. **Security hygiene** - Periodically rotating keys limits exposure if a key is compromised
2. **Key compromise response** - Allows recovery after a suspected key leak
3. **Compliance** - Some security standards require key rotation

## Design

### Key Storage

Keys are stored in a keys directory with version numbers:

```
data/
  keys/
    key_v1.age       # Original key (version 1)
    key_v2.age       # Rotated key (version 2)
    current_version  # File containing current version number (e.g., "2")
```

The `current_version` file contains just the version number as a string (e.g., "2").

### Database Schema

A new column tracks which key version encrypted each file:

```sql
ALTER TABLE videos ADD COLUMN key_version INTEGER NOT NULL DEFAULT 1;
```

Burn jobs output files also need key version tracking:

```sql
ALTER TABLE burn_jobs ADD COLUMN output_key_version INTEGER;
```

### MultiKeyEncryptor

A new `MultiKeyEncryptor` type wraps multiple `Encryptor` instances:

```go
type MultiKeyEncryptor struct {
    keys       map[int]*Encryptor  // version -> encryptor
    currentVer int                 // current key version for new files
    mu         sync.RWMutex
}
```

#### Methods

| Method | Purpose |
|--------|---------|
| `LoadKeys(keysDir string)` | Load all keys from directory, read current version |
| `EncryptFile(path string) (string, int, error)` | Encrypt with current key, return path and version |
| `DecryptFile(path string, version int) ([]byte, error)` | Decrypt using specified key version |
| `Rotate() (int, error)` | Generate new key, return new version number |
| `GetCurrentVersion() int` | Get current key version |
| `ReencryptFile(path string, oldVersion int) (string, int, error)` | Re-encrypt file with current key |

### Key Rotation Procedure

#### Phase 1: Key Generation

1. Generate new age key
2. Save to `data/keys/key_vN.age` where N is next version
3. Update `current_version` file to N
4. New uploads now use version N

Files encrypted with old keys continue to work - the system keeps all keys.

#### Phase 2: Re-encryption (Optional)

Run during idle periods to re-encrypt old files with new key:

```bash
go run ./cmd/rotate-keys [--batch-size=100] [--dry-run]
```

This:
1. Queries videos with `key_version < current_version`
2. For each file:
   - Decrypt with old key
   - Re-encrypt with current key
   - Update database with new version
   - Delete old encrypted file
3. Repeat until all files are re-encrypted

### CLI Tool

`backend/cmd/rotate-keys/main.go`:

```go
Usage:
  rotate-keys rotate              Generate new key and start using it
  rotate-keys status              Show rotation status (files per version)
  rotate-keys reencrypt           Re-encrypt files with old keys
  rotate-keys reencrypt --batch=N Re-encrypt N files at a time
  rotate-keys reencrypt --dry-run Show what would be re-encrypted
```

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `KEYS_DIR` | `data/keys` | Directory containing versioned keys |
| `ROTATION_BATCH_SIZE` | `100` | Files to re-encrypt per batch |

### Backward Compatibility

- Existing `KEY_PATH` environment variable continues to work
- If `KEY_PATH` is set and `KEYS_DIR` not set, legacy single-key mode is used
- If only legacy key exists, it becomes version 1 automatically

### Migration from Single Key

When upgrading from single-key to versioned keys:

1. If `data/age.key` exists and `data/keys/` does not:
   - Create `data/keys/` directory
   - Copy `data/age.key` to `data/keys/key_v1.age`
   - Create `data/keys/current_version` with "1"
2. All existing videos get `key_version = 1`

### Security Considerations

1. **Old keys must be kept** - Until all files are re-encrypted
2. **Atomic operations** - Re-encryption uses temp files to prevent data loss
3. **Permission on keys** - All key files have 0600 permissions
4. **No downgrade** - Once rotated, the version only increases

### Error Handling

| Scenario | Behavior |
|----------|----------|
| Key version not found | Return error (don't fail silently) |
| Re-encryption fails mid-file | Keep original, log error, continue |
| Missing key for a file | Log error, skip file (admin must restore key) |

## Files

| File | Purpose |
|------|---------|
| `backend/crypto/multi.go` | MultiKeyEncryptor implementation |
| `backend/crypto/multi_test.go` | Tests |
| `backend/cmd/rotate-keys/main.go` | CLI tool |
| `backend/db/migrations/004_add_key_version.up.sql` | Schema migration |
| `backend/db/migrations/004_add_key_version.down.sql` | Rollback migration |

## Testing

Tests verify:
- Loading multiple keys
- Encrypting with current version
- Decrypting with any version
- Re-encryption preserves content
- Version upgrade path
- Legacy migration
