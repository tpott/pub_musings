const crypto = require('crypto');
const path = require('path');

const express = require('express');
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


const app = express();
const PORT = process.env.PORT || 8080;

let parties = {};
let randHostID = crypto.randomBytes(16).toString('hex');
const { publicKey, secretKey } = tweetnacl.sign.keyPair();
const publicKeyStr = uint8ArrayToHex(publicKey);

// Serve static files from the build directory
app.use(express.static(path.join(__dirname, '../silentdisco/build')));

app.get('/', (req, res) => {
  // TODO route / to party_list.html
  res.sendFile(path.join(__dirname, '../silentdisco/build/index.html'));
});

app.post('/create-party', (req, res) => {
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
  parties[partyID] = {};
  res.send(partyID);
});

app.get('/party/:partyID', (req, res) => {
  const partyID = req.params.partyID;
  // TODO route /party/:partyID to now_playing.html
  res.sendFile(path.join(__dirname, '../silentdisco/build/index.html'));
});

app.get('/parties', (req, res) => {
  res.json(Object.keys(parties));
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

app.get('/public-key', (req, res) => {
  res.send(publicKeyStr);
})

// Start the server
app.listen(PORT, () => {
  console.log(`Generated signing key: ${publicKeyStr}`);
  console.log(`Server is running on http://localhost:${PORT}`);
  console.log(`Host should visit http://localhost:${PORT}/iamhost/${randHostID}`);
});
