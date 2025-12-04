# Schwab Login Automation

A simple Puppeteer script to automate Schwab login navigation with manual authentication.

## Setup

### Prerequisites

- **Bun** (v1.0.0 or higher) - [Install Bun](https://bun.sh/)

### Installation

```bash
bun install
```

This will install:
- `puppeteer` - Browser automation
- `@types/node` - Node.js type definitions
- `bun-types` - Bun runtime types

## Usage

### Basic Usage (Default Account)

```bash
bun run start
# or directly:
bun run schwab-login.ts
```

This will search for the default account: **"Investor Savings"** ending in **"0000"**

### Custom Account

```bash
bun run schwab-login.ts "Account Name" "1234"
```

**Arguments:**
1. **Account Name** (optional) - The name of the account to click (default: "Investor Savings")
2. **Account Number Ending** (optional) - The last digits of the account number (default: "0000")

**Examples:**
```bash
# Search for Special Savings account ending in 0001
bun run schwab-login.ts "Special Savings" "0001"

# Search for Individual account ending in 5678
bun run schwab-login.ts "Individual" "5678"

# Use default values
bun run schwab-login.ts
```

### Workflow

The script will:
1. Launch Chrome with a persistent user data directory
2. Navigate to Schwab login page
3. Wait for you to manually complete login + Symantec VIP
4. Detect when you reach the accounts summary page
5. Automatically search for and click on the specified account
6. Keep the browser open for further use

## Project Structure

- `schwab-login.ts` - Main script (TypeScript)
- `package.json` - Dependencies and scripts
- `chrome-user-data/` - Browser session persistence (gitignored)

## Why Bun + TypeScript?

- **No build step**: Run `.ts` files directly
- **Faster**: Bun is significantly faster than Node.js
- **Type safety**: Full TypeScript support with Puppeteer types
- **Simpler**: No need for `ts-node` or compilation

## Node.js Version (Alternative)

If you prefer Node.js instead of Bun:

```bash
npm install
npx ts-node schwab-login.ts
```

Update `package.json` engines:
```json
"engines": {
  "node": ">=18.0.0",
  "npm": ">=9.0.0"
}
```

## Configuration

### Command Line Arguments
- **Argument 1**: Account name (default: "Investor Savings")
- **Argument 2**: Account number ending (default: "0000")

### Browser Settings
- **User data directory**: `chrome-user-data/` (persists login sessions)
- **Login timeout**: 5 minutes (300000ms)
- **Viewport**: 1280x800

## Notes

- The browser stays open after login for manual inspection/use
- Session data is saved locally in `chrome-user-data/`
- Add `await browser.close()` at the end if you want auto-close
