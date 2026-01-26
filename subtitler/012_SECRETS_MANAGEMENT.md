# 012: Secrets Management with sops + age

## Overview

This document describes the secrets management workflow for the Subtitler project using **sops** (Secrets Operations) and **age** (simple, modern encryption tool). This approach allows us to:

- Store encrypted secrets in git (safe to commit)
- Decrypt secrets on deployment machines (VM, CI/CD)
- Avoid external key management services (self-contained)
- Share secrets securely with team members

## Architecture

```
Developer Machine:
  1. Create secrets.yaml (plaintext)
  2. Encrypt with sops → secrets.enc.yaml
  3. Commit secrets.enc.yaml to git
  4. Delete secrets.yaml

Production VM:
  1. Pull code from git (includes secrets.enc.yaml)
  2. Decrypt with sops → .env files
  3. Services read from .env files
```

## Files

| File | Location | Purpose | Committed? |
|------|----------|---------|------------|
| `.sops.yaml` | `/pub_musings/` | sops configuration (age public key) | ✅ Yes |
| `secrets.yaml.template` | `/subtitler/` | Template showing required secrets | ✅ Yes |
| `secrets.yaml` | `/subtitler/` | Plaintext secrets (temporary) | ❌ Never |
| `secrets.enc.yaml` | `/subtitler/` | Encrypted secrets | ✅ Yes |
| `keys.txt` | `~/.config/sops/age/` | age private key | ❌ Never |

## One-Time Setup

### Install Tools

**On Mac:**
```bash
brew install age sops
```

**On Ubuntu/Debian:**
```bash
# Install age
sudo apt install age

# Install sops
curl -LO https://github.com/getsops/sops/releases/download/v3.11.0/sops-v3.11.0.linux.amd64
chmod +x sops-v3.11.0.linux.amd64
sudo mv sops-v3.11.0.linux.amd64 /usr/local/bin/sops
```

### Generate age Key Pair

**First time only** (skip if you already have age keys):

```bash
# Create age key directory
mkdir -p ~/.config/sops/age

# Generate key pair
age-keygen -o ~/.config/sops/age/keys.txt

# Output will show:
# Public key: age1abc123... (this is your public key)
# Age identity (private key) written to ~/.config/sops/age/keys.txt
```

**Important:**
- The public key is already in `/pub_musings/.sops.yaml`: `age1dlv4emz589e2r7fyrudstaxw787as9dcpv0a93hg6d2n9p7tdexsvn5w7v`
- If you generated a new key pair, you need to update `.sops.yaml` with your new public key
- Store the private key (`~/.config/sops/age/keys.txt`) in your password manager (backup!)

### Verify Setup

```bash
# Check sops can find your age key
sops --version

# Try decrypting the existing secrets file (if you have the private key)
cd ~/pub_musings/subtitler
sops -d secrets.enc.yaml
```

## Workflow: Updating Secrets

### 1. Create Plaintext Secrets File

```bash
cd ~/pub_musings/subtitler

# Copy template
cp secrets.yaml.template secrets.yaml

# Edit with real values
nano secrets.yaml
```

**Example secrets.yaml:**
```yaml
# GitHub Webhook
WEBHOOK_SECRET: "your-actual-webhook-secret"

# Backend JWT
JWT_SECRET: "your-actual-jwt-secret"

# Email (Resend API)
RESEND_API_KEY: "re_your_actual_api_key"
EMAIL_FROM: "noreply@yourdomain.com"

# URLs
FRONTEND_URL: "https://subtitler.yourdomain.com"
PUBLIC_API_URL: "https://api.subtitler.yourdomain.com"
```

### 2. Encrypt Secrets

```bash
# Encrypt secrets.yaml → secrets.enc.yaml
sops -e secrets.yaml > secrets.enc.yaml

# Verify encryption worked
head -5 secrets.enc.yaml
# Should show encrypted data like:
# WEBHOOK_SECRET: ENC[AES256_GCM,data:...,iv:...,tag:...,type:str]
```

### 3. Commit Encrypted File

```bash
# Add encrypted file to git
git add secrets.enc.yaml

# IMPORTANT: Delete plaintext file (never commit it!)
rm secrets.yaml

# Commit
git commit -m "Update encrypted secrets"
git push origin trunk
```

## Workflow: Decrypting on VM

### One-Time: Transfer Private Key to VM

```bash
# On your local machine
scp ~/.config/sops/age/keys.txt trevor@your-vm:~/.config/sops/age/keys.txt

# On the VM
chmod 600 ~/.config/sops/age/keys.txt
```

### Decrypt Secrets to .env Files

**Backend .env:**
```bash
cd ~/pub_musings/subtitler/backend

# Decrypt and extract backend secrets
sops -d ../secrets.enc.yaml | \
  grep -E "^(JWT_SECRET|RESEND_API_KEY|EMAIL_FROM|FRONTEND_URL):" | \
  sed 's/: /=/' | sed 's/"//g' > .env

# Set permissions
chmod 600 .env

# Verify
cat .env
# Should show:
# JWT_SECRET=your-actual-jwt-secret
# RESEND_API_KEY=re_your_actual_api_key
# EMAIL_FROM=noreply@yourdomain.com
# FRONTEND_URL=https://subtitler.yourdomain.com
```

