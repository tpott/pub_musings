// test_sync.js
// Thu Dec 26 07:43:15 PST 2024
// initialized with the help of chatgpt

const assert = require('assert');
const fs = require('fs');
const process = require('process');

const recorder = require('node-record-lpcm16');
const puppeteer = require('puppeteer');

const main = async () => {
  assert.ok(process.argv.length >= 3, 'expected `node test_sync.js $url`, got just two argv')
  let url = new URL(process.argv[2]);
  // const url = 'http://localhost:8080/party/000000'; // TODO Replace with your actual URL
  if (url.href.includes('/iamhost/')) {
    // TODO curl http://localhost:8080/iamhost/9bcd868ba298a5001e04221165dee5c0 and check
    // Set-Cookie host=$blah so that we can authentiate to the server
    url = new URL(url.origin + '/party/000000');
  }
  await test(url.href);
};

const test = async (url) => {
  // Launch a new browser instance
  const browser = await puppeteer.launch({ headless: false }); // Set to 'true' to run headless
  const page = await browser.newPage();

  // TODO clone git repo and play audio from the beginning instead
  // TODO record audio from mic
  // TODO fetch audio here in the test
  const file = fs.createWriteStream('test.wav', { encoding: 'binary' })
  const recording = recorder.record({
    sampleRate: 44100,
  });
  recording.stream().pipe(file);

  try {
    // Navigate to the page
    await page.goto(url, { waitUntil: 'networkidle2' });

    await page.waitForFunction(() => {
      return [...document.querySelectorAll('button')].length > 2;
    });

    // Find the first button with textContent matching 'play'
    const buttonSelector = 'button';
    await page.waitForSelector(buttonSelector); // Wait for any button to appear

    const buttonHandle = await page.evaluateHandle(() => {
      // Query all button elements on the page
      const buttons = Array.from(document.querySelectorAll('button'));
      // Find the first button with textContent strictly matching 'play'
      return buttons.find(button => button.textContent.trim() === '▶️') || null;
    });

    if (!buttonHandle) {
      throw new Error('Button with text "play" not found.');
    }

    // Click the button
    await buttonHandle.click();
    console.log('Button with text "play" clicked successfully!');
  } catch (error) {
    console.error('Error occurred:', error);
  } finally {
    const waitTime = 10000; // 10 seconds
    await new Promise(resolve => setTimeout(resolve, waitTime));
    recording.stop();

    // Close the browser
    await browser.close();
  }

  // TODO compare the audio from mic from fetched audio
};

main();
