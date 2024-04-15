import * as git from 'isomorphic-git';
import FS from '@isomorphic-git/lightning-fs';
import { useEffect, useState } from 'react';

import Party from './Party';
import PartyList from './PartyList';

function App() {
  const [partyID, setPartyID] = useState(null);

  const partyRedirect = (partyID) => {
    return () => {
      if (partyID == null) {
        window.history.pushState({}, '', '/');
      } else {
        window.history.pushState({ partyID }, '', `/party/${partyID}`);
      }
      setPartyID(partyID);
    };
  };

  useEffect(() => {
    // Parse window.location to check if the URL is "/"
    const path = window.location.pathname;
    const [_, maybePartyID] = path.split('/party/');
    if (path.startsWith('/party/') && maybePartyID != null) {
      setPartyID(maybePartyID);
    } else {
      setPartyID(null);
    }

    // Add event listener to handle popstate events (back/forward navigation)
    const handlePopState = () => {
      const newPath = window.location.pathname;
      const [_, maybePartyID] = newPath.split('/party/');
      if (newPath.startsWith('/party/') && maybePartyID != null) {
        setPartyID(maybePartyID);
      } else {
        setPartyID(null);
      }
    };
    window.addEventListener('popstate', handlePopState);

    // Cleanup event listener on component unmount
    return () => {
      window.removeEventListener('popstate', handlePopState);
    };
  }, []);

  useEffect(() => {
    async function initializeGitRepository() {
      const fs = new FS('fs');
      await git.init({ fs, dir: '/' });
      console.log('done initializing fs and git');
      console.log(fs);
      // This is currently failing because "Buffer" is not defined in browsers
      // and the Buffer npm module isn't properly polyfilled in the isomorphic-git
      // repo.
      const files = await git.listFiles({ fs, dir: '/' });
      console.log(files);
    }
    initializeGitRepository();
  }, []);

  return (
    <div>
      {partyID == null ? <PartyList partyRedirect={partyRedirect} /> : <Party partyID={partyID} partyRedirect={partyRedirect} />}
    </div>
  );
}

export default App;
