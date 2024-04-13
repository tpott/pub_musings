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

# Server implementation

Note: the silentdisco/ dir contains the react app. I created node_server_v1 for an express server, since I can eventually re-use the [isomorphic-git](https://isomorphic-git.org/) library on both the server and react client app. It should also be easy to add websockets. I'm not sure how I will be able to compile/transpile it to a Java server though for running on native Android...

I tried to scope `npm install express` to `silent_disco2/node_server_v1/`, but it updated `silentdisco2/package.json` instead... :shrug:

I ran `pushd node_server_v1 && cp -R ../silentdisco/build/* build`.. And then reverted that to just use ../silentdisco/build/ directly.

I tried to have `npm run build` output two separate files, i.e. party_list.html and party.html instead of index.html, but it turns out to be [required by react-scripts](https://github.com/facebook/create-react-app/blob/0a827f69ab0d2ee3871ba9b71350031d8a81b7ae/packages/react-scripts/scripts/start.js#L50). So I need to rethink this a bit.

Reverting some of the renaming, deleting App.css, I had to `npm install react-router-dom`, and things seem to be okay again.

My new workflow is working out of `silentdisco/` and iterating on react while running `npm run start`. Once that seemed okay, I'd run `npm run build && node ../node_server_v1/server.js`. This is working nicely, but now I want the party list to actually fetch a list of existing parties from the server.

# Flight notes

I was able to get the server setup to track parties. The build command is clearing the build/e_J14fbBluE.mp3 file. So I need to run `npm run build && cp src/e_J14fbBluE.mp3 build/ && node ../node_server_v1/server.js` to keep that file available.

I learned that `node something.js --inspect` doesn't work, but `node --inspect something.js` does... My iteration command is now `npm run build && cp src/e_J14fbBluE.mp3 build/ && node --inspect ../node_server_v1/server.js`. I also forgot that vanilla node isn't actually typescript...
