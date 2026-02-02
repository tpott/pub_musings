# Security Event Audit Logging

This document describes the security events logged by the Subtitler backend for monitoring and auditing purposes.

## Overview

All security events are logged through the `security` package with consistent formatting for easy log parsing and analysis. Events include:

- **Timestamp** - When the event occurred
- **Event type** - Categorized event identifier
- **IP address** - Client IP (respects TRUST_PROXY setting)
- **Additional context** - User ID, email, resource IDs as applicable

## Event Types

### Authentication Events

| Event | Description | Fields |
|-------|-------------|--------|
| `auth.login.success` | Successful login | `user_id`, `email` |
| `auth.login.failed.password` | Failed login - wrong password | `email`, `attempt_count` |
| `auth.login.failed.user_not_found` | Failed login - user doesn't exist | `email` |
| `auth.login.failed.unverified` | Failed login - email not verified | `email` |
| `auth.login.failed.locked` | Failed login - account locked | `email`, `locked_minutes_remaining` |
| `auth.login.failed.captcha` | Failed login - CAPTCHA failed | `reason` |
| `auth.logout` | User logged out | `user_id` |
| `auth.registration` | New user registered | `user_id`, `email` |

### Two-Factor Authentication Events

| Event | Description | Fields |
|-------|-------------|--------|
| `auth.2fa.setup_initiated` | User started 2FA setup | `user_id`, `email` |
| `auth.2fa.enabled` | 2FA enabled successfully | `user_id`, `email` |
| `auth.2fa.disabled` | 2FA disabled | `user_id`, `email`, `via_recovery` |
| `auth.2fa.verify.success` | 2FA code verified | `user_id`, `email` |
| `auth.2fa.verify.failed` | Invalid 2FA code | `user_id`, `email` |
| `auth.2fa.recovery_used` | Recovery code used | `user_id`, `email` |
| `auth.2fa.recovery_failed` | Invalid recovery code | `email` |
| `auth.2fa.codes_regenerated` | Recovery codes regenerated | `user_id`, `email` |

### Password Events

| Event | Description | Fields |
|-------|-------------|--------|
| `auth.password.reset_requested` | Password reset requested | `email` (at INFO if user exists, DEBUG if not) |
| `auth.password.reset_success` | Password reset completed | `user_id` |
| `auth.password.reset_failed` | Password reset failed | `reason` |

### Magic Link Events

| Event | Description | Fields |
|-------|-------------|--------|
| `auth.magiclink.requested` | Magic link requested | `email` |
| `auth.magiclink.success` | Magic link login successful | `user_id`, `email` |
| `auth.magiclink.failed` | Magic link failed | `reason` |

### Account Lockout Events

| Event | Description | Fields |
|-------|-------------|--------|
| `auth.account.locked` | Account locked due to failed attempts | `email`, `failed_attempts` |
| `auth.account.unlocked` | Account unlocked | `email` |

### Session Events

| Event | Description | Fields |
|-------|-------------|--------|
| `session.created` | New session created | `user_id`, `session_id` |
| `session.revoked` | Session revoked | `user_id`, `session_id`, `self_revoke` |
| `session.expired` | Session expired | `session_id` |
| `session.invalid` | Invalid session presented | `reason` |

### Access Control Events

| Event | Description | Fields |
|-------|-------------|--------|
| `access.denied` | Generic access denied | `resource`, `reason`, `user_id` |
| `access.denied.not_owner` | User tried to access another user's resource | `user_id`, `resource_type`, `resource_id` |
| `access.denied.not_authenticated` | Unauthenticated access to protected resource | `resource` |
| `access.denied.not_admin` | Non-admin access to admin resource | `user_id`, `resource` |

### Rate Limit Events

| Event | Description | Fields |
|-------|-------------|--------|
| `ratelimit.exceeded` | Rate limit exceeded | `endpoint`, `limit_name` |

### File Access Events

| Event | Description | Fields |
|-------|-------------|--------|
| `file.access` | File accessed (audit) | `user_id`, `file_type`, `file_id` |
| `file.access.denied` | File access denied | `file_type`, `file_id`, `reason` |

### Admin Operations - Key Rotation

| Event | Description | Fields |
|-------|-------------|--------|
| `admin.rotation.started` | New encryption key generated | `old_version`, `new_version` |
| `admin.rotation.completed` | Key rotation finished | `old_version`, `new_version` |
| `admin.rotation.reencrypt_started` | File re-encryption started | `total_files` |
| `admin.rotation.reencrypt_progress` | Batch re-encryption progress | `processed_files`, `total_files`, `current_version` |
| `admin.rotation.reencrypt_completed` | Re-encryption finished | `total_files`, `duration_seconds` |
| `admin.rotation.reencrypt_failed` | Re-encryption failed | `video_id`, `error` |

