# Production Security Hardening Checklist

Pre-deployment checklist for securing a production Subtitler instance.

## Quick Assessment

| Category | Status | Priority |
|----------|--------|----------|
| [Server Hardening](#1-server-hardening) | [ ] | Critical |
| [Environment Variables](#2-environment-variable-security) | [ ] | Critical |
| [Database Security](#3-database-security) | [ ] | Critical |
| [File Security](#4-file-and-encryption-security) | [ ] | Critical |
| [Network Security](#5-network-security) | [ ] | High |
| [Rate Limiting](#6-rate-limiting-verification) | [ ] | High |
| [Monitoring](#7-monitoring-and-logging) | [ ] | High |
| [Backup Procedures](#8-backup-and-recovery) | [ ] | High |
| [Access Control](#9-access-control) | [ ] | Medium |
| [Dependency Security](#10-dependency-security) | [ ] | Medium |

---

## 1. Server Hardening

### Operating System

- [ ] **Update system packages**
  ```bash
  sudo apt update && sudo apt upgrade -y
  ```

- [ ] **Enable automatic security updates**
  ```bash
  sudo apt install unattended-upgrades
  sudo dpkg-reconfigure -plow unattended-upgrades
  ```

- [ ] **Disable root SSH login**
  ```bash
  # /etc/ssh/sshd_config
  PermitRootLogin no
  PasswordAuthentication no  # Use SSH keys only
  ```

- [ ] **Configure fail2ban for SSH protection**
  ```bash
  sudo apt install fail2ban
  sudo systemctl enable fail2ban
  ```

### systemd Service Hardening

The backend service file should include these security directives:

- [ ] **Verify service hardening in `/etc/systemd/system/subtitler.service`:**
  ```ini
  # Prevent privilege escalation
  NoNewPrivileges=true

  # Filesystem restrictions
  ProtectSystem=strict
  ProtectHome=true
  ReadWritePaths=/opt/subtitler/data /opt/subtitler/uploads

  # Kernel restrictions
  ProtectKernelTunables=true
  ProtectKernelModules=true
  ProtectKernelLogs=true

  # Network restrictions (optional - if backend only needs localhost)
  # RestrictAddressFamilies=AF_INET AF_INET6

  # Memory execution protection
  MemoryDenyWriteExecute=true
  ```

- [ ] **Run as dedicated non-root user**
  ```bash
  # Verify service user exists and has no shell
  id subtitler
  # Expected: subtitler user with /bin/false or /usr/sbin/nologin shell
  ```

---

## 2. Environment Variable Security

### Critical Production Settings

- [ ] **HTTPS_ONLY=true** - Enables secure cookies
  ```bash
  grep "HTTPS_ONLY=true" /etc/systemd/system/subtitler.service
  ```

- [ ] **LOG_VERBOSE=false** (or unset) - Prevents leaking internal errors
  ```bash
  # Should NOT be set in production
  grep -v "LOG_VERBOSE" /etc/systemd/system/subtitler.service || echo "Not set (good)"
  ```

- [ ] **TRUST_PROXY=true** - Only if behind a reverse proxy
  ```bash
  # Only set this if you're behind Caddy/nginx/Cloudflare
  # Allows trusting X-Forwarded-For headers for rate limiting
  ```

### Secrets Management

- [ ] **CSRF_SECRET or CSRF_SECRET_PATH set**
  ```bash
  # Either set CSRF_SECRET directly (not recommended) or let it auto-generate
  # Auto-generated secrets are stored in data/csrf.key
  ls -la /opt/subtitler/data/csrf.key
  ```

- [ ] **RESEND_API_KEY secured** (if email enabled)
  ```bash
  # Should be set via environment file, not directly in service file
  # /opt/subtitler/.env (mode 0600)
  ```

- [ ] **CAPTCHA keys configured** (if CAPTCHA enabled)
  ```bash
  # Both CAPTCHA_SITE_KEY and CAPTCHA_SECRET_KEY required
  ```

- [ ] **METRICS_API_KEY set** (if metrics endpoint needed)
  ```bash
  # Strong random key for /metrics endpoint access
  # Generate with: openssl rand -hex 32
  ```

### Environment File Permissions

- [ ] **Environment file has restricted permissions**
  ```bash
  ls -la /opt/subtitler/.env
  # Expected: -rw------- (0600) owned by subtitler user
  ```

---

## 3. Database Security

### File Permissions

- [ ] **Database file has restricted permissions**
  ```bash
  ls -la /opt/subtitler/data/subtitler.db
  # Expected: -rw------- (0600) or -rw-r----- (0640) owned by subtitler
  ```

- [ ] **Database directory protected**
  ```bash
  ls -la /opt/subtitler/data/
  # Expected: drwx------ (0700) owned by subtitler
  ```

### Database Integrity

- [ ] **WAL mode enabled** (default, improves crash recovery)
  ```bash
  sqlite3 /opt/subtitler/data/subtitler.db "PRAGMA journal_mode;"
  # Expected: wal
  ```

- [ ] **Integrity check passes**
  ```bash
  sqlite3 /opt/subtitler/data/subtitler.db "PRAGMA integrity_check;"
  # Expected: ok
  ```

- [ ] **Required indices exist**
  ```bash
  sqlite3 /opt/subtitler/data/subtitler.db ".indices"
  # Should include: idx_videos_user_id, idx_videos_session_id, idx_sessions_user_id, etc.
  ```

### Maintenance Scheduler

- [ ] **DB_MAINTENANCE_INTERVAL configured**
  ```bash
  # Default: 24h - runs VACUUM and ANALYZE
  grep "DB_MAINTENANCE_INTERVAL" /etc/systemd/system/subtitler.service
  ```

---

## 4. File and Encryption Security

### Encryption Key Protection

- [ ] **Encryption enabled**
  ```bash
  # ENCRYPTION_ENABLED=false only for debugging, never production
  grep -v "ENCRYPTION_ENABLED=false" /etc/systemd/system/subtitler.service || echo "Check manually"
  ```

- [ ] **Key file has 0600 permissions**
  ```bash
  ls -la /opt/subtitler/data/age.key
  # Expected: -rw------- (0600) owned by subtitler
  ```

- [ ] **Key backup exists in secure location**
  ```bash
  # Verify backup exists (location depends on your setup)
  # See docs/DISASTER_RECOVERY.md for backup procedures
  ```

- [ ] **Multi-key rotation configured** (if using key rotation)
  ```bash
  ls -la /opt/subtitler/data/keys/
  # Check for key_v1.age, key_v2.age, etc.
  ```

### Upload Directory

- [ ] **Upload directory has restricted permissions**
  ```bash
  ls -la /opt/subtitler/uploads/
  # Expected: drwx------ (0700) owned by subtitler
  ```

- [ ] **All uploaded files are encrypted**
  ```bash
  ls /opt/subtitler/uploads/ | head -5
  # All files should end in .age
  ```

### Path Validation

The backend includes path validation to prevent directory traversal attacks. This is tested automatically, but verify:

- [ ] **Path validation tests pass**
  ```bash
  cd /opt/subtitler/backend && go test ./pathvalidator/... -v
  ```

---

## 5. Network Security

### Firewall Configuration

- [ ] **Only required ports exposed**
  ```bash
  sudo ufw status
  # Expected: 22/tcp (SSH), 80/tcp, 443/tcp
  # Backend port (8060) should NOT be exposed externally
  ```

- [ ] **Backend not directly accessible**
  ```bash
  # From external network, this should fail:
  curl http://your-server:8060/api/health
  # Only Caddy/tunnel should reach the backend
  ```

- [ ] **Whisper-server restricted to VM**
  ```bash
  # On host: whisper-server should only accept connections from VM
  sudo ufw allow from 10.0.2.0/24 to any port 8765
  # Deny from other sources
  ```

### Cloudflare Tunnel (if used)

- [ ] **Tunnel configuration reviewed**
  ```bash
  cat ~/.cloudflared/config.yml
  # Verify hostname matches your domain
  # Verify service points to localhost:80
  ```

- [ ] **Tunnel credentials secured**
  ```bash
  ls -la ~/.cloudflared/*.json
  # Should have restricted permissions
  ```

### TLS/HTTPS

- [ ] **HTTPS enforced** (via Cloudflare or Caddy)
  ```bash
  curl -I http://your-domain.com
  # Should redirect to HTTPS or refuse connection
  ```

- [ ] **HSTS header present** (when HTTPS_ONLY=true)
  ```bash
  curl -I https://your-domain.com | grep -i strict-transport
  # Expected: Strict-Transport-Security: max-age=31536000; includeSubDomains
  ```

---

## 6. Rate Limiting Verification

### Endpoint Protection

All rate limits are applied automatically. Verify they're working:

- [ ] **Auth endpoints rate limited** (5/min per IP)
  ```bash
  for i in {1..6}; do
    curl -s -o /dev/null -w "%{http_code}\n" -X POST https://your-domain.com/api/auth/login
  done
  # 6th request should return 429
  ```

- [ ] **Upload endpoint rate limited** (10/min per IP)
- [ ] **Transcription endpoint rate limited** (5/min per IP)
- [ ] **Download endpoints rate limited** (30/min per IP)
- [ ] **Metrics endpoint rate limited** (10/min per IP)

### Per-Email Lockout

- [ ] **Account lockout working** (5 failed logins = 15 min lock)
  ```bash
  # Test by sending 5 failed logins for same email
  # 6th attempt should get "Account temporarily locked"
  ```

### Configuration Verification

- [ ] **Rate limit environment variables reviewed** (if customized)
  ```bash
  # See docs/RATE_LIMITS.md for configurable limits
  # AUTH_RATE_LIMIT, UPLOAD_RATE_LIMIT, etc.
  ```

---

## 7. Monitoring and Logging

### Log Configuration

- [ ] **Structured logging enabled**
  ```bash
  # Backend uses slog with structured output
  journalctl -u subtitler | head -5
  # Should show JSON-formatted logs with fields
  ```

- [ ] **Log level appropriate for production**
  ```bash
  # LOG_LEVEL=info (default) or LOG_LEVEL=warn
  # Never use LOG_LEVEL=debug in production
  ```

- [ ] **Caddy access logs enabled**
  ```bash
  ls -la /var/log/caddy/
  # Should contain access log files
  ```

### Health Monitoring

- [ ] **Health check endpoint working**
  ```bash
  curl https://your-domain.com/api/health
  # For authenticated users, shows full status
  # For public requests, shows only status: ok/degraded
  ```

- [ ] **External monitoring configured** (recommended)
  - Set up uptime monitoring (e.g., UptimeRobot, Pingdom)
  - Alert on health check failures
  - Monitor response times

### Security Event Logging

The backend logs authentication events:
- Login attempts (success/failure)
- 2FA verification attempts
- Password reset requests
- File access events (when audit logging enabled)

- [ ] **Log retention policy configured**
  ```bash
  # journalctl vacuum to limit log size
  sudo journalctl --vacuum-size=500M
  ```

---

## 8. Backup and Recovery

### Backup Schedule

- [ ] **Automated backups configured**
  ```bash
  crontab -l | grep backup
  # Should show daily backup script
  ```

- [ ] **Backup script tested**
  ```bash
  /opt/subtitler/scripts/backup.sh
  # Verify backup files created
  ls -la /backup/subtitler/
  ```

### Backup Contents

- [ ] **Database backed up**
- [ ] **Encryption keys backed up** (CRITICAL)
- [ ] **Uploads backed up** (if required)
- [ ] **Configuration backed up**

### Recovery Testing

- [ ] **Monthly restoration test completed**
  - See [docs/DISASTER_RECOVERY.md](DISASTER_RECOVERY.md) for procedures
  - Test on isolated environment
  - Verify login, video playback, upload functionality

### Key Backup Verification

- [ ] **Key backup tested**
  ```bash
  # From backup location, verify key can decrypt:
  echo "test" | age -e -r "$(cat /backup/age.key | grep 'public key:' | cut -d: -f2)" | \
    age -d -i /backup/age.key
  # Should output: test
  ```

---

## 9. Access Control

### Authentication Configuration

- [ ] **Email verification enabled**
  ```bash
  # New users must verify email before logging in
  # Enabled by default when EMAIL_ENABLED=true
  ```

- [ ] **CAPTCHA enabled** (recommended for public instances)
  ```bash
  # CAPTCHA_SITE_KEY and CAPTCHA_SECRET_KEY both set
  ```

- [ ] **Password complexity requirements active**
  - Minimum 8 characters
  - At least 1 uppercase, 1 lowercase, 1 number, 1 special character
  - Enforced by both frontend and backend

### Session Security

- [ ] **Session expiration configured** (default: 7 days)
- [ ] **Secure cookie flag enabled** (when HTTPS_ONLY=true)
- [ ] **HttpOnly cookies** (automatic)
- [ ] **SameSite=Lax** (automatic)

### 2FA (Optional but Recommended)

- [ ] **TOTP 2FA available to users**
- [ ] **Recovery codes properly stored** (hashed in database)

---

## 10. Dependency Security

### Backend Dependencies

- [ ] **Go modules up to date**
  ```bash
  cd /opt/subtitler/backend
  go list -m -u all | grep -v '^\s*$'
  # Review available updates
  ```

- [ ] **Security advisories reviewed**
  ```bash
  go list -m -json all | govulncheck -json ./...
  # Install govulncheck: go install golang.org/x/vuln/cmd/govulncheck@latest
  ```

### Frontend Dependencies

- [ ] **npm packages updated**
  ```bash
  cd /opt/subtitler/frontend
  npm audit
  # Address high/critical vulnerabilities
  ```

### External Services

- [ ] **Whisper model version documented**
  ```bash
  # Record which model is in use (e.g., large-v3-turbo)
  ```

- [ ] **Resend API key rotated periodically** (if used)
- [ ] **Cloudflare tunnel credentials reviewed**

---

## Security Headers Verification

The backend automatically sets these security headers. Verify they're present:

```bash
curl -I https://your-domain.com | grep -E "X-Content-Type|X-Frame|Content-Security|Strict-Transport|Permissions-Policy|Referrer-Policy"
```

Expected headers:
- [ ] `X-Content-Type-Options: nosniff`
- [ ] `X-Frame-Options: DENY`
- [ ] `Content-Security-Policy: ...` (script-src, style-src, etc.)
- [ ] `Strict-Transport-Security: ...` (when HTTPS_ONLY=true)
- [ ] `Permissions-Policy: ...` (disables unused features)
- [ ] `Referrer-Policy: strict-origin-when-cross-origin`

---

## Post-Deployment Verification

After completing this checklist:

1. [ ] **Run all backend tests**
   ```bash
   cd /opt/subtitler/backend && go test ./... -v
   ```

2. [ ] **Test complete user flow**
   - Register new account
   - Verify email
   - Upload video
   - Wait for transcription
   - Download subtitles
   - Delete video

3. [ ] **Security scan** (recommended)
   - Run OWASP ZAP or similar against the application
   - Review findings and address high-severity issues

4. [ ] **Document deployment**
   - Record date of deployment
   - Note any deviations from this checklist
   - Update emergency contacts in DISASTER_RECOVERY.md

---

## Periodic Review Schedule

| Task | Frequency | Last Completed |
|------|-----------|----------------|
| Review this checklist | Monthly | |
| Update system packages | Weekly | |
| Review access logs | Weekly | |
| Test backup restoration | Monthly | |
| Rotate API keys | Quarterly | |
| Dependency audit | Monthly | |
| Security scan | Quarterly | |

---

## See Also

- [docs/DISASTER_RECOVERY.md](DISASTER_RECOVERY.md) - Recovery procedures
- [docs/RATE_LIMITS.md](RATE_LIMITS.md) - Rate limiting configuration
- [docs/ENV.md](ENV.md) - Environment variable reference
- [specs/deployment.md](../specs/deployment.md) - Full deployment architecture
- [specs/auth.md](../specs/auth.md) - Authentication security details
- [specs/encryption.md](../specs/encryption.md) - Encryption implementation
