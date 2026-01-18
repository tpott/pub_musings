# Learnings

This file captures lessons learned, failed approaches, and decisions made during development. Ralph should add entries here when:

- A plan fails and is deleted
- Dependencies are added or modified (explain why)
- An unexpected issue is encountered and resolved
- A design decision is made that future iterations should know about

---

## Task 11: Cloudflare Tunnel Setup (2026-01-18)

**Status:** Blocked - requires human intervention

**What was completed:**
- Implemented `/api/health` endpoint with JSON response
- Added configurable CORS via `FRONTEND_URL` environment variable
- Created comprehensive setup documentation in `009_CLOUDFLARE_TUNNEL.md`
- All tests passing, code ready for deployment

**Blocker:**
Task 11's "done_when" criterion requires a live tunnel: `curl https://subtitler.yourdomain.com/api/health returns 200 via tunnel`

This requires:
1. **Domain decision** - Need to choose domain name (e.g., subtitler.yourdomain.com)
2. **VM access** - Need SSH access to Ubuntu VM to install cloudflared
3. **Cloudflare account** - Need authentication to create tunnel
4. **DNS configuration** - Need to configure DNS routes in Cloudflare dashboard

**Decision:** Mark task as "blocked" rather than "complete" because the verification step cannot be performed by an autonomous agent. The implementation is complete, but deployment requires human action.

**For next iteration:**
- If human has completed VM setup, unblock this task and verify the tunnel
- Otherwise, skip to Task 12 (CI/CD) or Task 13 (Secrets) which may also have deployment dependencies

