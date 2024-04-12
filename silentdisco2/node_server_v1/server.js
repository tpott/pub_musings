const crypto = require('crypto');
const path = require('path');

const express = require('express');


const app = express();
const PORT = process.env.PORT || 8080;

// Serve static files from the build directory
app.use(express.static(path.join(__dirname, '../silentdisco/build')));

app.get('/', (req, res) => {
  // TODO route / to party_list.html
  res.sendFile(path.join(__dirname, '../silentdisco/build/index.html'));
});

// TODO check if req.roles includes "host"
let parties = {};
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

// Start the server
app.listen(PORT, () => {
  console.log(`Server is running on http://localhost:${PORT}`);
});
