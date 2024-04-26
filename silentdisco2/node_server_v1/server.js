const child_process = require('child_process');
const util = require('util');

const crypto = require('crypto');
const fs = require('fs/promises');
const os = require('os');
const path = require('path');

const express = require('express');
const git = require('isomorphic-git');
const tweetnacl = require('tweetnacl');


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

// Note: this is Uint8Array([104, 111, 115, 116, 58, 116, 114, 117, 101])
const hostIsTrue = textToUint8Array('host:true');


async function main() {
  const app = express();
  const PORT = process.env.PORT || 8080;

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
  console.log(`Created tempDir: ${tmpDir}`);

  async function shutDown() {
    server.close();
    await fs.rm(tmpDir, { recursive: true });
    console.log('Shut down');
  }

  // TODO move this into a lib to share with Party.js
  const isValidPartyID = (partyID /*: string */) => {
    const hexPattern = /^[0-9A-Fa-f]{6}$/i;
    return partyID.length === 6 && hexPattern.test(partyID);
  };

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
  app.use(express.static(path.join(__dirname, '../silentdisco/build')));

  // Serve static object files from the uploaded object dir
  app.use('/objects', express.static(path.join(tmpDir, 'objects')));

  // accept post data
  app.use(express.raw({ type: '*/*' }));

  app.get('/', (req, res) => {
    // TODO route / to party_list.html
    res.sendFile(path.join(__dirname, '../silentdisco/build/index.html'));
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
    res.sendFile(path.join(__dirname, '../silentdisco/build/index.html'));
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

  app.get('/public-key', (req, res) => {
    res.send(publicKeyStr);
  });

  app.post('/device-key', (req, res) => {
    // TODO update devices.json
    res.send({});
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
    await fs.writeFile(path.join(partyDir, 'now_playing.txt'), '');
    await git.add({ fs, dir: partyDir, filepath: 'now_playing.txt' });
    await git.commit({ fs, dir: partyDir, message: 'init party', author: {
      name: 'Harry Potter',
      email: 'harry@example.com',
    }});
  };

  // Start the server
  const server = app.listen(PORT, () => {
    console.log(`Generated signing key: ${publicKeyStr}`);
    console.log(`Server is running on http://localhost:${PORT}`);

    initParty('000000');
    console.log('Created empty 000000 party');

    console.log(`Host should visit http://localhost:${PORT}/iamhost/${randHostID}`);
  });
}

main()
