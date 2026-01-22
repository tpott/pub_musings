# Webhook Deployer Integration Plan

Plan for triggering automated deployments of the Subtitler application via `pub_musings/webhook-deployer`.

## Current State

The existing webhook-deployer (`/home/trevor/pub_musings/webhook-deployer/`) handles:
- GitHub webhook deployments for a single site (personal website)
- Contact form submissions via Resend

**Limitation:** Currently hardcoded for one site path via `SITE_PATH` env var.

## Goal

Extend webhook-deployer to support multiple deployment targets:
1. **subtitler-frontend** - Astro static build
2. **subtitler-backend** - Go binary compilation and restart

## Architecture

### Multi-Site Configuration

```go
// Site represents a deployable site
type Site struct {
    Name       string   // Unique identifier
    Path       string   // Local path to source
    Branch     string   // Branch to deploy (e.g., "trunk", "main")
    DeployCmd  string   // Custom deploy command
    Repository string   // GitHub repository name (e.g., "trevor/pub_musings")
}

// Configuration loaded from YAML or env
var sites = []Site{
    {
        Name:       "personal",
        Path:       "/home/trevor/pub_musings/personal",
        Branch:     "trunk",
        Repository: "trevor/pub_musings",
        DeployCmd:  "npm ci && npm run build",
    },
    {
        Name:       "subtitler-frontend",
        Path:       "/home/trevor/pub_musings/subtitler/frontend",
        Branch:     "subtitler_v3",
        Repository: "trevor/pub_musings",
        DeployCmd:  "npm ci && npm run build && rsync -avz dist/ /var/www/subtitler/",
    },
    {
        Name:       "subtitler-backend",
        Path:       "/home/trevor/pub_musings/subtitler/backend",
        Branch:     "subtitler_v3",
        Repository: "trevor/pub_musings",
        DeployCmd:  "/home/trevor/go/bin/go build -o /opt/subtitler/backend/subtitler && sudo systemctl restart subtitler",
    },
}
```

### Webhook Routing

The webhook handler needs to:
1. Validate GitHub signature (existing)
2. Parse push event to get repository and changed files
3. Determine which site(s) were affected
4. Trigger appropriate deploy(s)

```go
func (h *WebhookHandler) Handle(w http.ResponseWriter, r *http.Request) {
    // ... existing validation ...

    // Get commits and changed files
    var payload GitHubPushEventExtended
    json.Unmarshal(body, &payload)

    // Find affected sites
    affectedSites := h.findAffectedSites(payload)

    // Deploy each affected site
    for _, site := range affectedSites {
        go h.deploySite(site)
    }
}

func (h *WebhookHandler) findAffectedSites(payload GitHubPushEventExtended) []Site {
    var affected []Site

    for _, site := range h.sites {
        // Check if repository matches
        if site.Repository != payload.Repository.FullName {
            continue
        }

        // Check if branch matches
        expectedRef := "refs/heads/" + site.Branch
        if payload.Ref != expectedRef {
            continue
        }

        // Check if any changed files are in site's path
        for _, commit := range payload.Commits {
            for _, file := range append(commit.Added, append(commit.Modified, commit.Removed...)...) {
                if strings.HasPrefix(file, site.PathPrefix) {
                    affected = append(affected, site)
                    break
                }
            }
        }
    }

    return affected
}
```

### Extended GitHub Payload

```go
type GitHubPushEventExtended struct {
    Ref        string `json:"ref"`
    Repository struct {
        FullName string `json:"full_name"`
    } `json:"repository"`
    Pusher struct {
        Name string `json:"name"`
    } `json:"pusher"`
    Commits []struct {
        ID       string   `json:"id"`
        Message  string   `json:"message"`
        Added    []string `json:"added"`
        Modified []string `json:"modified"`
        Removed  []string `json:"removed"`
    } `json:"commits"`
}
```

## Configuration File

Create `webhook-deployer/config.yaml`:

```yaml
sites:
  - name: personal
    path: /home/trevor/pub_musings/personal
    path_prefix: personal/
    branch: trunk
    repository: trevor/pub_musings
    deploy:
      - npm ci
      - npm run build

  - name: subtitler-frontend
    path: /home/trevor/pub_musings/subtitler/frontend
    path_prefix: subtitler/frontend/
    branch: subtitler_v3
    repository: trevor/pub_musings
    deploy:
      - npm ci
      - npm run build
      - rsync -avz dist/ /var/www/subtitler/

  - name: subtitler-backend
    path: /home/trevor/pub_musings/subtitler/backend
    path_prefix: subtitler/backend/
    branch: subtitler_v3
    repository: trevor/pub_musings
    deploy:
      - /home/trevor/go/bin/go build -o /opt/subtitler/backend/subtitler
      - sudo systemctl restart subtitler
```

## Deployment Commands

### subtitler-frontend