**Frontend .env:**
```bash
cd ~/pub_musings/subtitler/frontend

# Decrypt and extract frontend secrets
sops -d ../secrets.enc.yaml | \
  grep -E "^PUBLIC_API_URL:" | \
  sed 's/: /=/' | sed 's/"//g' > .env

# Set permissions
chmod 600 .env

# Verify
cat .env
# Should show:
# PUBLIC_API_URL=https://api.subtitler.yourdomain.com
```

### Manual .env Creation (Alternative)

If you prefer to create `.env` files manually (without sops), you can:

**Backend:**
```bash
cat > backend/.env << 'EOF'
JWT_SECRET=your-jwt-secret
RESEND_API_KEY=re_your_api_key
EMAIL_FROM=noreply@yourdomain.com
FRONTEND_URL=https://subtitler.yourdomain.com
EOF
chmod 600 backend/.env
```

**Frontend:**
```bash
cat > frontend/.env << 'EOF'
PUBLIC_API_URL=https://api.subtitler.yourdomain.com
EOF
chmod 600 frontend/.env
```

## Integration with Deployment

The deployment script (`deploy-subtitler.sh`) assumes `.env` files already exist in `backend/` and `frontend/` directories. You must create these `.env` files **once** on the VM before the first deployment.

**Deployment flow:**
1. One-time setup: Transfer age key to VM
2. One-time setup: Decrypt secrets → .env files
3. Deploy: `./deploy-subtitler.sh` (reads from .env files)
4. Update secrets: Re-encrypt locally → push → re-decrypt on VM → restart service

## Security Best Practices

### DO:
- ✅ Commit `secrets.enc.yaml` to git (it's encrypted!)
- ✅ Store age private key in password manager
- ✅ Set `.env` file permissions to 600 (read/write for owner only)
- ✅ Rotate secrets regularly (JWT_SECRET, WEBHOOK_SECRET)
- ✅ Use different secrets for development/staging/production

### DON'T:
- ❌ Commit plaintext `secrets.yaml` to git
- ❌ Share age private key via email/Slack
- ❌ Use example/template values in production
- ❌ Store plaintext `.env` files in git
- ❌ Use the same secrets across multiple projects

## Troubleshooting

### Error: "no matching creation rules found"

**Cause:** sops can't find `.sops.yaml` or the output filename doesn't match the pattern.

**Solution:**
```bash
# Check .sops.yaml exists
cat .sops.yaml

# Ensure output filename ends with .enc.yaml
sops -e secrets.yaml > secrets.enc.yaml  # ✅ Correct
sops -e secrets.yaml > secrets.encrypted  # ❌ Wrong
```

### Error: "failed to load age identities"

**Cause:** age private key not found at `~/.config/sops/age/keys.txt`.

**Solution:**
```bash
# Check if key exists
ls -la ~/.config/sops/age/keys.txt

# If missing, generate new key pair
mkdir -p ~/.config/sops/age
age-keygen -o ~/.config/sops/age/keys.txt

# If generated new key, update .sops.yaml with new public key
```

### Error: "Failed to get the data key"

**Cause:** The age private key on this machine doesn't match the public key used for encryption.

**Solution:**
```bash
# Option 1: Transfer the correct private key from another machine
scp other-machine:~/.config/sops/age/keys.txt ~/.config/sops/age/keys.txt

# Option 2: Re-encrypt secrets.enc.yaml with your current key
# (Requires access to plaintext secrets)
sops -e secrets.yaml > secrets.enc.yaml
```

### Secrets not loading in backend

**Cause:** `.env` file missing or incorrect format.

**Solution:**
```bash
# Check .env file exists
ls -la backend/.env

# Check format (KEY=value, no quotes unless spaces)
cat backend/.env

# Re-create from encrypted file
cd backend
sops -d ../secrets.enc.yaml | grep -E "^JWT_SECRET:" | sed 's/: /=/' | sed 's/"//g'
```

## Required Secrets

### Backend (.env)

| Variable | Description | Example |
|----------|-------------|---------|
| `JWT_SECRET` | Secret for signing JWT tokens | `openssl rand -base64 32` |
| `RESEND_API_KEY` | Resend email API key | `re_xxxxx` from resend.com |
| `EMAIL_FROM` | Sender email address | `noreply@yourdomain.com` |
| `FRONTEND_URL` | Frontend URL (for CORS) | `https://subtitler.yourdomain.com` |

### Frontend (.env)

| Variable | Description | Example |
|----------|-------------|---------|
| `PUBLIC_API_URL` | Backend API URL | `https://api.subtitler.yourdomain.com` |

### Webhook Deployer (.env)

| Variable | Description | Example |
|----------|-------------|---------|
| `WEBHOOK_SECRET` | GitHub webhook secret | `openssl rand -base64 32` |

## Generating Secrets

```bash
# Generate strong random secrets
openssl rand -base64 32  # For JWT_SECRET, WEBHOOK_SECRET

# Get Resend API key
# Visit: https://resend.com/api-keys
```

## References

- [sops documentation](https://github.com/getsops/sops)
- [age documentation](https://github.com/FiloSottile/age)
- [Personal site secrets pattern](/home/trevor/pub_musings/personal/001_INITIALIZATION.md)
