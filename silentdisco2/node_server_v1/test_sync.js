// test_sync.js
// Thu Dec 26 07:43:15 PST 2024
// initialized with the help of chatgpt

const puppeteer = require('puppeteer');

(async () => {
  // Launch a new browser instance
  const browser = await puppeteer.launch({ headless: false }); // Set to 'true' to run headless
  const page = await browser.newPage();

  try {
    // Navigate to the page
    const url = 'http://localhost:8080/party/000000'; // TODO Replace with your actual URL
    await page.goto(url, { waitUntil: 'networkidle2' });

    await page.waitForFunction(() => {
      return [...document.querySelectorAll('button')].length > 2;
    });

    // Find the first button with textContent matching "play"
    const buttonSelector = 'button';
    await page.waitForSelector(buttonSelector); // Wait for any button to appear

    const buttonHandle = await page.evaluateHandle(() => {
      // Query all button elements on the page
      const buttons = Array.from(document.querySelectorAll('button'));
      // Find the first button with textContent strictly matching "play"
      return buttons.find(button => button.textContent.trim() === '▶️') || null;
    });

    if (!buttonHandle) {
      throw new Error('Button with text "play" not found.');
    }

    // TODO clone git repo and play audio from the beginning instead
    // TODO record audio from mic
    // TODO fetch audio here in the test

    // Click the button
    await buttonHandle.click();
    console.log('Button with text "play" clicked successfully!');
  } catch (error) {
    console.error('Error occurred:', error);
  } finally {
    const waitTime = 10000; // 10 seconds
    await new Promise(resolve => setTimeout(resolve, waitTime));

    // Close the browser
    await browser.close();
  }

  // TODO compare the audio from mic from fetched audio
})();

