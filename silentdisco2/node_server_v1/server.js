const crypto = require('crypto');
const path = require('path');

const express = require('express');


const app = express();
const PORT = process.env.PORT || 8080;

// Serve static files from the build directory
app.use(express.static(path.join(__dirname, '../silentdisco/build')));

app.get('/', (req, res) => {
  // TODO route / to party_list.html
  res.sendFile(path.join(__dirname, './build/index.html'));
});

// TODO check if req.roles includes "host"
app.post('/create-party', (req, res) => {
  const partyId = crypto.randomBytes(3).toString('hex');
  res.send(partyId);
});

app.get('/party/:partyId', (req, res) => {
  const partyId = req.params.partyId;
  // TODO route /party/:partyId to now_playing.html
  res.sendFile(path.join(__dirname, '../silentdisco/build/index.html'));
});

// Start the server
app.listen(PORT, () => {
  console.log(`Server is running on http://localhost:${PORT}`);
});
