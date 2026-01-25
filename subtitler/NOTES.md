# Corrections to deployment.md

The existing `subtitler/specs/deployment.md` contains some incorrect assumptions about the deployment architecture.

## Architecture Reality

The webhook-deployer runs **inside the Ubuntu VM**, not on the Mac host. This means:

1. **No cross-compilation needed** - builds happen natively on ARM64
2. **No SSH/SCP transfers** - files are local to the VM
3. **Simpler deployment** - just git pull and build

## Specific Corrections

| deployment.md Says | Reality |
|--------------------|---------|
| Build on Mac host with cross-compilation | Build natively on ARM64 VM |
| `CGO_ENABLED=1 GOOS=linux GOARCH=arm64` | `CGO_ENABLED=1 go build` (native, CGO required for sqlite) |
| SSH/SCP from host to VM | No SSH - webhook-deployer runs on VM |
| `rsync -avz dist/ vm:/var/www/subtitler/` | No rsync needed - Caddy serves from `frontend/dist/` |

## Why This Matters

The deployment spec was written assuming a "build on dev machine, deploy to VM" workflow. The actual architecture uses webhook-deployer running inside the VM, which:

- Eliminates cross-compilation complexity
- Removes the need for SSH key management
- Simplifies the deployment pipeline
- Allows the VM to be completely self-sufficient

## Current Deployment Flow

1. Push to GitHub
2. GitHub sends webhook to `webhook.pottingers.us`
3. Cloudflare tunnel routes to `localhost:9000` on the VM
4. webhook-deployer receives the push event
5. If branch/path matches a configured site, deploy script runs
6. Script does `git pull`, builds, and deploys - all locally on the VM
