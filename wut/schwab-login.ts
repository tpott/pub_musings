import puppeteer, { type Browser, type Page, type ElementHandle } from 'puppeteer';
import { join, dirname } from 'path';
import { fileURLToPath } from 'url';

const __dirname = dirname(fileURLToPath(import.meta.url));

interface CliArgs {
  accountName: string;
  accountNumberEnding: string;
}

function parseArgs(): CliArgs {
  // process.argv format: [bun, script.ts, ...args]
  const args = process.argv.slice(2);

  const accountName = args[0] || 'Investor Savings';
  const accountNumberEnding = args[1] || '0000';

  return { accountName, accountNumberEnding };
}

async function findAccountElement(page: Page, accountName: string, accountNumberEnding: string): Promise<ElementHandle | null> {
  const element = await page.evaluateHandle((name, ending) => {
    const allElements = Array.from(document.querySelectorAll('*'));
    const targetElement = allElements.find(el => {
      const text = el.textContent || '';
      return text.includes(name) && text.includes(ending);
    });
    return targetElement || null;
  }, accountName, accountNumberEnding);

  // Check if we got a valid element
  const isNull = await element.evaluate(el => el === null);
  if (isNull) {
    return null;
  }

  return element as ElementHandle;
}

async function findClickableParent(page: Page, element: ElementHandle): Promise<ElementHandle> {
  return await page.evaluateHandle((el) => {
    let current = el as HTMLElement;
    while (current !== null && current !== document.body) {
      const isClickable =
        current.tagName === 'A' ||
        current.tagName === 'BUTTON' ||
        current.onclick !== null ||
        current.style.cursor === 'pointer' ||
        window.getComputedStyle(current).cursor === 'pointer';

      if (isClickable) {
        return current;
      }
      current = current.parentElement!;
    }
    return el;
  }, element) as Promise<ElementHandle>;
}

async function scrollIntoView(page: Page, element: ElementHandle): Promise<void> {
  await page.evaluate((el) => {
    (el as HTMLElement).scrollIntoView({ behavior: 'smooth', block: 'center' });
  }, element);
}

async function clickAccount(page: Page, accountName: string, accountNumberEnding: string): Promise<boolean> {
  console.log(`\nSearching for ${accountName} account ending in ${accountNumberEnding}...`);

  await page.waitForTimeout(2000);

  const accountElement = await findAccountElement(page, accountName, accountNumberEnding);

  if (accountElement === null) {
    console.log(`⚠ Could not find ${accountName} account ending in ${accountNumberEnding}`);
    return false;
  }

  console.log(`✓ Found ${accountName} account ending in ${accountNumberEnding}`);

  const clickableElement = await findClickableParent(page, accountElement);

  await scrollIntoView(page, clickableElement);
  await page.waitForTimeout(500);

  await clickableElement.click();
  console.log(`✓ Clicked on ${accountName} account`);

  await page.waitForTimeout(2000);
  return true;
}

(async (): Promise<void> => {
  // Parse command line arguments
  const { accountName, accountNumberEnding } = parseArgs();

  console.log(`Target account: ${accountName} ending in ${accountNumberEnding}`);

  // Set up a custom user data directory to persist session
  const userDataDir = join(__dirname, 'chrome-user-data');

  console.log('Launching browser...');
  const browser: Browser = await puppeteer.launch({
    headless: false, // Keep browser visible for manual login
    userDataDir: userDataDir,
    args: [
      `--user-data-dir=${userDataDir}`,
      '--no-sandbox',
      '--disable-setuid-sandbox'
    ]
  });

  const page: Page = await browser.newPage();

  // Set a reasonable viewport
  await page.setViewport({ width: 1280, height: 800 });

  console.log('Navigating to Schwab login page...');
  await page.goto('https://www.schwab.com/client-home', {
    waitUntil: 'networkidle2'
  });

  console.log('\n========================================');
  console.log('Please complete your login manually:');
  console.log('1. Enter your credentials');
  console.log('2. Complete Symantec VIP authentication');
  console.log('========================================\n');

  // Wait for navigation to the accounts summary page
  console.log('Waiting for login to complete...');
  await page.waitForNavigation({
    url: 'https://client.schwab.com/app/accounts/summary/',
    timeout: 300000 // 5 minute timeout for manual login
  }).catch(() => {
    // If exact URL doesn't match, check if we're at least on the summary page
    console.log('Checking if login was successful...');
  });

  const currentUrl: string = page.url();
  const isOnSummaryPage = currentUrl.includes('client.schwab.com/app/accounts/summary');

  if (isOnSummaryPage === false) {
    console.log('\nCurrent URL:', currentUrl);
    console.log('Login may not have completed. Please check the browser.');
    console.log('\nBrowser will remain open. Close it manually when done, or press Ctrl+C to exit.');
    return;
  }

  console.log('\n✓ Successfully logged in!');
  console.log('Current URL:', currentUrl);

  try {
    const success = await clickAccount(page, accountName, accountNumberEnding);

    if (success === true) {
      console.log('Current URL after click:', page.url());
    } else {
      console.log('Please manually click on the account');
    }
  } catch (error) {
    console.error('Error while searching for account:', error);
    console.log('You may need to manually click on the account');
  }

  // Keep browser open for inspection
  console.log('\nBrowser will remain open. Close it manually when done, or press Ctrl+C to exit.');

  // Uncomment the line below if you want the browser to close automatically
  // await browser.close();

})();
