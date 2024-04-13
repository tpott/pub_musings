import { useEffect, useState } from 'react';
import tweetnacl from 'tweetnacl';

import './PartyList.css';

// TODO move this into a lib because it's shared with server.js
function hexToUint8Array(str: string ): Uint8Array {
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

// TODO don't hardcode this from server.js
const hostIsTrue = new Uint8Array([104, 111, 115, 116, 58, 116, 114, 117, 101]);

// This is to make react happy
// https://stackoverflow.com/questions/56800694/what-is-the-expected-return-of-useeffect-used-for
const noEffect = () => {};

function PartyList() {
  const [parties, setParties] = useState([]);
  const [publicKey, setPublicKey] = useState(null);
  // TODO generalize this to more roles
  const [isHost, setIsHost] = useState(false);

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

  const fetchPublicKey = async () => {
    // TODO re-enable early returns from reading from local storage.
    // the problem was if the server was restarted, the key would change.
    // if (localStorage.hostPublicKeyStr != null) {
      // setPublicKey(localStorage.hostPublicKeyStr);
      // return;
    // }
    const response = await fetch('/public-key');
    if (!response.ok) {
      console.error('Failed to fetch public key');
    }
    const publicKeyStr = await response.text();
    localStorage.hostPublicKeyStr = publicKeyStr;
    setPublicKey(publicKeyStr);
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
    window.location.href = `/party/${partyID}`;
  };

  // return a function that does the redirect, otherwise we will auto trigger this
  const redirect = (partyID: string) => {
    return () => {
      window.location.href = `/party/${partyID}`;
    };
  };

  useEffect(() => {
    fetchParties();
  }, []);

  useEffect(() => {
    fetchPublicKey();
  }, []);

  useEffect(() => {
    if (publicKey === null) {
      setIsHost(false);
      return noEffect;
    }
    // Convert raw document.cookie string into a dictionary object
    const cookies = document.cookie.split(';').reduce((acc, cookie) => {
      const [name, value] = cookie.trim().split('=');
      return { ...acc, [name]: value };
    }, {});
    console.log('host signature is', cookies.host);
    if (!('host' in cookies)) {
      setIsHost(false);
      return noEffect;
    }
    setIsHost(tweetnacl.sign.detached.verify(
      hostIsTrue,
      hexToUint8Array(cookies.host),
      hexToUint8Array(publicKey),
    ));
  }, [publicKey]);

  return (
    <div className="PartyList">
      <header className="PartyList-header">
        <p>Party List</p>
        <ul>
          {parties.map((partyID) => (
            <li key={partyID}><button onClick={redirect(partyID)}>Join {partyID}</button></li>
          ))}
        </ul>
        {isHost && <p><button onClick={handleCreateParty}>Create New Party</button></p>}
      </header>
    </div>
  );
}

export default PartyList;
