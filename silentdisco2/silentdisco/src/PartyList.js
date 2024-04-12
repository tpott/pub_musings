import { useEffect, useState } from 'react';

import './PartyList.css';

function PartyList() {
  const [parties, setParties] = useState([]);

  useEffect(async () => {
    const response = await fetch('/parties');
    if (!response.ok) {
      console.error('Failed to fetch parties');
      return;
    }
    // Redirect to the newly created party
    const partiesArr = await response.json();
    // TODO validate each partyID with the same logic as in Party.js
    // i.e. hexPattern and length 6. Must also match logic in node_server_v1/server.js
    setParties(partiesArr);
  }, []);

  // TODO require roles.includes "host"
  const handleCreateParty = async () => {
    try {
      const response = await fetch('/create-party', { method: 'POST' });
      if (!response.ok) {
        console.error('Failed to create party');
        return;
      }
      // Redirect to the newly created party
      const partyID = await response.text();
      window.location.href = `/party/${partyID}`;
    } catch (err) {
      console.error('Error creating party:', err);
    }
  };

  // return a function that does the redirect, otherwise we will auto trigger this
  const redirect = (partyID: string) => {
    return () => {
      window.location.href = `/party/${partyID}`;
    };
  };

  // TODO only include create new party button if (roles.includes "host")
  return (
    <div className="PartyList">
      <header className="PartyList-header">
        <p>Party List</p>
        <ul>
          {parties.map((partyID) => (
            <li key={partyID}><button onClick={redirect(partyID)}>Join {partyID}</button></li>
          ))}
        </ul>
        <p><button onClick={handleCreateParty}>Create New Party</button></p>
      </header>
    </div>
  );
}

export default PartyList;
