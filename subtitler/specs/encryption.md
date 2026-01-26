# File Encryption Specification

This document describes the file-at-rest encryption implementation for uploaded media files.

## Overview

All uploaded video/audio files are encrypted at rest using the `age` encryption tool. Files are decrypted on-demand for processing (transcription, serving, burning) and re-encrypted after operations that produce new files.

## Library

- **Package:** `filippo.io/age` v1.2.1
- **Algorithm:** X25519 key exchange + ChaCha20-Poly1305 authenticated encryption
- **Key format:** X25519 identity (public starts with `age1`, private with `AGE-SECRET-KEY-`)

## Files

| File | Purpose |
|------|---------|
| `backend/crypto/crypto.go` | Encryptor type and methods |
| `backend/crypto/crypto_test.go` | Unit tests |
| `backend/main.go` | Integration with upload/serve/burn flows |

## Encryptor Type

```go
type Encryptor struct {
    identity  *age.X25519Identity  // Private key
    recipient *age.X25519Recipient // Public key
    mu        sync.RWMutex         // Thread safety
}
```

### Methods

| Method | Purpose |
|--------|---------|
| `NewEncryptor(privateKey string)` | Create from key or generate new |
| `LoadOrGenerateKey(keyPath string)` | Load from file or generate & save |
| `PublicKey() string` | Get public key |
| `PrivateKey() string` | Get private key |
| `EncryptFile(srcPath string)` | Encrypt file, returns `.age` path |
| `DecryptFile(encPath string)` | Decrypt file to bytes |
| `DecryptToFile(encPath, dstPath string)` | Decrypt to destination |
| `DecryptToTempFile(encPath string)` | Decrypt to temp file (preserves extension) |
| `EncryptReader(io.Reader)` | Stream encryption |
| `DecryptReader(io.Reader)` | Stream decryption |
| `EncryptBytes([]byte)` | Encrypt bytes |
| `DecryptBytes([]byte)` | Decrypt bytes |

## Key Management

### Storage

- **Location:** `data/age.key` (relative to backend working directory)
- **Permissions:** `0600` (owner read/write only)

### File Format

```
# age secret key - DO NOT SHARE
# public key: age1xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
AGE-SECRET-KEY-1xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
```

### Initialization

On server startup, `LoadOrGenerateKey("data/age.key")` is called:
1. If key file exists: parse and load
2. If key file missing: generate new X25519 identity and save
3. Returns `(encryptor, wasNewlyGenerated, error)`

## Encryption Flow

### Upload

```
User uploads video → Save to disk → EncryptFile() → Delete original
```

1. File received via `/api/upload`
2. Saved to `uploads/{id}.{ext}`
3. `encryptor.EncryptFile()` called
4. Creates `uploads/{id}.{ext}.age`
5. Original unencrypted file deleted
6. Database stores path with `.age` extension

### Transcription

```
Read encrypted file → DecryptToTempFile() → Process with whisper → Delete temp
```

1. Check if `FilePath` ends with `.age`
2. Call `encryptor.DecryptToTempFile()`
3. Process with whisper-cli
4. Cleanup: `defer os.Remove(tempPath)`

### Video Serving

```
Read encrypted file → DecryptToTempFile() → Stream to client → Delete temp
```

1. Check if encrypted (`.age` extension)
2. Decrypt to temp file
3. Serve via `http.ServeFile()`
4. Cleanup temp file

### Subtitle Burning

```
Decrypt video → ffmpeg burns subtitles → Encrypt output → Delete temps
```

1. Decrypt source video to temp
2. Run ffmpeg with subtitle overlay
3. Encrypt output file
4. Delete temporary files
5. Store encrypted output path in `burn_jobs.output_path`

## Database Integration

### Videos Table

```sql
file_path TEXT NOT NULL  -- Stores path with .age extension
```

Example: `/absolute/path/uploads/abc123.mp4.age`

### BurnJobs Table

```sql
output_path TEXT  -- Stores encrypted burned video path
```

Example: `/absolute/path/uploads/abc123_burned.mp4.age`

## Configuration

| Setting | Default | Notes |
|---------|---------|-------|
| Key path | `data/age.key` | Configurable via `KEY_PATH` env var |
| Max file size | 500 MB | Configurable via `MAX_UPLOAD_SIZE` env var |
| File extension | `.age` | Appended to encrypted files |
| Encryption toggle | enabled | Configurable via `ENCRYPTION_ENABLED` env var |

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `KEY_PATH` | `data/age.key` | Path to the age encryption key file |
| `ENCRYPTION_ENABLED` | `true` | Set to `false` or `0` to disable encryption |

### Disabling Encryption

Setting `ENCRYPTION_ENABLED=false` affects only NEW file uploads:
- New files are stored without encryption (no `.age` extension)
- Existing encrypted files continue to work (they're decrypted on access)
- This setting is intended for development/testing only

**Warning:** Do not disable encryption in production. Uploaded videos may contain sensitive content.

## Error Handling

| Scenario | Behavior |
|----------|----------|
| Missing key file | Generate new key |
| Corrupt key file | Server fails to start |
| Decryption with wrong key | Returns error (AEAD tag mismatch) |
| File not found | Returns file open error |
| Partial encryption failure | Cleans up incomplete `.age` file |

## Security Characteristics

### Strengths

- Modern `age` library by cryptography expert (Filippo Valsorda)
- ChaCha20-Poly1305 provides confidentiality + authenticity
- X25519 key exchange resistant to known attacks
- Key file has restricted permissions (0600)
- Thread-safe via `sync.RWMutex`
- Temporary decrypted files cleaned up

### Considerations

- Single key for all files (no per-file key derivation)
- No key rotation mechanism
- Decrypted files briefly exist on disk during processing
- Database metadata not encrypted (transcriptions, user info)

## Testing

Tests in `backend/crypto/crypto_test.go`:

- `TestNewEncryptor` - Generate new key
- `TestNewEncryptorWithKey` - Parse existing key
- `TestNewEncryptorInvalidKey` - Reject malformed keys
- `TestEncryptDecryptBytes` - Round-trip bytes
- `TestEncryptDecryptFile` - Round-trip files
- `TestEncryptDecryptLargeData` - 1MB data test
- `TestDecryptToTempFile` - Extension preservation
- `TestDecryptWithWrongKey` - Security: wrong key fails
- `TestLoadOrGenerateKey` - Key persistence

Run tests:
```bash
cd backend && go test ./crypto -v
```

## Related Specs

- [auth.md](auth.md) - Authentication (separate from encryption)
- [totp.md](totp.md) - 2FA (uses different crypto primitives)
