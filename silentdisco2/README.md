# silent disco v2

Oops

Ran `npm install create-react-app`

Then `./node_modules/.bin/create-react-app silentdisco`

Then `cd silentdisco` and then `npm start`. This will effectively auto `open http://localhost:3000`

Alternatively, run `npm run build` and `python3 -m http.server --directory build` and then you can `open http://localhost:8000`

I tried `npm test`

# Test data

I used `youtube_dl` to fetch https://www.youtube.com/watch?v=e_J14fbBluE

Then I ran `ffmpeg -n -i Cello\ Suite\ -\ Bach\ \[COPYRIGHT\ FREE\]-e_J14fbBluE.mp4 e_J14fbBluE.mp3` to get just the mp3

At this point, running `npm start` was mostly usable, but it wasn't loading the mp3 <audio>. It wasn't playable. My guess is that the webserver that `npm start` runs doesn't actually serve static content, which makes sense...

So I ran the `npm run build` commands from above. I had to `cp src/e_J14fbBluE.mp3 build/` to make it work, but then my browser was happy.

Notes:
* `document.getElementsByTagName('audio')[0].currentTime` is a floating point number. Includes six digits of precision (microseconds?!)...
* `(new Date()).getTime() / 1000` is a floating point number. Seconds since unix epoch (jan 1st, 1970). Three digits of precision (milliseconds).
* Apparently `document.getElementsByTagName('audio')[0].play()` may raise an exception: `NotAllowedError`. It also returns a promise, so needs to be await'ed. https://developer.mozilla.org/en-US/docs/Web/API/HTMLMediaElement/play
