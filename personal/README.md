# Personal Website

A personal website built with Astro, featuring a blog, contact form, and GDPR-compliant analytics.

## Development

```bash
# Install dependencies
npm install

# Start dev server
npm run dev

# Build for production
npm run build

# Preview production build
npm run preview
```

## Testing

```bash
# Unit tests
npm run test

# E2E tests (requires build first)
npm run build && npm run test:e2e

# All tests
npm run test:all
```

## Project Structure

- `src/pages/` - Astro pages (index, blog, contact)
- `src/layouts/` - Base layout with header, footer, analytics
- `src/components/` - Reusable components (React and Astro)
- `src/content/blog/` - Markdown blog posts
- `tests/` - Unit and E2E tests
- `public/` - Static assets

## Features

- Static site generation with Astro
- Blog with markdown content collections
- Contact form with client-side validation
- GDPR-compliant cookie consent for GA4 analytics
- Dark mode support via CSS media queries

## Deployment

See `001_INITIALIZATION.md` for the full deployment architecture with Caddy, Cloudflare Tunnel, and webhook-based auto-deploy.

## Sops

Once you have `sops` and `age` installed, you should be able to run `sops secrets.enc.yaml` to use $EDITOR to view and edit the decrypted version of secrets. Note that if you open the file directly without using `sops` then you will see the encrypted version of the secrets. You may need to add `export SOPS_AGE_KEY_FILE=...` to your `~/.zshrc` or `~/.bashrc` to make `sops` work out of the box.