### Admin Operations - File Deletion

| Event | Description | Fields |
|-------|-------------|--------|
| `admin.delete.video_user` | User deleted own video | `user_id`, `video_id`, `filename` |
| `admin.delete.video_system` | System deleted expired video | `video_id`, `filename`, `is_anonymous`, `retention_hours` |

### Admin Operations - Feedback Management

| Event | Description | Fields |
|-------|-------------|--------|
| `admin.feedback.updated` | Admin updated feedback status | `admin_user_id`, `feedback_id`, `new_status` |

### Admin Operations - Database Maintenance

| Event | Description | Fields |
|-------|-------------|--------|
| `admin.maintenance.started` | Database maintenance started | - |
| `admin.maintenance.completed` | Database maintenance finished | `duration_seconds` |
| `admin.maintenance.failed` | Database maintenance failed | `error` |
| `admin.cleanup.started` | Scheduled cleanup started | - |
| `admin.cleanup.completed` | Scheduled cleanup finished | `videos_deleted`, `sessions_deleted`, `login_attempts_deleted`, `upload_sessions_deleted`, `orphan_chunks_deleted` |

## Log Levels

Security events use different log levels to indicate severity:

- **INFO** - Normal security operations (successful logins, registrations)
- **WARN** - Security warnings requiring attention (failed logins, rate limits, access denied)
- **ERROR** - Security errors (system failures during auth operations)

## Example Log Entries

```
level=INFO msg="Security event" event=auth.login.success ip=192.168.1.1 user_id=abc123 email=user@example.com
level=WARN msg="Security warning" event=auth.login.failed.password ip=192.168.1.1 email=user@example.com attempt_count=3
level=WARN msg="Security warning" event=auth.account.locked ip=192.168.1.1 email=user@example.com failed_attempts=5
level=WARN msg="Security warning" event=ratelimit.exceeded ip=10.0.0.5 endpoint=/api/auth/login limit_name=ip
```

## Monitoring Recommendations

### High Priority Alerts

Configure alerts for these patterns:

1. **Account Lockouts** - `event=auth.account.locked`
   - Multiple lockouts from same IP may indicate attack
   - Alert on > 5 lockouts per hour

2. **Brute Force Attempts** - `event=auth.login.failed.password`
   - Multiple failures from same IP
   - Alert on > 20 failures per hour per IP

3. **Rate Limit Violations** - `event=ratelimit.exceeded`
   - Sustained violations from single IP
   - Alert on > 50 violations per hour per IP

4. **2FA Bypass Attempts** - `event=auth.2fa.recovery_failed`
   - Multiple failed recovery code attempts
   - Alert on any occurrences

5. **Admin Access Denied** - `event=access.denied.not_admin`
   - Non-admin attempting admin functions
   - Alert on any occurrence, investigate immediately

### Dashboard Metrics

Track these metrics on your monitoring dashboard:

- Login success/failure ratio over time
- Unique IPs hitting rate limits
- 2FA adoption rate (enabled events)
- Password reset frequency
- Session revocations per user

## Troubleshooting: No Logs in journalctl

If `journalctl -u subtitler --since "1 day ago"` returns nothing:

1. **Verify service is running:**
   ```bash
   sudo systemctl status subtitler
   ```
   If the service is stopped or failed, no new logs will appear.

2. **Verify the unit name matches:**
   ```bash
   systemctl list-units --type=service | grep subtitler
   ```

3. **Check all-time logs (no time filter):**
   ```bash
   sudo journalctl -u subtitler -n 50
   ```
   If this shows output, the service is logging but there were no events in your time window.

4. **Verify journal is configured to persist:**
   ```bash
   sudo mkdir -p /var/log/journal
   sudo systemd-tmpfiles --create --prefix /var/log/journal
   sudo systemctl restart systemd-journald
   ```
   Without persistent journal storage, logs are lost on reboot.

5. **Verify systemd captures stdout/stderr:**
   The service file should include `StandardOutput=journal` and `StandardError=journal`
   (see [specs/deployment.md](../specs/deployment.md) for the complete service file).

6. **Test that logging works:**
   ```bash
   # Restart and immediately check
   sudo systemctl restart subtitler
   sudo journalctl -u subtitler -n 20 --no-pager
   ```
   You should see startup messages like `Server starting on port 8060`.

**Note:** LOG_LEVEL defaults to `info` when not set, which includes all security events. Setting `LOG_LEVEL=error` would suppress security events (logged at INFO/WARN).

## Real-World Monitoring Examples

### Journalctl Commands

Filter security events using systemd journal:

**View all security events (live):**
```bash
journalctl -u subtitler -f | grep -E "Security (event|warning)"
```

