const child_process = require('child_process');
const util = require('util');

const crypto = require('crypto');
const fs = require('fs/promises');
const http = require('http');
const os = require('os');
const path = require('path');

const express = require('express');
const git = require('isomorphic-git');
const tweetnacl = require('tweetnacl');
const ws = require('ws');


// TODO run this file through ts-compile so we can include type hints here
function uint8ArrayToHex(arr /* Uint8Array */) /* string*/ {
  return [...arr].map(b => b.toString(16).padStart(2, '0')).join('');
}

function hexToUint8Array(str /* string */) /* Uint8Array */ {
  if (str.length % 2 !== 0) {
    throw new Error('Invalid hex string: length must be even');
  }
  const bytes = [];
  for (let i = 0; i < str.length; i += 2) {
    const byteString = str.substring(i, i + 2);
    bytes.push(parseInt(byteString, 16));
  }
  return new Uint8Array(bytes);
}

function textToUint8Array(str /* string */) /* Uint8Array */ {
  return Uint8Array.from(Array.from(str).map(letter => letter.charCodeAt(0)));
}

function isArrayEqual(arr1 /* Uint8Array */, arr2 /* Uint8Array */) {
  if (arr1.length !== arr2.length) {
    return false;
  }
  for (let i = 0; i < arr1.length; i++) {
    if (arr1[i] !== arr2[i]) {
      return false;
    }
  }
  return true;
}

// ------WebKitFormBoundary
// EgoUi9YxyjfwBaZU
// \r\n
// Content-Disposition: form-data; name="file"; filename="u6bk53x2Kno.mp3"
// \r\n
// Content-Type: audio/mpeg
// \r\n
// \r\n
// fileBytes
// \r\n
// ------WebKitFormBoundary
// EgoUi9YxyjfwBaZU
// \r\n
// Content-Disposition: form-data; name="party_id"
// \r\n
// \r\n
// 2fda55
// \r\n
// ------WebKitFormBoundary
// EgoUi9YxyjfwBaZU
// --
// \r\n

function tokenizeChunk(bytes /* Uint8Array */) /* Array<Uint8Array> */ {
  // \r\n
  const returnNewline = new Uint8Array([13, 10]);
  // --
  const dashDash = new Uint8Array([45, 45]);
  let ret = [];
  for (let i = 0; i < bytes.length && i >= 0; i = bytes.indexOf(returnNewline, i + 1)) {
    const count = ret.length;
    if (count >= 2 && isArrayEqual(ret[count - 1], returnNewline) &&
        isArrayEqual(ret[count - 2], returnNewline)) {
      // the form-data!
      ret.push(bytes.slice(i + 2, bytes.length - 2));
      // trailing \r\n
      ret.push(bytes.slice(bytes.length - 2));
      return ret;
    } else if (isArrayEqual(bytes.slice(i, i + 2), returnNewline)) {
      ret.push(bytes.slice(i, i + 2));
      // when there's a \r\n\r\n, bytes.slice(i + 2, i +2) will return the whole bytes
      const next = bytes.indexOf(returnNewline, i + 1);
      if (i + 2 === next) {
        ret.push(bytes.slice(i + 2, i + 4));
      } else {
        ret.push(bytes.slice(i + 2, bytes.indexOf(returnNewline, i + 1)));
      }
    } else if (isArrayEqual(bytes.slice(i, i + 2), dashDash)) {
      ret.push(bytes.slice(i, i + 2));
    } else {
      ret.push(bytes.slice(i, bytes.indexOf(returnNewline, i + 1)));
    }
  }
  return ret;
}

// ------WebKitFormBoundaryEgoUi9YxyjfwBaZU
// \r\nContent-Disposition: form-data; name="file"; filename="u6bk53x2Kno.mp3"
// \r\nContent-Type: audio/mpeg
// \r\n\r\nfileBytes
// \r\n
// ------WebKitFormBoundaryEgoUi9YxyjfwBaZU
// \r\nContent-Disposition: form-data; name="party_id"
// \r\n\r\n2fda55
// \r\n
// ------WebKitFormBoundaryEgoUi9YxyjfwBaZU
// --\r\n

