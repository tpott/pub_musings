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

### Dashboard Metrics

Track these metrics on your monitoring dashboard:

- Login success/failure ratio over time
- Unique IPs hitting rate limits
- 2FA adoption rate (enabled events)
- Password reset frequency
- Session revocations per user

### Log Aggregation Queries

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
