import { useEffect, useState } from 'react';

import './PartyList.css';

function PartyList({ partyRedirect, isHost }) {
  const [parties, setParties] = useState([]);

  // TODO read parties from the local browser's FS
  const fetchParties = async () => {
    const response = await fetch('/parties');
    if (!response.ok) {
      return;
    }
    // Redirect to the newly created party
    const partiesArr = await response.json();
    // TODO validate each partyID with the same logic as in Party.js
    // i.e. hexPattern and length 6. Must also match logic in node_server_v1/server.js
    setParties(partiesArr);
  };

  // TODO require roles.includes "host"
  const handleCreateParty = async () => {
    const response = await fetch('/create-party', { method: 'POST' });
    if (!response.ok) {
      console.error('Failed to create party');
      return;
    }
    // Redirect to the newly created party
    const partyID = await response.text();
    partyRedirect(partyID)();
  };

  useEffect(() => {
    fetchParties();
  }, []);

  return (
    <div className="PartyList">
      <header className="PartyList-header">
        <p>Party List</p>
        <ul>
          {parties.map((partyID) => (
            <li key={partyID}><button onClick={partyRedirect(partyID)}>Join {partyID}</button></li>
          ))}
        </ul>
        {isHost && <p><button onClick={handleCreateParty}>Create New Party</button></p>}
      </header>
    </div>
  );
}

export default PartyList;