function tokenizeBoundaries(bytes /* Uint8Array */) /* Array<Uint8Array> */ {
  // ------WebKitFormBoundary
  const boundary = new Uint8Array([45, 45, 45, 45, 45, 45, 87, 101, 98, 75, 105, 116, 70, 111, 114, 109, 66, 111, 117, 110, 100, 97, 114, 121]);
  let ret = [];
  for (let i = 0; i < bytes.length && i >= 0; i = bytes.indexOf(boundary, i + 1)) {
    // this should just be the literal boundary...
    ret.push(bytes.slice(i, i + boundary.length));
    // 16 bytes for the unique ID that follows the boundary
    ret.push(bytes.slice(i + boundary.length, i + boundary.length + 16));
    const end = bytes.indexOf(boundary, i + 1);
    let chunk = null;
    if (end >= 0) {
      chunk = bytes.slice(i + boundary.length + 16, end);
    } else {
      // pick up the trailing "--\r\n"
      chunk = bytes.slice(i + boundary.length + 16);
    }
    tokenizeChunk(chunk).forEach((token) => ret.push(token));
  }
  return ret;
}

function parseFileFromBody(bytes /* Uint8Array */) /* Uint8Array */ {
  const tokens = tokenizeBoundaries(bytes);

  var doodads = [{}];
  for (let i = 1; i < tokens.length; i++) {
    if (tokens[i].length <= 2) {
      continue;
    }
    if (tokens[i].length >= 1000) {
      continue;
    }
    // tokens[0] should always be the const boundary
    if (isArrayEqual(tokens[i], tokens[0])) {
      doodads[doodads.length - 1]['value'] = tokens[i - 2];
      doodads.push({});
      continue;
    }
    const token = (new TextDecoder()).decode(tokens[i]);
    // Content-Type: audio/mpeg
    if (token.startsWith('Content-Type')) {
      doodads[doodads.length - 1]['content-type'] = token.split(': ')[1];
      continue;
    }
    if (!token.startsWith('Content-Disposition: form-data')) {
      continue;
    }
    const subtokens = token.split('; ');
    for (let j = 1; j < subtokens.length; j++) {
      const parts = subtokens[j].split('=');
      doodads[doodads.length - 1][parts[0]] = parts[1].slice(1, -1);
    }
  }

  let ret = {};
  for (let i = 0; i < doodads.length; i++) {
    if (!('name' in doodads[i])) {
      continue;
    }
    ret[doodads[i]['name']] = doodads[i];
  }

  return ret;
}

// Note: this is Uint8Array([104, 111, 115, 116, 58, 116, 114, 117, 101])
const hostIsTrue = textToUint8Array('host:true');


