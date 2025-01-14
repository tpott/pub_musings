const puppeteer = require('puppeteer');

/**
 * hostUrl should be like new URL("http://localhost:8080/iamhost/ced582910d4ce706cd629839a8621b77")
 */
async function measureAudioSync(hostUrl) {
  const browser = await puppeteer.launch({
    headless: false, // Set to false if you want to see the browsers open
  });

  // login as a host
  const page1 = await browser.newPage();
  await page1.goto(hostUrl.toString(), { waitUntil: 'networkidle2' });

  // this is stupid, but this is how we get cookies
  await page1.goto(`${hostUrl.origin}/party/000000`, { waitUntil: 'networkidle2' });

  // go home
  // let button = await page1.waitForSelector('button ::-p-text(Leave Party)');
  // await button.click();
  // now it should have a Create New Party button
  // button = await page1.waitForSelector('button ::-p-text(Create New Party)');
  // console.assert(button.length > 0, 'Couldnt find host new party button');
  // await button.click();

  await page1.waitForNetworkIdle();

  // Define URLs for the audio content
  const audioUrl2 = page1.url();

  // Open two pages
  const page2 = await browser.newPage();
  await page2.goto(audioUrl2, { waitUntil: 'networkidle2' });
  console.log('goto done');
  // await page1.waitForSelector('p ::-p-text(Commit: )');
  await page1.waitForFunction(() => {
    return [...document.querySelectorAll('p')]
      .map(el => el.innerText)
      .filter(txt => txt.startsWith('Commit: ') && txt.length > 8)
      .length > 0;
  });

  // Start audio playback in both pages
  let wut = await page1.evaluate(() => {
    const audio = document.querySelector('audio');
    return Promise.resolve(audio);
    // audio.play();
  });
  console.log(wut);

  await page2.evaluate(() => {
    const audio = document.querySelector('audio');
    console.log(audio);
    // audio.play();
  });
  console.log('evaluate done');

  // Give the audio some time to play
  await new Promise(resolve => setTimeout(resolve, 3000)); // Wait 3 second

  // Measure playback time on both pages
  /*
  const playbackTime1 = await page1.evaluate(() => {
    const audio = document.querySelector('audio');
    return audio.currentTime;
  });

  const playbackTime2 = await page2.evaluate(() => {
    const audio = document.querySelector('audio');
    return audio.currentTime;
  });

  // Measure the difference in playback times
  const timeDifference = Math.abs(playbackTime1 - playbackTime2);
  console.log(`Time difference between the two audio playbacks: ${timeDifference} seconds`);
  */

  // Optionally, close the browser
  await browser.close();
}

if (process.argv.length < 3) {
  throw new Error('Need host URL argument');
}
measureAudioSync(new URL(process.argv[2]));
