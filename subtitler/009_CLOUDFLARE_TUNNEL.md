# 009 Cloudflare Tunnel Setup

## Overview

Expose the Subtitler service publicly via Cloudflare Tunnel. The tunnel will route traffic to both the Astro frontend (via Caddy) and the Go API backend.

## Architecture

```
┌─────────────┐                         ┌──────────────────────────────────────────┐
│ Cloudflare  │   tunnel                │              Ubuntu VM                   │
│   Edge      │◀────────────────────────│  ┌─────────────────────────────────────┐ │
│             │                         │  │  cloudflared                        │ │
└─────────────┘                         │  │  - subtitler.example.com → :4321    │ │
       │                                │  │  - api.subtitler.example.com → :8080│ │
       ▼                                │  └─────────────────────────────────────┘ │
   Internet                             │                    │                     │
                                        │                    ▼                     │
                                        │  ┌─────────────────────────────────────┐ │
                                        │  │  Astro frontend (port 4321)         │ │
                                        │  │  - serves static site                │ │
                                        │  └─────────────────────────────────────┘ │
                                        │                                          │
                                        │  ┌─────────────────────────────────────┐ │
                                        │  │  Go backend (port 8080)             │ │
                                        │  │  - /api/* endpoints                 │ │
                                        │  └─────────────────────────────────────┘ │
                                        └──────────────────────────────────────────┘
```

## Prerequisites

- Domain name with DNS managed by Cloudflare (or TBD domain)
- Ubuntu VM with internet access
- Cloudflare account

## Implementation Steps

### Step 1: Install cloudflared

```bash
# Download and install cloudflared
wget https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-amd64.deb
sudo dpkg -i cloudflared-linux-amd64.deb

# Verify installation
cloudflared --version
```

### Step 2: Authenticate with Cloudflare

```bash
# Login to Cloudflare (opens browser for authentication)
cloudflared tunnel login

# This creates ~/.cloudflared/cert.pem
```

### Step 3: Create Tunnel

```bash
# Create a new tunnel named "subtitler"
cloudflared tunnel create subtitler

# Note the tunnel ID from output (e.g., "Created tunnel subtitler with id abc123...")
# This creates ~/.cloudflared/<tunnel-id>.json credentials file
```

### Step 4: Configure DNS Routes

**Option A: Using a domain** (e.g., `subtitler.yourdomain.com`):
```bash
cloudflared tunnel route dns subtitler subtitler.yourdomain.com
cloudflared tunnel route dns subtitler api.subtitler.yourdomain.com
```

**Option B: TBD domain** (placeholder for now):
```bash
# To be determined based on domain acquisition
# For now, document the pattern:
# cloudflared tunnel route dns subtitler <frontend-domain>
# cloudflared tunnel route dns subtitler <api-domain>
```

### Step 5: Create Tunnel Configuration

Create `/etc/cloudflared/config.yml`:

```yaml
tunnel: <tunnel-id>
credentials-file: /home/trevor/.cloudflared/<tunnel-id>.json

ingress:
  # Frontend (Astro)
  - hostname: subtitler.yourdomain.com
    service: http://localhost:4321

  # API Backend (Go)
  - hostname: api.subtitler.yourdomain.com
    service: http://localhost:8080

  # Catch-all
  - service: http_status:404
```

Replace:
- `<tunnel-id>` with actual tunnel ID from Step 3
- `subtitler.yourdomain.com` with actual domain
- `api.subtitler.yourdomain.com` with actual API domain

### Step 6: Install and Start as Service

```bash
# Install as systemd service
sudo cloudflared service install

# Start the service
sudo systemctl start cloudflared

# Enable on boot
sudo systemctl enable cloudflared

# Check status
sudo systemctl status cloudflared

# View logs
journalctl -u cloudflared -f
```

### Step 7: Update Backend Configuration

Update `backend/internal/config/config.go` to accept CORS from tunnel domains:

```go
// Add frontend URL configuration
FrontendURL string
```

