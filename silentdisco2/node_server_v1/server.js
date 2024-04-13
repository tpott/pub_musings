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

// TODO check if req.roles includes "host"
app.post('/create-party', (req, res) => {
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
  // Note: this is Uint8Array([104, 111, 115, 116, 58, 116, 114, 117, 101])
  const signature = tweetnacl.sign.detached(textToUint8Array('host:true'), secretKey);
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
