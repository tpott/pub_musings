# Personal Website Implementation Plan

## Overview

A personal website built with Astro, served via Caddy on an Ubuntu VM, exposed through Cloudflare Tunnel, with automated deployments via GitHub webhooks.

## Architecture

**Site URL**: `https://t.pottingers.us`
**Webhook URL**: `https://webhook.pottingers.us`
**Site path on VM**: `/home/trevor/pub_musings/personal`

```
┌─────────────┐  webhook.pottingers.us  ┌──────────────────────────────────────────┐
│   GitHub    │────────────────────────▶│              Ubuntu VM                   │
│  (trunk)    │                         │  ┌─────────────────────────────────────┐ │
└─────────────┘                         │  │  webhook-deployer (Go, port 9000)   │ │
                                        │  │  - validates signature              │ │
                                        │  │  - runs: git pull && npm run build  │ │
                                        │  └─────────────────────────────────────┘ │
                                        │                                          │
┌─────────────┐   tunnel                │  ┌─────────────────────────────────────┐ │
│ Cloudflare  │◀────────────────────────│  │  cloudflared                        │ │
│   Edge      │  t.pottingers.us:8080   │  │  - t.pottingers.us → :8080          │ │
└─────────────┘  webhook...:9000        │  │  - webhook.pottingers.us → :9000    │ │
       │                                │  └─────────────────────────────────────┘ │
       ▼                                │                    │                     │
   Internet                             │                    ▼                     │
                                        │  ┌─────────────────────────────────────┐ │
                                        │  │  Caddy (port 8080)                  │ │
                                        │  │  - serves /dist static files        │ │
                                        │  └─────────────────────────────────────┘ │
                                        │                                          │
                                        │  ┌─────────────────────────────────────┐ │
                                        │  │  /home/trevor/pub_musings/personal/ │ │
                                        │  │  └── dist/  (built static files)    │ │
                                        │  └─────────────────────────────────────┘ │
                                        └──────────────────────────────────────────┘
```

## Project Structure

```
pub_musings/
├── personal/                    # Astro website (this directory)
│   ├── src/
│   │   ├── pages/
│   │   │   ├── index.astro      # Home/About page
│   │   │   ├── blog/
│   │   │   │   ├── index.astro  # Blog listing
│   │   │   │   └── [slug].astro # Blog post template
│   │   │   └── contact.astro    # Contact form page
│   │   ├── content/
│   │   │   └── blog/            # Markdown blog posts
│   │   ├── components/
│   │   │   ├── ContactForm.tsx  # React component (client-side validation)
│   │   │   ├── Header.astro
│   │   │   └── Footer.astro
│   │   └── layouts/
│   │       └── BaseLayout.astro # Includes analytics snippet
│   ├── tests/
│   │   ├── unit/                # Vitest unit tests
│   │   └── e2e/                 # Playwright E2E tests
│   ├── astro.config.mjs
│   ├── playwright.config.ts
│   ├── vitest.config.ts
│   └── package.json
│
└── webhook-deployer/            # Go webhook service (separate project)
    ├── main.go
    ├── go.mod
    └── README.md
```

## Node Version

**Runtime**: Node.js 20 LTS (official Astro recommendation)

1. **Create `.nvmrc`** in `personal/`:
   ```
   20
   ```

2. **Local development** (you run these manually):
   ```bash
   nvm install 20
   nvm use 20
   npm install
   ```

3. **On the VM**:
   ```bash
   # Install nvm
   curl -o- https://raw.githubusercontent.com/nvm-sh/nvm/v0.40.0/install.sh | bash
   source ~/.bashrc

   # Install Node
   nvm install 20
   ```

4. **GitHub Actions** (for CI):
   ```yaml
   - uses: actions/setup-node@v4
     with:
       node-version-file: '.nvmrc'
   ```

## Configuration & Secrets

### Secrets Management with sops + age

We use **sops + age** for encrypted secrets in git. No external key management service required.

#### One-time setup

