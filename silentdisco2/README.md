# Silent Disco

A tool for hosting synchronized parties

Someone needs to run the party server. Ideally we get this to work as a
mobile app, so all someone needs is to bring their phone.

We probably want someone to hotspot, so everyone is on the same WiFi
network. This should minimize latency between all the devices and should
ensure the best synchronization possible.

# History

I ran `npm install create-react-app` and then `./node_modules/.bin/create-react-app party-web`

Next, I think `npm start` starts watching the react changes.

`npm run build` should build static files, at which point, we no longer need npm nor node.

# silent disco v2

Oops, I forgot to commit the above^... So the next few sections were re-done from scratch.

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

I learned that e.preventDefault() and e.stopPropagation() don't actually prevent playing/pausing of <audio> elements. Calling play() or pause() _works_ from the UX perspective, but may actually be incrementing playback.

`git upload-pack --advertise-refs .` is cool. TIL about [https://git-scm.com/docs/pack-protocol/2.2.3](https://git-scm.com/docs/pack-protocol/2.2.3) . I was also reading about `git http-backend`. I was having a hard time figuring out the correct env vars, and pipes...

I finally got `GIT_HTTP_EXPORT_ALL= GIT_PROJECT_ROOT=/tmp/silentdisco/e964a3e5e7c35043596841054b02900a/parties/000000/ PATH_INFO="/info/refs" QUERY_STRING="service=git-upload-pack" REQUEST_METHOD=GET git http-backend` to work. After it was working in server.js, I was also able to run `git clone http://localhost:8080/party/000000.git`.

I'm getting a weird "EmptyServerResponseError: Empty response from git server.", which seems to be from https://github.com/isomorphic-git/isomorphic-git/blob/545c8f128763cb2f76a831f69aee8745089c359b/src/wire/parseRefsAdResponse.js#L18 .

# Instrumenting git

I figured out that I could pipe and modify the output from `CONTENT_TYPE=application/x-git-upload-pack-request GIT_HTTP_EXPORT_ALL= GIT_PROJECT_ROOT=/Users/tpott/Github/pub_musings PATH_INFO="/info/refs" QUERY_STRING="service=git-upload-pack" REQUEST_METHOD=GET git http-backend` and then pipe that as stdin to `CONTENT_TYPE=application/x-git-upload-pack-request GIT_HTTP_EXPORT_ALL= GIT_PROJECT_ROOT=/Users/tpott/Github/pub_musings/ PATH_INFO="/git-upload-pack" QUERY_STRING="" REQUEST_METHOD=POST git http-backend`. But it comes back empty (headers + empty response)... And testing with `git clone ...` still yields nothing. It just infinite loops? My best guess is that express's body parser isn't parsing the POST body properly and therefore passing an empty body...

Bubbling back up, looking at the isomorphic-git error line, I think it's actually because the client doesn't have `Buffer`.

# Almost there

It's mostly working... But multi clients isn't working because the other clients can't seem to get their FS updated...

I'm trying to replicate `git clone http://localhost:8080/party/000000.git`
* `mkdir 000000 && cd 000000 && ~/Github/pub_musings/silentdisco2/node_modules/.bin/isogit clone --url=http://localhost:8080/party/000000.git --depth=1 --singleBranch`
* `~/Github/pub_musings/silentdisco2/node_modules/.bin/isogit fetch origin trunk`
* `cp ~/.gitconfig .git/config && ~/Github/pub_musings/silentdisco2/node_modules/.bin/isogit merge --ours=trunk --theirs=remotes/origin/trunk`

# Git is working

Git is now working! I think the hardest parts to figure out are done. To run the server, I typically run `cd silentdisco && npm run build && node --inspect ../node_server_v1/server.js`

Up next:
* [ ] User authentication. Need some sort of device keys, user names, and user
roles (listener, DJ, host). Every user name should be rendered with first
two (N) hex chars from the device key.
* [ ] Audio autoplay (loop?). And ideally drag to re-order (if you're the DJ).
* [ ] Delete / unschedule a song (if you're the DJ). Also maybe song name?
* [x] Websockets! We need websockets to make everything more responsive. Then reduce
the interval set for `asyncPullGit` and timeout set with `clickDelayMs`.
* [ ] Client side full state from fs parsing. Every function that calls `readFile`...
Will be able to handle scheduling in the past. And can delete rows from 
`now_playing.txt` once they're no longer relevant. Check an <audio> element's
`duration`.  Will need to set the "Now playing" banner text.
* [x] Cache what commit the FS is currently at? Eh not necessary because of the
`alradyMerged` flag. Actually, this will be good for flagging react to re-render

# Issues

* Playing one song while another is already playing. Its not correctly `.pause()`ing
the song that was playing first.
* The git commit ID on the NowPlaying.js component takes a long time to render.
* Loading the party doesn't load the current timestamp based on the last parsed
pause. Or last play with currentTime = target + diff (?).
* Idk if websockets are working correctly?
* File uploads always go to the `000000` party

I added the following to `/etc/nginx/nginx.conf` inside the `http` section...
Replace all $server_name manually

```
    server {
        listen 80;
        server_name $sever_name;
        return 301 https://$server_name$request_uri;
    }

    server {
       listen 443 ssl;
       server_name $server_name;

       ssl_certificate /etc/letsencrypt/live/$server_name/fullchain.pem;
       ssl_certificate_key /etc/letsencrypt/live/$server_name/privkey.pem;

       location /ws {
           proxy_pass http://localhost:8001;
           proxy_http_version 1.1;
           proxy_set_header Upgrade $http_upgrade;
           proxy_set_header Connection 'upgrade';
           proxy_set_header Host $host;
           proxy_cache_bypass $http_upgrade;
       }

       location / {
           proxy_pass http://localhost:8000;
           proxy_http_version 1.1;
           proxy_set_header Upgrade $http_upgrade;
           proxy_set_header Connection 'upgrade';
           proxy_set_header Host $host;
           proxy_cache_bypass $http_upgrade;
       }
   }
```

# Revisiting

After a long while back, I ran `npm update` for the server and client, which bumped a bunch of
package-lock.json bits. I can still run `cd silentdisco && npm run build && node --inspect ../node_server_v1/server.js`
so hopefully nothing is breaking...

# Testing

I'm running `node --inspect ../node_server_v1/server.js` in one terminal,
and `node node_server_v1/test_sync.js http://localhost:8080/iamhost/d7ff7332c6a617c153b75e27f836902e`
in another. Once that finishes, I run `open -a "Google Chrome" test.wav`.
