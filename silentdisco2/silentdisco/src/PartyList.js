import { useState } from 'react';

import './App.css';

// TODO App should be called PartyList
function App() {
  const [partyID, setPartyID] = useState(null);
  const [participantID, setParticipantID] = useState(null);
  const [listParty, setListParty] = useState(null);

  // TODO nowPlaying, so we know what song/video is playing
  // TODO DJ's name... idk if there's multiple DJs
  // TODO my roles... listener (everyone...), host, DJ
  // TODO my name

  // TODO require roles.includes "host"
  const handleCreateParty = async () => {
    try {
      const response = await fetch('/create-party', { method: 'POST' });
      if (!response.ok) {
        console.error('Failed to create party');
        return;
      }
      // Redirect to the newly created party
      const partyId = await response.text();
      window.location.href = `/party/${partyId}`;
    } catch (err) {
      console.error('Error creating party:', err);
    }
  };

  if (partyID == null) {
    // <PartyList>
    // TODO only include create new party button if (roles.includes "host")
    return (
      <div className="App">
        <header className="App-header">
          <p>Party List</p>
          <p>* <button onClick={() => setPartyID("todoXY")}>TODO</button></p>
          <p><button onClick={handleCreateParty}>Create New Party</button></p>
        </header>
      </div>
    );

  } else if (participantID != null) {
    // <Participant>
    return (
      <div className="App">
        <header className="App-header">
          <p>Name: No Name // TODO</p>
          <p>Roles: // TODO</p>
          <p>TODO if (roles.includes "host") "Invite to DJ"</p>
          <p><button onClick={() => setParticipantID(null)}>Participants list</button></p>
        </header>
      </div>
    );

  } else if (listParty ?? false) {
    // <ParticipantList>
    return (
      <div className="App">
        <header className="App-header">
          <p><button onClick={() => setParticipantID("abcdef")}>No name</button></p>
          <p><button onClick={() => setListParty(null)}>Now Playing</button></p>
        </header>
      </div>
    );

  } else {
    // <NowPlaying>
    // TODO if roles includes "dj" then replace "Leave Party" button with "Stop DJ"
    return (
      <div className="App">
        <header className="App-header">
          <p>Welcome to {partyID}</p>
          <p>Now playing: TODO</p>
          <audio controls preload="auto">
            <source src="e_J14fbBluE.mp3" />
          </audio>
          <p>My name: TODO</p>
          <p><button onClick={() => setListParty(true)}>Participants list</button></p>
          <p><button onClick={() => setPartyID(null)}>Leave Party</button></p>
        </header>
      </div>
    );

  }
}

export default App;
