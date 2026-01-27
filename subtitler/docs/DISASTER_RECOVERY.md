# Disaster Recovery Procedures

This document describes how to recover from various failure scenarios in the Subtitler application.

## Quick Reference

| Scenario | Data Loss Risk | Recovery Time | See Section |
|----------|---------------|---------------|-------------|
| Server failure | Low | 1-4 hours | [Server Failure](#server-failure) |
| Database corruption | Low-Medium | 30 min - 2 hours | [Database Corruption](#database-corruption) |
| Encryption key loss | **Critical** | Unrecoverable | [Encryption Key Loss](#encryption-key-loss) |
| Whisper server failure | None | 5-15 minutes | [Whisper Server Failure](#whisper-server-failure) |

## Critical Files

These files must be backed up and protected:

| File | Location | Purpose | Recovery if Lost |
|------|----------|---------|------------------|
| `age.key` or `keys/` | `data/age.key` or `data/keys/` | Encryption keys | **Unrecoverable** - all encrypted files lost |
| `subtitler.db` | `data/subtitler.db` | SQLite database | Restore from backup |
| `uploads/` | `uploads/` | Encrypted video files | Restore from backup |
| CSRF secret | `data/csrf.key` | CSRF protection | Regenerates (sessions invalidated) |

---

## Encryption Key Loss

### Severity: CRITICAL - UNRECOVERABLE

**The encryption key is the most critical piece of data.** If lost without backup:
- All encrypted video files become permanently unreadable
- There is NO recovery method - age encryption uses X25519 which cannot be brute-forced
- User accounts and metadata remain accessible but their videos are lost

### Prevention

1. **Backup immediately after generation:**
   ```bash
   # After first server start
   cp /opt/subtitler/data/age.key /secure/backup/location/
   # Or for versioned keys:
   cp -r /opt/subtitler/data/keys/ /secure/backup/location/
   ```

2. **Store backups in multiple locations:**
   - Encrypted cloud storage (e.g., Backblaze B2, AWS S3 with encryption)
   - Physical offline storage (USB drive in safe)
   - Password manager secure note

3. **Verify backup regularly:**
   ```bash
   # Compare key fingerprints
   diff /opt/subtitler/data/age.key /secure/backup/age.key
   ```

4. **Use key rotation** (see [key-rotation.md](../specs/key-rotation.md)):
   - Keep old keys in `data/keys/` directory
   - Each version adds redundancy

### If Key is Lost

1. **Accept data loss** - Encrypted files cannot be recovered
2. **Generate new key:**
   ```bash
   cd /opt/subtitler/backend
   rm data/age.key  # Remove corrupted/partial key
   ./subtitler      # Will generate new key on startup
   ```
3. **Clear orphaned data:**
   ```bash
   # Remove encrypted files that can no longer be decrypted
   rm -rf /opt/subtitler/uploads/*.age

   # Clear database references to lost files
   sqlite3 /opt/subtitler/data/subtitler.db "DELETE FROM videos;"
   sqlite3 /opt/subtitler/data/subtitler.db "DELETE FROM transcriptions;"
   sqlite3 /opt/subtitler/data/subtitler.db "DELETE FROM burn_jobs;"
   ```
4. **Notify affected users** that their videos must be re-uploaded

---

## Database Corruption

### Symptoms

- Application errors: "database disk image is malformed"
- Backend refuses to start with SQLite errors
- Data inconsistencies (missing records, partial writes)

### Diagnosis

```bash
# Check database integrity
sqlite3 /opt/subtitler/data/subtitler.db "PRAGMA integrity_check;"

# Expected output: "ok"
# If corrupted: "database disk image is malformed" or specific errors
```

### Recovery from Backup

1. **Stop the backend:**
   ```bash
   sudo systemctl stop subtitler
   ```

2. **Preserve corrupted database for analysis:**
   ```bash
   mv /opt/subtitler/data/subtitler.db /opt/subtitler/data/subtitler.db.corrupted
   mv /opt/subtitler/data/subtitler.db-wal /opt/subtitler/data/subtitler.db-wal.corrupted
   mv /opt/subtitler/data/subtitler.db-shm /opt/subtitler/data/subtitler.db-shm.corrupted
   ```

3. **Restore from backup:**
   ```bash
   cp /backup/subtitler-latest.db /opt/subtitler/data/subtitler.db
   ```

4. **Verify restoration:**
   ```bash
   sqlite3 /opt/subtitler/data/subtitler.db "PRAGMA integrity_check;"
   sqlite3 /opt/subtitler/data/subtitler.db "SELECT COUNT(*) FROM videos;"
   ```

5. **Restart backend:**
   ```bash
   sudo systemctl start subtitler
   sudo systemctl status subtitler
   ```

### Recovery Without Backup (Partial)

If no backup exists, attempt repair:

```bash
# Attempt to dump recoverable data
sqlite3 /opt/subtitler/data/subtitler.db.corrupted ".dump" > recovered.sql

# Create new database from dump
sqlite3 /opt/subtitler/data/subtitler.db < recovered.sql

# Run migrations to ensure schema is current
cd /opt/subtitler/backend
./subtitler migrate up
```

### Prevention

1. **Regular backups:**
   ```bash
   # Add to crontab
   0 */6 * * * sqlite3 /opt/subtitler/data/subtitler.db ".backup /backup/subtitler-$(date +\%Y\%m\%d-\%H\%M).db"
   ```

2. **Keep multiple backup generations:**
   ```bash
   # Rotate backups (keep last 7 days)
   find /backup -name "subtitler-*.db" -mtime +7 -delete
   ```

3. **Use WAL mode** (enabled by default):
   ```sql
   PRAGMA journal_mode = WAL;
   ```

---

## Server Failure

### Complete VM Failure

1. **Recreate VM:**
   ```bash
   # On Mac Mini host
   qemu-img create -f qcow2 subtitler.qcow2 50G
   # Install Ubuntu Server, configure networking
   ```

2. **Restore application files:**
   ```bash
   # Copy backups to new VM
   scp /backup/subtitler-backend.tar.gz vm:/tmp/
   scp /backup/subtitler-data.tar.gz vm:/tmp/
   scp /backup/subtitler-frontend.tar.gz vm:/tmp/

   # On VM
   tar -xzf /tmp/subtitler-backend.tar.gz -C /opt/subtitler/
   tar -xzf /tmp/subtitler-data.tar.gz -C /opt/subtitler/
   tar -xzf /tmp/subtitler-frontend.tar.gz -C /var/www/
   ```

3. **Restore configuration:**
   ```bash
   # Caddy config
   sudo cp /backup/Caddyfile /etc/caddy/Caddyfile
   sudo systemctl restart caddy

   # Cloudflared tunnel config
   cp /backup/cloudflared-config.yml ~/.cloudflared/config.yml
   # Re-authenticate if needed: cloudflared tunnel login
   sudo systemctl restart cloudflared

   # Subtitler service
   sudo cp /backup/subtitler.service /etc/systemd/system/
   sudo systemctl daemon-reload
   sudo systemctl start subtitler
   ```

### Host System Failure (Mac Mini)

1. **Replace hardware or reinstall macOS**
2. **Restore whisper-server:**
   ```bash
   # Clone whisper.cpp
   git clone https://github.com/ggerganov/whisper.cpp
   cd whisper.cpp && make

   # Download model
   ./models/download-ggml-model.sh large-v3-turbo

   # Restore launch script
   cp /backup/start-whisper-server.sh ~/bin/

   # Restore launchd plist
   cp /backup/com.user.whisper-server.plist ~/Library/LaunchAgents/
   launchctl load ~/Library/LaunchAgents/com.user.whisper-server.plist
   ```

3. **Start VM:**
   ```bash
   # Restore VM disk image
   cp /backup/subtitler.qcow2 ~/VMs/

   # Start VM (use your normal startup script)
   ```

---

## Whisper Server Failure

### Symptoms

- Transcription jobs stuck at "processing" status
- Health check shows `whisper_available: false`
- Backend logs: "whisper-server connection refused"

### Diagnosis

```bash
# On Mac Mini host
pgrep -a whisper-server
curl http://localhost:8765/health

# From VM
curl http://10.0.2.2:8765/health
```

### Recovery

1. **Check whisper-server logs:**
   ```bash
   tail -100 ~/Library/Logs/whisper-server.log
   tail -100 ~/Library/Logs/whisper-server.error.log
   ```

2. **Restart whisper-server:**
   ```bash
   launchctl stop com.user.whisper-server
   launchctl start com.user.whisper-server
   ```

3. **If launchd fails, start manually:**
   ```bash
   ~/bin/start-whisper-server.sh
   ```

4. **Verify recovery:**
   ```bash
   curl http://localhost:8765/health
   # Expected: {"status":"ok"}
   ```

5. **Reprocess failed transcriptions:**
   - Users can click "Reprocess" on failed videos
   - Or via API: `POST /api/videos/{id}/reprocess`

---

## Backup Verification

Backups are only useful if they can be restored. Verify regularly:

### Weekly Verification Checklist

1. **Database integrity:**
   ```bash
   sqlite3 /backup/subtitler-latest.db "PRAGMA integrity_check;"
   sqlite3 /backup/subtitler-latest.db "SELECT COUNT(*) FROM videos;"
   sqlite3 /backup/subtitler-latest.db "SELECT COUNT(*) FROM users;"
   ```

2. **Encryption key validity:**
   ```bash
   # Test decryption with backed-up key
   cd /tmp
   cp /backup/age.key test-key
   # Create test encrypted file
   echo "test" | age -r $(cat test-key | grep "public key:" | cut -d: -f2 | tr -d ' ') > test.age
   # Decrypt with backed-up key
   age -d -i test-key test.age
   # Expected output: "test"
   rm test-key test.age
   ```

3. **Uploads directory:**
   ```bash
   # Count files
   ls /backup/uploads/ | wc -l
   # Compare with production
   ssh vm 'ls /opt/subtitler/uploads/ | wc -l'
   ```

### Monthly Restoration Test

Perform a full restoration to a test environment:

1. Create a test VM
2. Restore all backups
3. Start all services
4. Verify:
   - Login works
   - Existing videos play
   - New uploads work
   - Transcription works

---

## Backup Script

Create `/opt/subtitler/scripts/backup.sh`:

```bash
#!/bin/bash
set -e

BACKUP_DIR="/backup/subtitler"
DATE=$(date +%Y%m%d-%H%M%S)

# Stop backend for consistent backup
sudo systemctl stop subtitler

# Backup database
sqlite3 /opt/subtitler/data/subtitler.db ".backup $BACKUP_DIR/db/subtitler-$DATE.db"

# Backup encryption keys
cp -r /opt/subtitler/data/keys/ $BACKUP_DIR/keys-$DATE/
# Or for single key:
cp /opt/subtitler/data/age.key $BACKUP_DIR/age-$DATE.key

# Backup uploads (incremental)
rsync -av --link-dest=$BACKUP_DIR/uploads-latest/ \
  /opt/subtitler/uploads/ \
  $BACKUP_DIR/uploads-$DATE/

# Update latest symlinks
ln -sfn $BACKUP_DIR/db/subtitler-$DATE.db $BACKUP_DIR/subtitler-latest.db
ln -sfn $BACKUP_DIR/uploads-$DATE $BACKUP_DIR/uploads-latest

# Restart backend
sudo systemctl start subtitler

# Cleanup old backups (keep 7 days)
find $BACKUP_DIR/db -name "*.db" -mtime +7 -delete
find $BACKUP_DIR -maxdepth 1 -name "uploads-*" -type d -mtime +7 -exec rm -rf {} \;

echo "Backup completed: $DATE"
```

Add to crontab:
```bash
# Daily at 3 AM
0 3 * * * /opt/subtitler/scripts/backup.sh >> /var/log/subtitler-backup.log 2>&1
```

---

## Emergency Contacts

Document your recovery contacts:

| Role | Contact | Access |
|------|---------|--------|
| System Admin | [your contact] | Full server access |
| Backup Manager | [contact] | Backup storage access |
| Cloudflare Admin | [contact] | DNS/tunnel management |

---

## See Also

- [specs/deployment.md](../specs/deployment.md) - Full deployment architecture
- [specs/encryption.md](../specs/encryption.md) - Encryption implementation details
- [specs/key-rotation.md](../specs/key-rotation.md) - Key rotation procedures
- [docs/ENV.md](ENV.md) - Environment variable reference
