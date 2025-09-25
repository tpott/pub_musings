// From https://github.com/steveseguin/ninjasdk?tab=readme-ov-file#audiovideo-example
// This doesn't actually work for my purpose because node.js (server-side JS) doesn't
// support the `navigator.mediaDevices...` code below.

import VDONinjaSDK from '@vdoninja/sdk/node';

const vdo = new VDONinjaSDK({
    salt: "vdo.ninja"  // Required for streams to be viewable on https://vdo.ninja
});

// Handle incoming tracks
vdo.addEventListener('track', (event) => {
    const video = document.getElementById('video');
    if (!video.srcObject) {
        video.srcObject = new MediaStream();
    }
    video.srcObject.addTrack(event.detail.track);
});

// Get user media
const stream = await navigator.mediaDevices.getUserMedia({
    video: true
});

// Connect, join room, and publish
await vdo.connect();
await vdo.joinRoom({ 
    room: "trevorsroom",
    password: "81f39fb583416e28181ee73559f5841f3ab24ec80d58b434a3442ab2f9c742ea",
});
await vdo.publish(stream, { room: "videoroom" });

// Your stream will be viewable at: https://vdo.ninja/?view=YOUR_STREAM_ID
