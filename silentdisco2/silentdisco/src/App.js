import * as git from 'isomorphic-git';
import http from "isomorphic-git/http/web";
import FS from '@isomorphic-git/lightning-fs';
import { useEffect, useState } from 'react';
import tweetnacl from 'tweetnacl';

import Party from './Party';
import PartyList from './PartyList';

// WTF https://github.com/isomorphic-git/isomorphic-git/issues/1680
import { Buffer } from 'buffer';
window.Buffer = Buffer;

// This is to make react happy
// https://stackoverflow.com/questions/56800694/what-is-the-expected-return-of-useeffect-used-for
const noEffect = () => {};

// TODO don't hardcode this from server.js
const hostIsTrue = new Uint8Array([104, 111, 115, 116, 58, 116, 114, 117, 101]);

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

function App() {
  const [partyID, setPartyID] = useState(null);
  const [publicKey, setPublicKey] = useState(null);
  // TODO generalize this to more roles
  const [isHost, setIsHost] = useState(false);

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
    console.log('going to initialize App.js...');

    const initializeGitRepository = async () => {
      if (partyID === null) {
        return;
      }

      // This is necessary otherwise we may have loaded the browser with an old git repo
      console.log('clearing the fs');
      indexedDB.deleteDatabase('fs');

      // window.location.pathname == '/party/:partyID'
      const fs = new FS('fs'); // maybe call this partyID?
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

      // This is supposed to be like `git rev-parse HEAD`
      const currentCommit = await git.resolveRef({ fs, dir: window.location.pathname, ref: 'HEAD' });
      const files = await git.listFiles({ fs, dir: window.location.pathname });
      console.log('done cloning', currentCommit, files);

      const fileBytes = await fs.promises.readFile(window.location.pathname + '/public_key.txt');
      const publicKeyStr = (new TextDecoder()).decode(fileBytes);
      setPublicKey(publicKeyStr);
    };

    initializeGitRepository();
  }, [partyID]);

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

  if (partyID == null) {
    return (
      <div>
        <PartyList partyRedirect={partyRedirect} isHost={isHost} />
      </div>
    );
  }

  return (
    <div>
      <Party partyID={partyID} partyRedirect={partyRedirect} isHost={isHost} />
    </div>
  );
}

export default App;
