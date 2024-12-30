// test_sync.js
// Thu Dec 26 07:43:15 PST 2024
// initialized with the help of chatgpt

const assert = require('assert');
const child_process = require('child_process');
const fs = require('fs');
const fs_promises = require('fs/promises');
const os = require('os');
const path = require('path');
const process = require('process');

// node-record-lpcm16 requires `brew install sox` on mac
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

const browserPlay = async (url, browser) => {
  const page = await browser.newPage();
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
};

const gitPlay = async (url, dstDir) => {
  const cwd = process.cwd();
  // lazy, chdir so I don't need to path.join(dstDir, ...) everywhere
  process.chdir(dstDir);
  // trim is to remove the trailing newline
  const objectsStr = fs.readFileSync('objects.txt').toString().trim();
  console.log(`gitPlay ${objectsStr.split('\n')}`);
  const objects = objectsStr.split('\n').map(JSON.parse);
  assert.ok(objects.length > 0, 'need at least one object to play');

  const nowInSec = (new Date()).getTime() / 1000.0;
  // (play/pause, index, object, object time offset, wall time
  const nowPlayingStr = `(play, 0, ${objects[0]['sha256']}.${objects[0]['filetype']}, 0, ${nowInSec})`;
  fs.writeFileSync('now_playing.txt', nowPlayingStr);
  child_process.execSync(`git add now_playing.txt`);
  child_process.execSync(`git commit -m 'testing from beginning'`);
  child_process.execSync(`git push origin trunk`);

  // return the current working directory back to original
  process.chdir(cwd);
};

// TODO reduce duplication with gitPlay
const gitPause = async (url, dstDir) => {
  const cwd = process.cwd();
  // lazy, chdir so I don't need to path.join(dstDir, ...) everywhere
  process.chdir(dstDir);
  // trim is to remove the trailing newline
  const objectsStr = fs.readFileSync('objects.txt').toString().trim();
  console.log(`gitPause ${objectsStr.split('\n')}`);
  const objects = objectsStr.split('\n').map(JSON.parse);
  assert.ok(objects.length > 0, 'need at least one object to pause');

  const nowInSec = (new Date()).getTime() / 1000.0;
  // (play/pause, index, object, object time offset, wall time
  const nowPlayingStr = `(pause, 0, ${objects[0]['sha256']}.${objects[0]['filetype']}, 10, ${nowInSec})`;
  fs.writeFileSync('now_playing.txt', nowPlayingStr);
  child_process.execSync(`git add now_playing.txt`);
  child_process.execSync(`git commit -m 'testing from beginning'`);
  child_process.execSync(`git push origin trunk`);

  // return the current working directory back to original
  process.chdir(cwd);
};


// Expects `url` to be like http://localhost:8080/party/000000 which can be easily
// formatted as http://localhost:8080/party/000000.git
const test = async (url) => {
  const tmpDir = await fs_promises.mkdtemp(path.join(os.tmpdir(), 'test_sync_'));
  const dstDir = path.join(tmpDir, '000000');
  console.log(`test_sync temp dir: ${tmpDir}`);
  // TODO use isomorphic-git instead of child_process
  child_process.execSync(`git clone ${url}.git ${dstDir}`);

  // Launch a new browser instance
  const browser = await puppeteer.launch({ headless: false }); // Set to 'true' to run headless

  // TODO fetch audio here in the test
  // assuming objects sha256 9f033b2cf7176e5c18d9694103ac7ca9cbdad1a70d02a96648850690e9760542,
  // the original wav is just e_J14fbBluE.mp3
  const file = fs.createWriteStream('test.wav', { encoding: 'binary' })
  const recording = recorder.record({
    sampleRate: 44100,
  });
  recording.stream().pipe(file);

  try {
    // await browserPlay(url, browser);
    await gitPlay(url, dstDir);
  } catch (error) {
    console.error('Error occurred:', error);
  } finally {
    const waitTime = 10000; // 10 seconds
    await new Promise(resolve => setTimeout(resolve, waitTime));
    recording.stop();

    // Close the browser
    await gitPause(url, dstDir);
    await browser.close();
  }

  // TODO compare the audio from mic from fetched audio
};

main();
