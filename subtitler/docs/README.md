# docs/

Reference documentation for the subtitler project. For specifications and design
documents, see `specs/`.

## Files

| File | Description |
|------|-------------|
| [API.md](API.md) | Complete API documentation: all endpoints, request/response schemas, authentication methods, and example curl commands |
| [deps.md](deps.md) | Dependency justification for Go modules, npm packages, system tools, external services, and deployment dependencies |
| [DISASTER_RECOVERY.md](DISASTER_RECOVERY.md) | Disaster recovery procedures for backups, restores, and failover scenarios |
| [ENV.md](ENV.md) | Environment variable reference for server, storage, Whisper, email, security, and rate limit configuration |
| [ERROR_CODES.md](ERROR_CODES.md) | Catalog of backend error codes returned by the API |
| [METRICS.md](METRICS.md) | Prometheus metrics exposed by the backend (`/metrics` endpoint) |
| [RATE_LIMITS.md](RATE_LIMITS.md) | Rate limiting configuration and per-endpoint limits |
| [SECURITY_CHECKLIST.md](SECURITY_CHECKLIST.md) | Security hardening checklist for production deployments |
| [SECURITY_EVENTS.md](SECURITY_EVENTS.md) | Security event types, log format, and usage of `scripts/query-security-events.py` |
