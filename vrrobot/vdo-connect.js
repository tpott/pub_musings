import puppeteer from 'puppeteer-core';

console.log(puppeteer);

////////////////////
// CREATE A NEW ROOM
////////////////////

// const domain = 'vdo.ninja';
// Visit https://${domain}/
// Click "Create a Room", i.e. `document.getElementById('container-1').click()`
// Check "The guests can see the director, but not other guests' videos", i.e. `document.getElementById('broadcastFlag').click()`
// Check "The director will be performing as well, appearing in group scenes", i.e. `document.getElementById('showdirectorFlag').click()`
// Room name: `crypto.randomBytes(4).toString('hex')`
// Password: `crypto.randomBytes(32).toString('hex')`
// Click `document.getElementsByClassName('gowebcam')[0]` (index 0 is probably right, its the first button on the page) to create the room

// TODOs:
// * [ ] if robot computer is _joining_ an existing room, then the room probably needs to allow guests to share their video
// * [ ] the room's bitrate probably needs to be higher to allow higher quality video from the robot computer

////////////////////////
// JOIN AN EXISTING ROOM
////////////////////////

// const domain = 'vdo.ninja';
// const room = 'd4c26ea4';
// const password = '0ff00c24b011cc08a6fb432cf798e60779805afb9f44484386fd26d77bc333b5';
// Visit https://${domain}/?room=${room}&password=${password}
// Click "Join Room with Camera", i.e. `document.getElementById('container-3').click()`
// TODO select Video Source
// Click "START", i.e. `document.getElementById('gowebcam').click()`

async function joinVdoRoomImpl(browser, options) {
    const {
        domain = 'vdo.ninja',
        room,
        password,
        executablePath,
        headless = false,
        videoSource = null
    } = options;

    const page = await browser.newPage();
        
    // Grant camera and microphone permissions
    const context = browser.defaultBrowserContext();
    await context.overridePermissions(`https://${domain}`, [
        'camera',
        'microphone'
    ]);

    // Navigate to the VDO.Ninja room
    const url = `https://${domain}/?room=${room}&password=${password}`;
    console.log(`Navigating to: ${url}`);
    
    await page.goto(url, { 
        waitUntil: 'networkidle2',
        timeout: 30000 
    });

    // Wait for the page to load and the join button to be available
    console.log('Waiting for join button...');
    await page.waitForSelector('#container-3', { 
        visible: true, 
        timeout: 15000 
    });

    // Click "Join Room with Camera"
    console.log('Clicking "Join Room with Camera"...');
    await page.click('#container-3');

    // TODO Wait for the camera selection interface to load

    // TODO: Select video source if specified
    if (videoSource) {
        console.log(`Attempting to select video source: ${videoSource}`);
        try {
            // This is a placeholder - actual implementation would depend on 
            // VDO.Ninja's UI structure for video source selection
            await page.waitForSelector('select[id*="video"], select[id*="camera"]', { 
                timeout: 5000 
            });
            
            // Example: Select video source from dropdown
            await page.select('select[id*="video"], select[id*="camera"]', videoSource);
            console.log(`Selected video source: ${videoSource}`);
        } catch (error) {
            console.warn(`Could not select video source: ${error.message}`);
        }
    }

    // Wait for the START button to be available
    console.log('Waiting for START button...');
    await page.waitForSelector('#gowebcam', { 
        visible: true, 
        timeout: 15000 
    });
    await page.waitForFunction("document.getElementById('gowebcam').textContent.includes('START')");

    // Click "START"
    console.log('Clicking "START"...');
    await page.click('#gowebcam');

    // TODO Wait for the stream to start

    console.log('Successfully joined VDO.Ninja room!');
    
    return { browser, page };
}


/**
 * Joins a VDO.Ninja room with camera
 * @param {Object} options - Configuration options
 * @param {string} options.domain - VDO.Ninja domain (default: 'vdo.ninja')
 * @param {string} options.room - Room ID
 * @param {string} options.password - Room password
 * @param {string} options.executablePath - Path to Chrome/Chromium executable
 * @param {boolean} options.headless - Run in headless mode (default: false)
 * @param {string} options.videoSource - Optional video source to select
 */
async function joinVdoRoom(options) {
    const {
        room,
        password,
        executablePath,
        headless = false,
    } = options;

    // Validate required parameters
    if (!room || !password) {
        throw new Error('Room ID and password are required');
    }

    if (!executablePath) {
        throw new Error('executablePath is required for puppeteer-core');
    }

    const browser = await puppeteer.launch({
        executablePath,
        headless,
        args: [
            '--use-fake-ui-for-media-stream', // Allow camera access without user interaction
            // '--use-fake-device-for-media-stream', // Use fake camera/mic
            '--disable-web-security', // Disable web security for media access
            '--allow-running-insecure-content',
            '--disable-features=VizDisplayCompositor'
        ]
    });

    try {
        return await joinVdoRoomImpl(
            browser,
            options,
        );
    } catch (error) {
        console.error('Error joining VDO.Ninja room:', error.message);
        await browser.close();
        throw error;
    }

    return { browser: null, page: null };
}

joinVdoRoom({
    domain: 'vdo.ninja',
    room: 'd4c26ea4',
    password: '0ff00c24b011cc08a6fb432cf798e60779805afb9f44484386fd26d77bc333b5',
    executablePath: '/Applications/Google Chrome.app/Contents/MacOS/Google Chrome',
    headless: false,
});