async function main() {
  const app = express();
  const PORT = process.env.PORT || 8080;
  const WS_PORT = process.env.WS_PORT || 8081;

  const httpServer = http.createServer(app);
  const wss = new ws.Server({ server: httpServer });
  const clients = [];

  wss.on('connection', (conn) => {
    clients.push(conn);

    conn.on('message', (msg) => {
      console.log('got unexpected message', msg, conn);
    });

    conn.on('close', () => {
      // remove conn from clients
      clients.splice(clients.indexOf(conn), 1);
    });
  });

  const broadcast = (message) => {
    clients.forEach((client) => {
      if (client.readyState === WebSocket.OPEN) {
        client.send(message);
      }
    });
  };

  let randHostID = crypto.randomBytes(16).toString('hex');
  const { publicKey, secretKey } = tweetnacl.sign.keyPair();
  const publicKeyStr = uint8ArrayToHex(publicKey);

  const tmpDir = path.join(os.tmpdir(), 'silentdisco', randHostID);
  await fs.mkdir(
    tmpDir,
    { recursive: true },
  );
  await fs.mkdir(path.join(tmpDir, 'objects')); // i.e. audio file references
  await fs.mkdir(path.join(tmpDir, 'parties'));
  console.log(`Created tmpDir: ${tmpDir}`);

  // TODO when running `node --inspect ../node_server_v1/server.js` from the
  // silentdisco dir, this file isn't in the local path..
  await fs.copyFile(
    path.join(__dirname, 'e_J14fbBluE.mp3'), // __dirname should be $something/node_server_v1/
    path.join(tmpDir, 'objects', '9f033b2cf7176e5c18d9694103ac7ca9cbdad1a70d02a96648850690e9760542.mp3'),
  );

  async function shutDown() {
    httpServer.close();
    await fs.rm(tmpDir, { recursive: true });
    console.log('Shut down');
  }

  // TODO move this into a lib to share with Party.js
  const isValidPartyID = (partyID /*: string */) => {
    const hexPattern = /^[0-9A-Fa-f]{6}$/i;
    return partyID.length === 6 && hexPattern.test(partyID);
  };

  // processGetRequest gets GET and POST requests
  // service=git-upload-pack is for `git pull` and `git clone` (?)
  // service=git-receive-pack is for is for `git push`
  const processGitRequest = async (req, res, partyID) => {
    const urlParts = req.url.split('/');
    if (urlParts.length < 3) {
      res.redirect('/');
    }
    // urlParts[0] should equal the empty string
    if (urlParts[0] !== '') {
      res.redirect('/');
    }
    if (urlParts[1] !== 'party') {
      res.redirect('/');
    }
    // urlParts[2] should end in .git
    if (!urlParts[2].startsWith(partyID) || !urlParts[2].endsWith('.git')) {
      res.redirect('/');
    }

    const partyDir = path.join(tmpDir, 'parties', partyID);
    const command = 'git http-backend';
    const envVars = {
      GIT_HTTP_EXPORT_ALL: '',
      GIT_PROJECT_ROOT: partyDir,
      PATH_INFO: '/' + urlParts.slice(3).join('/').split('?')[0],
      QUERY_STRING: req.url.split('?')[1],
      REQUEST_METHOD: req.method,
    };

    // If someone is trying to push, git requires them to be authenticated
    // TODO loop in authentication
    if (envVars.QUERY_STRING === 'service=git-receive-pack' || envVars.PATH_INFO === '/git-receive-pack') {
      envVars.REMOTE_USER = 'TODO';
    }

    for (let i = 0; i < req.rawHeaders.length; i++) {
      const header = req.rawHeaders[i];
      if ((header.toLowerCase() !== 'content-type') || ((i + 1) === req.rawHeaders.length)) {
        continue;
      }
      // For example: 'application/x-git-upload-pack-request',
      envVars.CONTENT_TYPE = req.rawHeaders[i + 1];
    }

    let options = {
      env: envVars,
    };
    if (req.method === 'POST') {
      options.input = req.body;
    }

    const stdoutBytes = child_process.execSync(command, options);

    let headers = {};
    const returnNewline = new Uint8Array([13, 10]); // \r\n
    for (let i = 0; i < stdoutBytes.length && i >= 0; ) {
      const end = stdoutBytes.indexOf(returnNewline, i)

      // Use writeHead instead of setHeader because isomorphic-git can't handle ; charset=utf-8 in Content-Type
      // https://github.com/isomorphic-git/isomorphic-git/blob/545c8f128763cb2f76a831f69aee8745089c359b/src/managers/GitRemoteHTTP.js#L137
      // and send too... https://stackoverflow.com/questions/59449221/express-remove-charset-utf-8-from-content-type-application-json-charset-utf-8
      const nextEnd = stdoutBytes.indexOf(returnNewline, end + 2);
      if (nextEnd < 0) {
        res.writeHead(200, headers);
        res.write(stdoutBytes.slice(i + 2));
        res.end();
        if (envVars.PATH_INFO === '/git-receive-pack') {
          broadcast('please-pull');
        }
        return;
      }

      const line = (new TextDecoder()).decode(stdoutBytes.slice(i, end));
      if (line.startsWith('Status')) {
        console.log('oops, git set a status', line);
        break;
      }
      if (line.startsWith('Cache-Control') ||
          line.startsWith('Content-Type') ||
          line.startsWith('Expires') ||
          line.startsWith('Pragma')) {
        const header = line.split(': ');
        // just skip over the incorrectly formatted headers...
        if (header.length !== 2) {
          continue;
        }
        headers[header[0]] = header[1];
        i = end + 2;
        continue;
      }

      i = end + 2;
    }

    res.status(500).send('Failed to parse git http-backend output');
  };

  // Serve static files from the build directory
  app.use(express.static(path.join(__dirname, '../silentdisco_v1/build')));

  // Serve static object files from the uploaded object dir
  app.use('/objects', express.static(path.join(tmpDir, 'objects')));

  // accept post data
  app.use(express.raw({ limit: '50mb', type: '*/*' }));

  app.get('/', (req, res) => {
    // TODO route / to party_list.html
    res.sendFile(path.join(__dirname, '../silentdisco_v1/build/index.html'));
  });

  app.post('/create-party', async (req, res) => {
    // Convert raw document.cookie string into a dictionary object
    let rawCookies = '';
    for (let i = 0; i < req.rawHeaders.length; i++) {
      if (req.rawHeaders[i] === 'Cookie' && i + 1 < req.rawHeaders.length) {
        rawCookies = req.rawHeaders[i + 1];
      }
    }
    if (rawCookies === '') {
      res.status(401).json({ error: 'Unauthenticated' });
      return;
    }

    // TODO move this into a lib because it's shared with PartyList.js
    const cookies = rawCookies.split(';').reduce((acc, cookie) => {
      const [name, value] = cookie.trim().split('=');
      return { ...acc, [name]: value };
    }, {});
    // TODO write a test for this... it will be tricky because PartyList.js
    // won't render the /create-party button. Maybe manually force it to render
    // and then restart the server to get a new key to force validation to fail
    if (!('host' in cookies)) {
      res.status(401).json({ error: 'Unauthenticated' });
      return;
    }

    const verified = tweetnacl.sign.detached.verify(
      hostIsTrue,
      hexToUint8Array(cookies.host),
      publicKey,
    );
    if (!verified) {
      res.status(403).json({ error: 'Unauthorized' });
      return;
    }

    const partyID = crypto.randomBytes(3).toString('hex');

    initParty(partyID);
    res.send(partyID);
  });

  app.post('/device-key', (req, res) => {
    // TODO update devices.json
    res.send({});
  });

  app.get('/iamhost/:hostID', (req, res) => {
    const { hostID } = req.params;
    if (hostID !== randHostID) {
      res.redirect('/');
      return;
    }
    // TODO generalize this to more roles
    // TODO is there any value in making some cookies httpOnly?
    const signature = tweetnacl.sign.detached(hostIsTrue, secretKey);
    res.cookie('host', uint8ArrayToHex(signature));
    res.redirect('/');
  });

  app.get('/party/:partyID', (req, res) => {
    const partyID = req.params.partyID;
    // TODO move this into a lib to share with Party.js
    const hexPattern = /^[0-9A-Fa-f]{6}$/i;
    const isValid = partyID.length === 6 && hexPattern.test(partyID);
    if (!isValid) {
      res.redirect('/');
    }
    // TODO route /party/:partyID to now_playing.html
    res.sendFile(path.join(__dirname, '../silentdisco_v1/build/index.html'));
  });

  app.get('/party/:partyID.git/*', async (req, res) => {
    console.log('git request', req.method, req.url);
    const partyID = req.params.partyID;
    const isValid = isValidPartyID(partyID);
    if (!isValid) {
      res.redirect('/');
    }
    await processGitRequest(req, res, partyID);
  });

  app.post('/party/:partyID.git/*', async (req, res) => {
    console.log('git request', req.method, req.url);
    const partyID = req.params.partyID;
    const isValid = isValidPartyID(partyID);
    if (!isValid) {
      res.redirect('/');
    }
    await processGitRequest(req, res, partyID);
  });

  // /parties is its own GET request because each party is its own git
  // repo. So there's no way to replicate this in isogit.
  app.get('/parties', async (req, res) => {
    try {
      const parties = await fs.readdir(path.join(tmpDir, 'parties'));
      res.json(parties);
      return;
    } catch (err) {
      res.json([]);
      return;
    }
  });

  app.get('/ping', async (req, res) => {
    let initInMs = null;
    if ('nowInMs' in req.query) {
      initInMs = req.query.nowInMs;
    }
    res.json({
      clientInitInMs: initInMs,
      serverNowInMs: (new Date()).getTime(),
    });
  });

  app.post('/upload', async (req, res) => {
    console.log('got an upload', req.body);

    const parsedData = parseFileFromBody(req.body);
    const fileBytes = parsedData['file']['value'];
    const partyID = (new TextDecoder()).decode(parsedData['party_id']['value']);

    const hash = crypto.createHash('sha256');
    hash.update(fileBytes);
    const hexDigest = hash.digest('hex');

    // TODO parse filetype above and figure out if mp3 is reasonable
    const tmpFile = path.join(tmpDir, 'objects', hexDigest + '.mp3');
    await fs.writeFile(tmpFile, fileBytes);

    const partyDir = path.join(tmpDir, 'parties', partyID);

    // Similar to silentdisco/src/NowPlaying.js, I'm not sure why this git checkout
    // is necessary with isogit
    await git.checkout({ fs, dir: partyDir });
    await fs.appendFile(
      path.join(partyDir, 'objects.txt'),
      `{"sha256": "${hexDigest}", "filetype": "mp3", "name": "TODO"}\n`,
    );
    await git.add({ fs, dir: partyDir, filepath: 'objects.txt' });
    await git.commit({ fs, dir: partyDir, message: 'uploaded file', author: {
      name: 'Harry Potter',
      email: 'harry@example.com',
    }});
    res.status(200).send('ok');

    broadcast('please-pull');
  });

  app.get('*', (req, res) => {
    console.log('got unexpected get request', req.method, req.url);
    res.status(404).send('unexpected request');
  });

  app.post('*', (req, res) => {
    console.log('got unexpected post request', req.method, req.url);
    res.status(404).send('unexpected request');
  });

  process.on('SIGTERM', shutDown);
  process.on('SIGINT', shutDown);

  const initParty = async (partyID /* string */) => {
    const partyDir = path.join(tmpDir, 'parties', partyID);
    await fs.mkdir(partyDir, { recursive: true });
    // bare means we can run this as a git server, like github...
    await git.init({ fs, dir: partyDir, bare: true, defaultBranch: 'trunk' });
    await git.branch({ fs, dir: partyDir, ref: 'trunk', checkout: true });

    // TODO: Run `git config --bool http.receivepack true` to allow pushes
    child_process.execSync('git config --bool http.receivepack true', { env: { GIT_DIR: partyDir } });

    // TODO remove me once we know what we're doing
    let objectStr = '';
    if (partyID === '000000') {
      objectStr = '{"sha256": "9f033b2cf7176e5c18d9694103ac7ca9cbdad1a70d02a96648850690e9760542", "filetype": "mp3", "name": "Cello Suite - Bach"}\n';
    }
    await fs.writeFile(
      path.join(partyDir, 'objects.txt'),
      objectStr,
    );
    await git.add({ fs, dir: partyDir, filepath: 'objects.txt' });

    await fs.writeFile(path.join(partyDir, 'now_playing.txt'), '');
    await git.add({ fs, dir: partyDir, filepath: 'now_playing.txt' });

    await fs.writeFile(path.join(partyDir, 'public_key.txt'), publicKeyStr);
    await git.add({ fs, dir: partyDir, filepath: 'public_key.txt' });

    await git.commit({ fs, dir: partyDir, message: 'init party', author: {
      name: 'Harry Potter',
      email: 'harry@example.com',
    }});
  };

  // Start the server
  httpServer.listen(PORT, () => {
    console.log(`Generated signing key: ${publicKeyStr}`);
    console.log(`Server is running on http://localhost:${PORT}`);

    initParty('000000');
    console.log('Created empty 000000 party');

    console.log(`Host should visit http://localhost:${PORT}/iamhost/${randHostID}`);
  });
}

main()
