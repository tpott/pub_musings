import * as git from 'isomorphic-git';
import http from "isomorphic-git/http/web";
import FS from '@isomorphic-git/lightning-fs';
import { useEffect, useState } from 'react';

import Party from './Party';
import PartyList from './PartyList';

// WTF https://github.com/isomorphic-git/isomorphic-git/issues/1680
import { Buffer } from 'buffer';
window.Buffer = Buffer;

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
    const [, maybePartyID] = path.split('/party/');
    if (path.startsWith('/party/') && maybePartyID != null) {
      setPartyID(maybePartyID);
    } else {
      setPartyID(null);
    }

    // Add event listener to handle popstate events (back/forward navigation)
    const handlePopState = () => {
      const newPath = window.location.pathname;
      const [, maybePartyID] = newPath.split('/party/');
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
    const initializeGitRepository = async () => {
      if (partyID === null) {
        return;
      }

      // window.location.pathname == '/party/:partyID'
      const fs = new FS('fs');
      await git.init({ fs, dir: window.location.pathname });
      console.log('done initializing fs and git');

      // Note: if the page is already loaded, then fs may be cached, and git may already
      // be cloned...

      // TODO I should use partyID state here instead of window.location...
      // otherwise, the browser will send two requests when it browses to /party/000000
      // once when partyID is null and once from when partyID is parsed properly
      await git.clone({
        fs,
        http,
        dir: window.location.pathname,
        url: window.location.href + '.git',
        singleBranch: true,
        depth: 1
      });
      console.log('done cloning');

      const files = await git.listFiles({ fs, dir: window.location.pathname });
      console.log(files);

/*
      // I couldn't find a great way to clear browser data... If you're inspecting the site
      // then click the "Application" tab, then "Storage" on the left and then "Clear site data".
      // That should clear the IndexedDB data that backs lightning FS.
      await fs.promises.writeFile(window.location.pathname + '/now_playing.txt', 'hey\n# start\n');
      await git.add({ fs, dir: window.location.pathname, filepath: 'now_playing.txt'});
      const sha = await git.commit({
        fs,
        dir: window.location.pathname,
        author: {
          name: 'Ron Weasley',
          email: 'ron@weasly.com',
        },
        message: 'lolz',
      });
      console.log('done committing', sha);
      const pushResult = await git.push({
        fs,
        http,
        dir: window.location.pathname,
        remote: 'origin',
        ref: 'trunk',
      });
      console.log('done pushing', pushResult);
*/

    };

    console.log('going to initialize...');
    initializeGitRepository();
  }, [partyID]);

  if (partyID == null) {
    return (
      <div>
        <PartyList partyRedirect={partyRedirect} />
      </div>
    );
  }

  return (
    <div>
      <Party partyID={partyID} partyRedirect={partyRedirect} />
    </div>
  );
}

export default App;