```bash
#!/bin/bash
cd /home/trevor/pub_musings/subtitler/frontend

# Pull latest
git pull origin subtitler_v3

# Install dependencies
npm ci

# Build static files
npm run build

# Copy to web root
rsync -avz --delete dist/ /var/www/subtitler/

echo "Frontend deployed successfully"
```

### subtitler-backend

```bash
#!/bin/bash
cd /home/trevor/pub_musings/subtitler/backend

# Pull latest
git pull origin subtitler_v3

# Build binary
/home/trevor/go/bin/go build -o /opt/subtitler/backend/subtitler-new

# Atomic swap
mv /opt/subtitler/backend/subtitler-new /opt/subtitler/backend/subtitler

# Restart service
sudo systemctl restart subtitler

# Verify health
sleep 2
curl -f http://localhost:8080/api/health || echo "Health check failed!"

echo "Backend deployed successfully"
```

## GitHub Webhook Configuration

### Webhook URL

```
https://webhook.pottingers.us/webhook
```

### Events

Enable only **push** events (already configured).

### Path Filtering

GitHub webhooks don't support path filtering natively. The webhook-deployer must inspect the `commits[].added/modified/removed` arrays to determine affected paths.

## Security Considerations

### Sudo for Backend Restart

The backend deploy needs `sudo systemctl restart subtitler`. Options:

1. **sudoers entry** (recommended):
   ```
   trevor ALL=(ALL) NOPASSWD: /usr/bin/systemctl restart subtitler
   ```

2. **Socket activation** - systemd starts service on demand, no restart needed

3. **Signal-based reload** - if backend supported graceful reload

### File Permissions

- Build artifacts should be owned by deploy user
- Service runs as `subtitler` user (non-privileged)
- `/var/www/subtitler` writable by deploy user

## Implementation Tasks

### Phase 1: Multi-site support in webhook-deployer

1. Add `config.yaml` support
2. Extend `GitHubPushEvent` struct
3. Implement `findAffectedSites()` logic
4. Create site-specific deploy functions

### Phase 2: Deploy scripts

1. Create `subtitler/scripts/deploy-frontend.sh`
2. Create `subtitler/scripts/deploy-backend.sh`
3. Test manually before enabling webhook

### Phase 3: Integration

1. Add subtitler sites to `config.yaml`
2. Test with non-production branch first
3. Enable for `subtitler_v3` branch

### Phase 4: Monitoring

1. Add deploy notifications (Slack, Discord, email)
2. Track deploy history
3. Add rollback capability

## Testing Locally

### Simulate webhook

```bash
# Generate signature
echo -n '{"ref":"refs/heads/subtitler_v3",...}' | \
  openssl dgst -sha256 -hmac "$WEBHOOK_SECRET" | \
  awk '{print "sha256="$2}'

# Send test webhook
curl -X POST http://localhost:9000/webhook \
  -H "Content-Type: application/json" \
  -H "X-GitHub-Event: push" \
  -H "X-Hub-Signature-256: sha256=..." \
  -d @test-payload.json
```

### Test payload

```json
{
  "ref": "refs/heads/subtitler_v3",
  "repository": {
    "full_name": "trevor/pub_musings"
  },
  "pusher": {
    "name": "trevor"
  },
  "commits": [
    {
      "id": "abc123",
      "message": "Update frontend",
      "added": [],
      "modified": ["subtitler/frontend/src/pages/index.astro"],
      "removed": []
    }
  ]
}
```

## Rollback Strategy

### Frontend Rollback

```bash
# Keep last N builds
ls -t /var/www/subtitler-backups/ | tail -n +6 | xargs rm -rf

# Before deploy, backup current
cp -r /var/www/subtitler /var/www/subtitler-backups/$(date +%Y%m%d_%H%M%S)

# Rollback
cp -r /var/www/subtitler-backups/YYYYMMDD_HHMMSS/* /var/www/subtitler/
```

### Backend Rollback

```bash
# Keep last N binaries
ls -t /opt/subtitler/backend/subtitler-* | tail -n +6 | xargs rm -f

# Before deploy, keep old binary
mv /opt/subtitler/backend/subtitler /opt/subtitler/backend/subtitler-$(date +%Y%m%d_%H%M%S)

# Rollback
mv /opt/subtitler/backend/subtitler-YYYYMMDD_HHMMSS /opt/subtitler/backend/subtitler
sudo systemctl restart subtitler
```

## Timeline

| Phase | Description | Effort |
|-------|-------------|--------|
| 1 | Multi-site webhook support | 2-3 hours |
| 2 | Deploy scripts | 1 hour |
| 3 | Integration & testing | 1-2 hours |
| 4 | Monitoring (optional) | 2-3 hours |

## See Also

- [deployment.md](deployment.md) - Deployment architecture
- [../pub_musings/webhook-deployer/README.md](../../webhook-deployer/README.md) - Current webhook-deployer