```bash
# Install (on Mac)
brew install age sops

# Install (on Ubuntu VM)
sudo apt install age
# sops: download from https://github.com/getsops/sops/releases

# Generate age key pair
mkdir -p ~/.config/sops/age
age-keygen -o ~/.config/sops/age/keys.txt
# Note the public key: age1abc123...
```

#### Create `.sops.yaml` in repo root

```yaml
creation_rules:
  - path_regex: \.enc\.yaml$
    age: age1abc123...  # your public key from above
```

#### Encrypt secrets

```bash
# Create secrets.yaml (never commit this file)
cat > secrets.yaml << 'EOF'
WEBHOOK_SECRET: "your-github-webhook-secret"
RESEND_API_KEY: "re_xxxxx"
EMAIL_FROM: "contact@pottingers.us"
EMAIL_TO: "your-email@example.com"
ALLOWED_ORIGIN: "https://t.pottingers.us"
PUBLIC_GA4_ID: "G-XXXXXXX"
PUBLIC_API_URL: "https://webhook.pottingers.us"
PUBLIC_SITE_URL: "https://t.pottingers.us"
EOF

# Encrypt it
sops -e secrets.yaml > secrets.enc.yaml
git add secrets.enc.yaml .sops.yaml
rm secrets.yaml  # delete plaintext
```

#### On the VM (decrypt)

```bash
# Copy age key to VM (one-time, secure transfer)
scp ~/.config/sops/age/keys.txt trevor@vm:~/.config/sops/age/keys.txt

# Decrypt secrets for webhook-deployer
cd ~/pub_musings
sops -d secrets.enc.yaml | \
  grep -E "^(WEBHOOK_SECRET|RESEND_API_KEY|EMAIL_FROM|EMAIL_TO|ALLOWED_ORIGIN):" | \
  sed 's/: /=/' | sed 's/"//g' > webhook-deployer/.env

# Decrypt secrets for personal Astro app
sops -d secrets.enc.yaml | \
  grep -E "^(PUBLIC_GA4_ID|PUBLIC_API_URL|PUBLIC_SITE_URL):" | \
  sed 's/: /=/' | sed 's/"//g' > personal/.env

# Verify the files
cat webhook-deployer/.env  # Should show WEBHOOK_SECRET, RESEND_API_KEY, etc.
cat personal/.env          # Should show PUBLIC_GA4_ID, PUBLIC_API_URL, PUBLIC_SITE_URL
```

**Note:** The `.env` files should use `KEY=value` format without quotes (unless the value contains spaces):
```bash
# webhook-deployer/.env format:
WEBHOOK_SECRET=your-secret-here
RESEND_API_KEY=re_xxxxx
EMAIL_FROM=contact@pottingers.us
EMAIL_TO=your-email@example.com
ALLOWED_ORIGIN=https://t.pottingers.us

# personal/.env format:
PUBLIC_GA4_ID=G-XXXXXXX
PUBLIC_API_URL=https://webhook.pottingers.us
PUBLIC_SITE_URL=https://t.pottingers.us
```

#### Key backup

- Store age private key in your password manager (1Password, Bitwarden)
- If key is lost, you'll need to re-encrypt secrets with a new key

### Required Secrets

