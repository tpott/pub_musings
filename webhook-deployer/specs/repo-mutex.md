# Repository-Level Mutex for Parallel Deploys

## Problem

When multiple sites sharing the same git repository (e.g., `subtitler-frontend` and `subtitler-backend`) are triggered by the same push, they deploy in parallel goroutines. Both scripts run `git fetch/checkout/pull` on `/home/trevor/pub_musings`, causing "cannot lock ref" errors.

The existing per-site mutex (`siteMutexs`) only prevents the same site from concurrent deploys—it doesn't serialize different sites that share a repository.

## Solution: Add `repo_path` Config Field + Repository-Level Mutex

### Approach

1. **Add `repo_path` field to SiteConfig** - Explicit config for the git repository root
2. **Add `repoMutexs` map to WebhookHandler** - Mutex per unique `repo_path`
3. **Acquire repo mutex before deploy** - Serialize all deploys sharing the same repo

### Why Explicit Config vs Auto-Detection

| Approach | Pros | Cons |
|----------|------|------|
| Explicit `repo_path` field | Clear, predictable, no runtime magic | Requires config update |
| Auto-detect via `git rev-parse` | No config changes | Adds complexity, potential edge cases |

**Decision:** Explicit config is cleaner and aligns with the existing design philosophy.

---

## Files to Modify

### 1. `webhook-deployer/config.go`

Add `RepoPath` field to `SiteConfig`:

```go
type SiteConfig struct {
    Name         string            `yaml:"name"`
    Path         string            `yaml:"path"`
    PathPrefix   string            `yaml:"path_prefix"`
    Branch       string            `yaml:"branch"`
    Repository   string            `yaml:"repository"`
    RepoPath     string            `yaml:"repo_path"`     // NEW: Git repository root
    DeployScript string            `yaml:"deploy_script"`
    Commands     []string          `yaml:"commands"`
    Environment  map[string]string `yaml:"environment"`
}
```

### 2. `webhook-deployer/webhook.go`

Add repository-level mutex alongside existing site mutex:

```go
type WebhookHandler struct {
    secret     string
    config     *Config
    siteMutexs map[string]*sync.Mutex
    repoMutexs map[string]*sync.Mutex  // NEW: mutex per repo_path
    mu         sync.Mutex
}

func NewWebhookHandler(secret string, config *Config) *WebhookHandler {
    return &WebhookHandler{
        secret:     secret,
        config:     config,
        siteMutexs: make(map[string]*sync.Mutex),
        repoMutexs: make(map[string]*sync.Mutex),  // NEW
    }
}

func (h *WebhookHandler) getRepoMutex(repoPath string) *sync.Mutex {
    h.mu.Lock()
    defer h.mu.Unlock()

    if h.repoMutexs[repoPath] == nil {
        h.repoMutexs[repoPath] = &sync.Mutex{}
    }
    return h.repoMutexs[repoPath]
}

func (h *WebhookHandler) deploySite(site SiteConfig) {
    // Acquire repo-level mutex first (if repo_path is set)
    if site.RepoPath != "" {
        repoMutex := h.getRepoMutex(site.RepoPath)
        repoMutex.Lock()
        defer repoMutex.Unlock()
    }

    // Then acquire per-site mutex (existing behavior)
    siteMutex := h.getSiteMutex(site.Name)
    siteMutex.Lock()
    defer siteMutex.Unlock()

    // ... rest of deploy logic unchanged
}
```

### 3. `webhook-deployer/config.yaml`

Add `repo_path` to sites sharing a repository:

```yaml
sites:
  - name: personal
    path: /home/trevor/pub_musings/personal
    path_prefix: personal/
    branch: trunk
    repository: tpott/pub_musings
    repo_path: /home/trevor/pub_musings      # NEW
    commands:
      - "git pull origin trunk"
      # ...

  - name: subtitler-frontend
    path: /home/trevor/pub_musings/subtitler/frontend
    path_prefix: subtitler/frontend/
    branch: subtitler_v3
    repository: tpott/pub_musings
    repo_path: /home/trevor/pub_musings      # NEW - same as backend
    deploy_script: /home/trevor/pub_musings/webhook-deployer/scripts/deploy-subtitler-frontend.sh

  - name: subtitler-backend
    path: /home/trevor/pub_musings/subtitler/backend
    path_prefix: subtitler/backend/
    branch: subtitler_v3
    repository: tpott/pub_musings
    repo_path: /home/trevor/pub_musings      # NEW - same as frontend
    deploy_script: /home/trevor/pub_musings/webhook-deployer/scripts/deploy-subtitler-backend.sh
```

---

## Behavior After Change

When a push to `subtitler_v3` triggers both frontend and backend deploys:

1. Both goroutines start
2. Both try to acquire the same repo mutex (`/home/trevor/pub_musings`)
3. One wins, deploys completely (git commands + build)
4. Then releases repo mutex
5. Second one acquires mutex, deploys
6. No more git lock conflicts

---

## Verification

1. **Build webhook-deployer**: `cd webhook-deployer && go build`
2. **Deploy to server** and restart the service
3. **Trigger a test push** that modifies files in both `subtitler/frontend/` and `subtitler/backend/`
4. **Check logs**: Should see sequential deploys instead of interleaved failures:
   ```
   [subtitler-frontend] Starting deploy...
   [subtitler-frontend] Deploy completed successfully
   [subtitler-backend] Starting deploy...
   [subtitler-backend] Deploy completed successfully
   ```

---

## Notes

- `repo_path` is optional for backward compatibility
- Sites without `repo_path` behave as before (no repo-level locking)
- The per-site mutex is still retained to prevent rapid duplicate webhooks for the same site
