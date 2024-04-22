import { useEffect, useState } from 'react';

import NowPlaying from './NowPlaying';
import './Party.css';

function Party({ partyID, partyRedirect }) {
  const [isValidPartyID, setIsValidPartyID] = useState(false);
  const [participantID, setParticipantID] = useState(null);
  const [listParty, setListParty] = useState(null);

  // TODO nowPlaying, so we know what song/video is playing
  // TODO DJ's name... idk if there's multiple DJs
  // TODO my roles... listener (everyone...), host, DJ
  // TODO my name

  useEffect(() => {
    // TODO move this to a lib to share with server.js
    const hexPattern = /^[0-9A-Fa-f]{6}$/i;
    const isValid = partyID.length === 6 && hexPattern.test(partyID);
    setIsValidPartyID(isValid);
  }, [partyID]);

  if (!isValidPartyID) {
    return (
      <>
        <h1>Invalid Party ID</h1>
        <p><button onClick={partyRedirect(null)}>Leave Party</button></p>
      </>
    );

  } else if (participantID != null) {
    // <Participant>
    return (
      <div className="Party">
        <header className="Party-header">
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
      <div className="Party">
        <header className="Party-header">
          <p><button onClick={() => setParticipantID("abcdef")}>No name</button></p>
          <p><button onClick={() => setListParty(null)}>Now Playing</button></p>
        </header>
      </div>
    );

  } else {
    return (
      <NowPlaying partyID={partyID} partyRedirect={partyRedirect} setListParty={setListParty} />
    );

  }
}

export default Party;
