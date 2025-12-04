# Initial Plan: Schwab Login Automation with Puppeteer

## Overview
Create a TypeScript-based Puppeteer script that automates navigation to Schwab, waits for manual authentication, and clicks on a specific account. The account name and number ending are configurable via command line arguments, with defaults of "Investor Savings" and "0000".

## Technology Stack
- **Runtime**: Bun (v1.0.0+)
- **Language**: TypeScript with ES modules
- **Automation**: Puppeteer
- **Session Persistence**: Custom Chrome user data directory

## Files to Create

### 1. package.json
Create a package.json with:
- Bun as the package manager and runtime
- ES module support (`"type": "module"`)
- Puppeteer dependency (v23.10.4)
- Type definitions for Node.js and Bun
- Start script to run the TypeScript file directly
- Engine specification for Bun >= 1.0.0

Dependencies:
- `puppeteer`: ^23.10.4
- `@types/node`: ^22.10.1 (devDependency)
- `bun-types`: ^1.1.38 (devDependency)

Scripts:
- `start`: `bun run schwab-login.ts`

### 2. schwab-login.ts
Create the main automation script with the following structure:

**Imports:**
- Import puppeteer with Browser, Page, and ElementHandle types
- Import path utilities (join, dirname)
- Import fileURLToPath from url
- Define __dirname using import.meta.url for ES modules

**Types and Interfaces:**

1. `CliArgs` interface with:
   - `accountName: string`
   - `accountNumberEnding: string`

**Helper Functions (in this order):**

1. `parseArgs(): CliArgs`
   - Parse command line arguments from `process.argv.slice(2)`
   - First argument: account name (default: "Investor Savings")
   - Second argument: account number ending (default: "0000")
   - Return CliArgs object

2. `findAccountElement(page: Page, accountName: string, accountNumberEnding: string): Promise<ElementHandle | null>`
   - Use page.evaluateHandle to search all DOM elements
   - Find element containing both accountName and accountNumberEnding in textContent
   - Return null if not found
   - Check if result is null before returning

3. `findClickableParent(page: Page, element: ElementHandle): Promise<ElementHandle>`
   - Traverse up the DOM tree from element to document.body
   - Check if element is clickable (A, BUTTON, has onclick, or cursor is pointer)
   - Use explicit null checks: `while (current !== null && current !== document.body)`
   - Return first clickable parent or original element

4. `scrollIntoView(page: Page, element: ElementHandle): Promise<void>`
   - Scroll element into view with smooth behavior and center alignment

5. `clickAccount(page: Page, accountName: string, accountNumberEnding: string): Promise<boolean>`
   - Log search message
   - Wait 2000ms for page to fully render
   - Call findAccountElement
   - Use explicit check: `if (accountElement === null)`
   - If not found, log warning and return false
   - Find clickable parent
   - Scroll into view
   - Wait 500ms
   - Click the element
   - Wait 2000ms
   - Return true

**Main Function:**
- IIFE with `Promise<void>` return type
- Parse command line arguments using `parseArgs()`
- Destructure accountName and accountNumberEnding from result
- Log the target account information
- Set up userDataDir using join(__dirname, 'chrome-user-data')
- Launch browser with:
  - headless: false
  - userDataDir specified
  - args: `--user-data-dir=${userDataDir}`, `--no-sandbox`, `--disable-setuid-sandbox`
- Create new page with viewport 1280x800
- Navigate to 'https://www.schwab.com/client-home' with waitUntil: 'networkidle2'
- Print instructions for manual login (credentials + Symantec VIP)
- Wait for navigation to 'https://client.schwab.com/app/accounts/summary/' with 300000ms timeout
- Get current URL
- Use explicit check: `const isOnSummaryPage = currentUrl.includes('client.schwab.com/app/accounts/summary')`
- If `isOnSummaryPage === false`, log error and return early
- If on summary page, log success
- Try to click account with try-catch:
  - Call clickAccount(page, accountName, accountNumberEnding) using parsed values
  - Use explicit check: `if (success === true)`
  - Log current URL if successful
  - If not successful, log manual instruction
  - Catch errors and log them
- Log that browser will remain open
- Do NOT close browser automatically (leave commented out)

### 3. .gitignore
Create .gitignore with:
- node_modules/
- chrome-user-data/
- *.log
- .DS_Store

### 4. README.md
Create comprehensive README with sections:
- Project title and description
- Prerequisites (Bun v1.0.0+)
- Installation instructions (`bun install`)
- Usage instructions with CLI arguments:
  - Basic usage: `bun run start`
  - With custom account: `bun run schwab-login.ts "Account Name" "1234"`
  - Default values: "Investor Savings" and "0000"
- Script workflow explanation (5 steps)
- Command line arguments section explaining the two positional parameters
- Project structure
- Why Bun + TypeScript section (no build step, faster, type safety, simpler)
- Alternative Node.js instructions (optional)
- Configuration details (user data dir, timeout, viewport)
- Notes about session persistence and auto-close option

## Implementation Notes

### Code Style Requirements
- Use ES module imports, not require()
- Use explicit type annotations for variables (Browser, Page, ElementHandle, string, boolean)
- Use explicit conditionals (e.g., `=== null`, `=== false`, `=== true`) instead of truthy checks
- Extract complex logic into helper functions to avoid deeply nested code
- Keep try-catch blocks small and focused
- Use early returns to reduce nesting
- Use template strings for dynamic log messages

### File Organization
- Helper functions at the top, main IIFE at the bottom
- Clear separation of concerns
- Each helper function does one thing well

### TypeScript Features
- Use type imports with `type` keyword for type-only imports
- Explicit return types on all functions
- Proper ElementHandle typing from Puppeteer
- No any types except where necessary for evaluate callbacks

## Execution Order
1. Create package.json
2. Create schwab-login.ts with all helper functions and main logic
3. Create .gitignore
4. Create README.md

## Post-Creation Instructions
After creating all files, the user should run:
```bash
bun install
bun run start
```

The script will:
1. Launch Chrome with persistent session
2. Navigate to Schwab login
3. Wait for manual authentication
4. Automatically click on the specified account (or "Investor Savings" ending in "0000" by default)
5. Keep browser open for further interaction

CLI usage examples:
```bash
# Use defaults (Investor Savings / 0000)
bun run start

# Specify custom account
bun run schwab-login.ts "Special Savings" "0001"
```

## Quine

Run `claude -p "please implement 001_IMPLEMENTATION_PLAN.md"` in order to re-create this project from scratch. And please include this quine note in each future plan you write to ensure the project can be reliably re-created"`