| Secret | Used by | Purpose |
|--------|---------|---------|
| `WEBHOOK_SECRET` | webhook-deployer | Validate GitHub webhook signatures |
| `RESEND_API_KEY` | webhook-deployer | Send emails from contact form |
| `EMAIL_FROM` | webhook-deployer | Sender email address for contact form |
| `EMAIL_TO` | webhook-deployer | Recipient email address for contact form |
| `ALLOWED_ORIGIN` | webhook-deployer | CORS allowed origin (e.g., https://t.pottingers.us) |
| `PUBLIC_GA4_ID` | Astro frontend | Google Analytics tracking ID |
| `PUBLIC_API_URL` | Astro frontend | Backend API URL (e.g., https://webhook.pottingers.us) |
| `PUBLIC_SITE_URL` | Astro frontend | Site base URL (e.g., https://t.pottingers.us) |

## Implementation Steps

### Phase 1: Astro Project Setup

1. **Initialize Astro project** in `personal/`
   - `npm create astro@latest`
   - Configure for static output
   - Add React integration for interactive components

2. **Configure Astro** (`astro.config.mjs`)
   ```js
   export default defineConfig({
     output: 'static',
     integrations: [react()],
     site: 'https://t.pottingers.us',
   });
   ```

3. **Create base layout** with:
   - Cookie consent banner component
   - Conditional GA4 loading (only after consent)
   - Common header/footer

4. **Create pages**:
   - `/` — About me page
   - `/blog` — Blog listing with Content Collections
   - `/blog/[slug]` — Individual posts
   - `/contact` — Contact form (React component with client-side validation)

5. **Set up Content Collections** for blog posts
   - Define schema in `src/content/config.ts`
   - Type-safe frontmatter validation

### Phase 2: Contact Form

1. **Create React ContactForm component**
   - Client-side validation
   - Honeypot field for spam prevention
   - Submit to Go backend endpoint at `https://webhook.pottingers.us/api/contact`

2. **Go backend endpoint** (in webhook-deployer service):
   - `POST /api/contact` endpoint
   - Validate honeypot field (reject if filled)
   - Rate limiting: 5 requests per IP per hour using `golang.org/x/time/rate`
   - Extract real IP from `CF-Connecting-IP` header (Cloudflare is in front)
   - Send email via **Resend API** (free tier: 100 emails/day)
   - Return JSON response

   **Rate limiter implementation** (in-memory, per-IP):
   ```go
   import "golang.org/x/time/rate"

   type visitor struct {
       limiter  *rate.Limiter
       lastSeen time.Time
   }
   var visitors = make(map[string]*visitor)

   func getVisitor(ip string) *rate.Limiter {
       // Returns rate.NewLimiter(rate.Every(12*time.Minute), 2)
       // = 5 requests per hour with burst of 2
   }

   func contactHandler(w http.ResponseWriter, r *http.Request) {
       ip := r.Header.Get("CF-Connecting-IP") // Real IP from Cloudflare
       if !getVisitor(ip).Allow() {
           http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
           return
       }
       // ... handle form
   }
   ```

### Phase 3: Analytics (GA4 + GDPR)

1. **Create CookieConsent component**
   - Shows banner on first visit
   - Stores preference in localStorage
   - Only loads GA4 after explicit consent

2. **GA4 integration**
   - Load gtag.js conditionally
   - Configure IP anonymization (default in GA4)
   - Set up Data Processing Agreement in GA admin

### Phase 4: Testing Setup

1. **Vitest for unit tests**
   ```bash
   npm install -D vitest @testing-library/react jsdom
   ```
   - Test React component logic
   - Test utility functions

2. **Playwright for E2E tests**
   ```bash
   npm install -D @playwright/test
   npx playwright install
   ```

   Key test scenarios:
   - Pages render without errors
   - Blog posts load from markdown
   - **JS components hydrate correctly** (critical for React components)
   - Contact form validation works
   - Cookie consent banner functions properly
   - GA4 only loads after consent

3. **Example E2E test for JS hydration**:
   ```ts
   test('contact form validates and submits', async ({ page }) => {
     await page.goto('/contact');
     // Wait for React hydration
     await page.waitForSelector('[data-hydrated="true"]');
     // Test form interaction
     await page.fill('#email', 'invalid');
     await page.click('button[type="submit"]');
     await expect(page.locator('.error')).toBeVisible();
   });
   ```

4. **npm scripts**:
   ```json
   {
     "test": "vitest",
     "test:e2e": "playwright test",
     "test:all": "vitest run && playwright test"
   }
   ```

### Phase 5: Webhook Deployer (Go Service)

Location: `pub_musings/webhook-deployer/`

1. **Core functionality**:
   - Listen on port 9000 (or configurable)
   - **Webhook endpoint** (`POST /webhook`):
     - Validate GitHub webhook signature (HMAC-SHA256)
     - On valid push to `main`: run deploy script
   - **Contact form endpoint** (`POST /api/contact`):
     - Accept form data (name, email, message)
     - Validate honeypot, rate limit by IP
     - Send email via SMTP or Resend API
   - Log all events

2. **Deploy script flow**:
   ```bash
   cd /home/trevor/pub_musings/personal
   git pull origin trunk
   npm ci
   npm run build
   # Caddy automatically serves new files from dist/
   ```

3. **Security**:
   - Validate `X-Hub-Signature-256` header
   - Only trigger on `push` events to `main` branch
   - Run with limited permissions (non-root user)

4. **systemd service** (`/etc/systemd/system/webhook-deployer.service`):
   ```ini
   [Unit]
   Description=GitHub Webhook Deployer
   After=network.target

   [Service]
   Type=simple
   User=trevor
   WorkingDirectory=/home/trevor
   ExecStart=/home/trevor/pub_musings/webhook-deployer/webhook-deployer
   EnvironmentFile=/home/trevor/pub_musings/webhook-deployer/.env
   Environment=SITE_PATH=/home/trevor/pub_musings/personal
   Restart=always

   [Install]
   WantedBy=multi-user.target
   ```

   **Important:** The `.env` file must be located at `/home/trevor/pub_musings/webhook-deployer/.env` (not in `personal/`) and must be readable by the `trevor` user:
   ```bash
   chmod 600 ~/pub_musings/webhook-deployer/.env
   chown trevor:trevor ~/pub_musings/webhook-deployer/.env
   ```

   After creating or modifying the service file:
   ```bash
   sudo systemctl daemon-reload
   sudo systemctl enable webhook-deployer
   sudo systemctl start webhook-deployer
   sudo systemctl status webhook-deployer
   ```

### Phase 6: Caddy Configuration

1. **Install Caddy on Ubuntu**:
   ```bash
   # Install Caddy's repository
   sudo apt install -y debian-keyring debian-archive-keyring apt-transport-https curl
   curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/gpg.key' | sudo gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg
   curl -1sLf 'https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt' | sudo tee /etc/apt/sources.list.d/caddy-stable.list

   # Install
   sudo apt update
   sudo apt install caddy
   ```

   This automatically creates:
   - systemd service (`caddy.service`)
   - Caddyfile at `/etc/caddy/Caddyfile`
   - `caddy` user

2. **Caddyfile** (`/etc/caddy/Caddyfile`):
   ```
   :8080 {
       root * /home/trevor/pub_musings/personal/dist
       file_server
       encode gzip

       # Cache static assets
       @static path *.js *.css *.png *.jpg *.svg *.woff2
       header @static Cache-Control "public, max-age=31536000, immutable"

       # Don't cache HTML
       @html path *.html /
       header @html Cache-Control "no-cache"
   }
   ```

3. **Reload after changes**: `sudo systemctl reload caddy`

### Phase 7: Cloudflare Tunnel

1. **Install cloudflared on Ubuntu**:
   ```bash
   # Download and install
   wget https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-amd64.deb
   sudo dpkg -i cloudflared-linux-amd64.deb
   ```

2. **Create and configure tunnel**:
   ```bash
   # Login to Cloudflare (opens browser for authentication)
   cloudflared tunnel login

   # Create a new tunnel
   cloudflared tunnel create pottingers-us

   # Note the tunnel ID from output (e.g., "Created tunnel pottingers-us with id abc123...")
   # This creates ~/.cloudflared/<tunnel-id>.json credentials file
   ```

3. **Add tunnel routes** (requires DNS zone in Cloudflare):
   ```bash
   cloudflared tunnel route dns pottingers-us t.pottingers.us
   cloudflared tunnel route dns pottingers-us webhook.pottingers.us
   ```

   This creates CNAME records pointing to the tunnel.

4. **Update tunnel config** (`~/.cloudflared/config.yml`):
   ```yaml
   tunnel: <tunnel-id>
   credentials-file: /home/trevor/.cloudflared/<tunnel-id>.json

   ingress:
     - hostname: t.pottingers.us
       service: http://localhost:8080
     - hostname: webhook.pottingers.us
       service: http://localhost:9000
     - service: http_status:404
   ```

   Replace `<tunnel-id>` with your actual tunnel ID from step 2.

5. **Install and start cloudflared as a service**:
   ```bash
   sudo cloudflared service install
   sudo systemctl start cloudflared
   sudo systemctl enable cloudflared
   sudo systemctl status cloudflared
   ```

## Verification & Testing

### Local Development
```bash
cd personal/
npm run dev          # Start dev server on :4321
npm run test         # Run Vitest unit tests
npm run test:e2e     # Run Playwright E2E tests
npm run build        # Build static site
npm run preview      # Preview built site
```

### On the VM

**Before testing, build the site first:**
```bash
cd ~/pub_musings/personal
npm ci
npm run build
# Verify dist directory exists
ls -la dist/
```

**If you get permission errors with Caddy:**
```bash
# Ensure caddy user can read the dist directory
chmod -R +rx ~/pub_musings/personal/dist/
# Test access as caddy user
sudo -u caddy ls ~/pub_musings/personal/dist/
```

1. **Test Caddy serves files**: `curl http://localhost:8080`
   - **Expected**: HTML output from `dist/index.html`
   - **If 403 error**: The `dist/` directory doesn't exist or is empty. Build the site first (see above).
   - **If permission denied**: Check directory permissions (see above).

2. **Test tunnel**: `curl https://t.pottingers.us` from external network

3. **Test webhook**: Send test payload from GitHub webhook settings (URL: `https://webhook.pottingers.us/webhook`)

4. **Monitor logs**:
   - `journalctl -u caddy -f`
   - `journalctl -u webhook-deployer -f`
   - `journalctl -u cloudflared -f`

### CI Testing (optional GitHub Actions)
- Run `npm run test:all` on every PR
- Ensures tests pass before merge triggers deploy

## Files to Create

### In `personal/` (Astro site):
- [ ] `.nvmrc` — Node version (20)
- [ ] `package.json` — dependencies and scripts
- [ ] `astro.config.mjs` — Astro configuration
- [ ] `tsconfig.json` — TypeScript config
- [ ] `src/layouts/BaseLayout.astro` — base layout with analytics
- [ ] `src/pages/index.astro` — about page
- [ ] `src/pages/contact.astro` — contact page
- [ ] `src/pages/blog/index.astro` — blog listing
- [ ] `src/pages/blog/[...slug].astro` — blog post template
- [ ] `src/content/config.ts` — content collection schema
- [ ] `src/components/ContactForm.tsx` — React contact form
- [ ] `src/components/CookieConsent.tsx` — GDPR consent banner
- [ ] `src/components/Analytics.astro` — GA4 loader
- [ ] `vitest.config.ts` — Vitest configuration
- [ ] `playwright.config.ts` — Playwright configuration
- [ ] `tests/e2e/pages.spec.ts` — E2E page tests
- [ ] `tests/e2e/contact-form.spec.ts` — Contact form tests

### In repo root (`pub_musings/`):
- [ ] `.sops.yaml` — sops encryption rules
- [ ] `secrets.enc.yaml` — encrypted secrets (committed)

### In `webhook-deployer/` (Go service):
- [ ] `main.go` — HTTP server, routing
- [ ] `webhook.go` — GitHub webhook handler
- [ ] `contact.go` — contact form handler + Resend email
- [ ] `ratelimit.go` — per-IP rate limiting
- [ ] `go.mod` — Go module definition
- [ ] `deploy.sh` — deployment script
- [ ] `README.md` — setup instructions

### On the VM (not in repo):
- [ ] `/etc/caddy/Caddyfile`
- [ ] `/etc/systemd/system/webhook-deployer.service`
- [ ] `~/.cloudflared/config.yml` — tunnel config with t.pottingers.us and webhook.pottingers.us
- [ ] `~/.config/sops/age/keys.txt` — age private key (copied securely)
- [ ] `/home/trevor/pub_musings/personal/.env` — decrypted secrets