Update backend startup to configure CORS:
```go
// In main.go
c := cors.New(cors.Options{
    AllowedOrigins:   []string{cfg.FrontendURL},
    AllowCredentials: true,
    AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
    AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
})
```

### Step 8: Update Frontend Configuration

Update `frontend/astro.config.mjs` to set correct API URL:

```javascript
export default defineConfig({
  // ... existing config
  env: {
    API_URL: process.env.PUBLIC_API_URL || 'http://localhost:8080'
  }
});
```

Update frontend API calls to use `import.meta.env.PUBLIC_API_URL`.

### Step 9: Health Check Endpoint

Add a health check endpoint to the backend for tunnel verification:

```go
// In backend/cmd/server/main.go
http.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]interface{}{
        "success": true,
        "message": "Service is healthy",
        "version": "1.0.0",
    })
})
```

## Verification

### Test Locally First

```bash
# Start frontend
cd frontend && npm run dev
# Should be accessible at http://localhost:4321

# Start backend
cd backend && go run ./cmd/server
# Should be accessible at http://localhost:8080
```

### Test via Tunnel

```bash
# Test frontend
curl https://subtitler.yourdomain.com

# Test API health check
curl https://api.subtitler.yourdomain.com/api/health
# Expected: {"success":true,"message":"Service is healthy","version":"1.0.0"}

# Test API endpoint
curl https://api.subtitler.yourdomain.com/api/me
# Expected: 401 Unauthorized (requires auth)
```

### Test Full Flow

1. Open `https://subtitler.yourdomain.com` in browser
2. Register a new account
3. Upload a test audio file
4. Verify job appears in dashboard
5. Wait for completion and download result

## Security Considerations

1. **HTTPS Only**: Cloudflare Tunnel provides TLS by default
2. **CORS**: Configure allowed origins to only tunnel domains
3. **JWT Cookies**: Ensure `Secure` flag is set in production
4. **Rate Limiting**: Consider adding rate limiting to API endpoints
5. **File Upload Limits**: Already configured at 200MB

## Secrets Management

For production deployment, create `.env` files:

```bash
# backend/.env
DATABASE_PATH=./data/db/subtitler.db
JWT_SECRET=<generate-secure-random-secret>
SERVER_PORT=8080
WHISPER_MODEL_PATH=/home/trevor/Github/whisper.cpp/models/ggml-medium.bin
WHISPER_SERVER_PATH=/home/trevor/Github/whisper.cpp/build/bin/whisper-server
RESEND_API_KEY=<your-resend-api-key>
EMAIL_FROM=noreply@subtitler.yourdomain.com
ENABLE_EMAIL=true
FRONTEND_URL=https://subtitler.yourdomain.com

# frontend/.env
PUBLIC_API_URL=https://api.subtitler.yourdomain.com
```

Consider using `sops` + `age` for encrypted secrets (see `personal/001_INITIALIZATION.md` for pattern).

## Monitoring

```bash
# Tunnel logs
journalctl -u cloudflared -f

# Backend logs
journalctl -u subtitler-backend -f  # (if running as systemd service)

# Frontend logs
journalctl -u subtitler-frontend -f  # (if running as systemd service)
```

## Troubleshooting

| Issue | Solution |
|-------|----------|
| Tunnel not connecting | Check `journalctl -u cloudflared -f` for errors |
| 404 on API calls | Verify backend is running on port 8080 |
| CORS errors | Check CORS configuration in backend |
| DNS not resolving | Verify DNS records in Cloudflare dashboard |
| Health check fails | Ensure backend is running and accessible |

## Done When

```bash
curl https://subtitler.yourdomain.com/api/health
# Returns: {"success":true,"message":"Service is healthy","version":"1.0.0"}
```

Or if domain TBD:
```bash
# Tunnel is configured and ready
sudo systemctl status cloudflared  # Shows active (running)
curl http://localhost:8080/api/health  # Returns success locally
```

## Next Steps (Task 12)

After tunnel is verified:
- Set up CI/CD pipeline for automated deployments
- Configure systemd services for frontend and backend
- Set up GitHub webhook for push-to-deploy workflow