**Failed logins in the last hour:**
```bash
journalctl -u subtitler --since "1 hour ago" | grep "auth.login.failed"
```

**Account lockouts today:**
```bash
journalctl -u subtitler --since today | grep "auth.account.locked"
```

**Rate limit violations by IP (count):**
```bash
journalctl -u subtitler --since "1 hour ago" | grep "ratelimit.exceeded" | \
  grep -oE 'ip=[0-9.]+' | sort | uniq -c | sort -rn | head -10
```

**2FA events for specific user:**
```bash
journalctl -u subtitler | grep -E "auth.2fa.*user@example.com"
```

**All events from a specific IP:**
```bash
journalctl -u subtitler | grep "ip=192.168.1.100"
```

**Session events for investigation:**
```bash
journalctl -u subtitler | grep -E "session\.(created|revoked|expired)" | \
  grep "user_id=abc123"
```

### Query Tool (`scripts/query-security-events.py`)

A dedicated Python script for filtering and analyzing security events. Parses the
slog logfmt output and supports filtering by event type, IP, user, date range, and
log level. Pipe journalctl output into it.

**View all security events today:**
```bash
journalctl -u subtitler --since today | ./scripts/query-security-events.py
```

**Filter failed logins:**
```bash
journalctl -u subtitler | ./scripts/query-security-events.py --event auth.login.failed
```

**Events from a specific IP:**
```bash
journalctl -u subtitler --since "1 hour ago" | ./scripts/query-security-events.py --ip 192.168.1.100
```

**All events for a user (by email or user_id):**
```bash
journalctl -u subtitler | ./scripts/query-security-events.py --user user@example.com
journalctl -u subtitler | ./scripts/query-security-events.py --user abc123-def456
```

**Warnings only in a date range:**
```bash
journalctl -u subtitler | ./scripts/query-security-events.py --level WARN --since 2024-01-01 --until 2024-01-31
```

**Count events by type:**
```bash
journalctl -u subtitler --since today | ./scripts/query-security-events.py --count-by event
```
```
 Count  event
 -----  ------------------------------
    42  auth.login.success
    12  auth.login.failed.password
     3  ratelimit.exceeded
     2  auth.account.locked
     1  access.denied.not_admin

Total: 60 events
```

**Count events by IP (top offenders):**
```bash
journalctl -u subtitler --since "1 hour ago" | ./scripts/query-security-events.py --level WARN --count-by ip
```

**Show last 10 events:**
```bash
journalctl -u subtitler | ./scripts/query-security-events.py --tail 10
```

**Raw logfmt output (for piping to other tools):**
```bash
journalctl -u subtitler | ./scripts/query-security-events.py --event auth.login.failed --format raw
```

**Combined filters (AND logic):**
```bash
journalctl -u subtitler | ./scripts/query-security-events.py --event auth --level WARN --ip 192.168.1.2
```

Run `./scripts/query-security-events.py --help` for full usage.

### Prometheus Alert Rules

Save as `/etc/prometheus/rules/subtitler_security.yml`:

```yaml
groups:
  - name: subtitler_security
    interval: 1m
    rules:
      # Alert: Too many failed logins from same IP
      - alert: BruteForceAttempt
        expr: |
          sum by (ip) (
            count_over_time({job="subtitler"} |~ "auth.login.failed" [15m])
          ) > 10
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Possible brute force attempt from {{ $labels.ip }}"
          description: "{{ $value }} failed login attempts in 15 minutes"

      # Alert: Account lockouts spike
      - alert: AccountLockoutSpike
        expr: |
          sum(count_over_time({job="subtitler"} |~ "auth.account.locked" [1h])) > 5
        for: 0m
        labels:
          severity: critical
        annotations:
          summary: "Multiple account lockouts detected"
          description: "{{ $value }} accounts locked in the last hour"

      # Alert: Rate limit abuse
      - alert: RateLimitAbuse
        expr: |
          sum by (ip) (
            count_over_time({job="subtitler"} |~ "ratelimit.exceeded" [1h])
          ) > 50
        for: 10m
        labels:
          severity: warning
        annotations:
          summary: "Rate limit abuse from {{ $labels.ip }}"
          description: "{{ $value }} rate limit violations in 1 hour"

      # Alert: Admin access denied (potential privilege escalation)
      - alert: UnauthorizedAdminAccess
        expr: |
          count_over_time({job="subtitler"} |~ "access.denied.not_admin" [1h]) > 0
        for: 0m
        labels:
          severity: critical
        annotations:
          summary: "Unauthorized admin access attempt"
          description: "Non-admin user attempted to access admin resource"

      # Alert: 2FA bypass attempts
      - alert: TwoFactorBypassAttempt
        expr: |
          count_over_time({job="subtitler"} |~ "auth.2fa.recovery_failed" [1h]) > 0
        for: 0m
        labels:
          severity: critical
        annotations:
          summary: "2FA bypass attempt detected"
          description: "Failed recovery code attempt - investigate immediately"
```

### Grafana Loki Queries

If using Grafana Loki for log aggregation:

**Failed logins over time:**
```logql
sum by (email) (count_over_time({job="subtitler"} |~ "auth.login.failed" [5m]))
```

**Rate limit violations by endpoint:**
```logql
{job="subtitler"} |~ "ratelimit.exceeded"
| regexp `endpoint=(?P<endpoint>[^\s]+)`
| sum by (endpoint) (count_over_time([1h]))
```

**Security events heatmap:**
```logql
{job="subtitler"} |~ "Security (event|warning)"
| regexp `event=(?P<event>[^\s]+)`
| sum by (event) (count_over_time([1h]))
```

### ELK Stack (Elasticsearch) Queries

**Kibana Query - Failed logins last 24 hours:**
```json
{
  "query": {
    "bool": {
      "must": [
        { "match": { "message": "auth.login.failed" } },
        { "range": { "@timestamp": { "gte": "now-24h" } } }
      ]
    }
  },
  "aggs": {
    "by_ip": {
      "terms": { "field": "ip.keyword", "size": 20 }
    }
  }
}
```

**Kibana Query - Account lockouts:**
```json
{
  "query": {
    "bool": {
      "must": [
        { "match": { "message": "auth.account.locked" } }
      ]
    }
  },
  "aggs": {
    "by_email": {
      "terms": { "field": "email.keyword" }
    },
    "over_time": {
      "date_histogram": {
        "field": "@timestamp",
        "fixed_interval": "1h"
      }
    }
  }
}
```

### Log Aggregation Queries (Generic)

**Failed login attempts by IP (last hour):**
```
event="auth.login.failed.*" | count by ip | sort -count | limit 10
```

**Account lockouts timeline:**
```
event="auth.account.locked" | timechart count by email
```

**Rate limit violations by endpoint:**
```
event="ratelimit.exceeded" | count by endpoint
```

## Incident Response Playbooks

### Brute Force Attack Response

1. **Detection:** Alert triggered for > 20 failed logins from single IP
2. **Initial Response:**
   ```bash
   # Check IP reputation
   curl "https://www.abuseipdb.com/check/${IP}/json"

   # Count affected accounts
   journalctl -u subtitler --since "1 hour ago" | grep "ip=${IP}" | \
     grep -oE 'email=[^\s]+' | sort -u | wc -l
   ```
3. **Containment:**
   ```bash
   # Block IP at firewall (example with ufw)
   sudo ufw deny from ${IP}

   # Or add to fail2ban
   sudo fail2ban-client set subtitler banip ${IP}
   ```
4. **Investigation:** Review logs for targeted accounts
5. **Recovery:** Reset affected accounts if needed

### Account Compromise Response

1. **Detection:** Unusual activity pattern (multiple IP login, location change)
2. **Initial Response:**
   ```bash
   # Get all sessions for user
   journalctl -u subtitler | grep "user_id=${USER_ID}" | \
     grep "session.created"
   ```
3. **Containment:**
   ```bash
   # Revoke all sessions via database
   sqlite3 /opt/subtitler/data/subtitler.db \
     "DELETE FROM sessions WHERE user_id='${USER_ID}'"
   ```
4. **Investigation:** Determine entry point (password, magic link, etc.)
5. **Recovery:** Force password reset, recommend 2FA

## Security Considerations

### Privacy

- Email addresses are logged for security monitoring
- User IDs (UUIDs) are logged for correlation
- IP addresses are logged (respect GDPR for EU users)
- Passwords are NEVER logged

### Log Retention

Recommended retention periods:
- Authentication events: 90 days
- Access control events: 90 days
- Rate limit events: 30 days
- File access events: 90 days

### Access Control

- Security logs should be access-controlled
- Only security personnel should have read access
- Consider separate storage for security events

## Integration

### Log Shipping

Security events can be shipped to external systems:

```bash
# Example: Send to syslog
journalctl -u subtitler -f | logger -t subtitler-security

# Example: Ship to Elasticsearch
journalctl -u subtitler -f --output=json | filebeat
```

### SIEM Integration

The structured log format is compatible with common SIEM systems:
- Splunk
- ELK Stack
- Datadog
- Grafana Loki

## See Also

- [docs/SECURITY_CHECKLIST.md](SECURITY_CHECKLIST.md) - Production security checklist
- [docs/RATE_LIMITS.md](RATE_LIMITS.md) - Rate limiting configuration
- [specs/auth.md](../specs/auth.md) - Authentication system details
