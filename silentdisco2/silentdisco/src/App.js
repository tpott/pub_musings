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
const doNothing = () => {};

const gitIntervalMs = 16000; // 16 seconds
const minOffsetChangeInSec = 0.05; // 50 milliseconds
const reconnectTime = 2000; // 2 seconds

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

const ping = async (offsetInSec, setOffsetInSec) => {
  const startInMs = (new Date()).getTime();
  const resp = await fetch(`/ping?nowInMs=${startInMs}`);
  const nowInMs = (new Date()).getTime();
  if (!resp.ok) {
    console.log('ping failed');
  }
  const result = await resp.json();
  // optional parse result.clientInitInMs vs startInMs
  const rtt = nowInMs - startInMs;
  const newOffsetInSec = (nowInMs - result.serverNowInMs - (rtt / 2)) / 1000.0;
  console.log(`ping results, rtt=${rtt}, offset=${newOffsetInSec}, old offset=${offsetInSec}, now=${nowInMs}`);
  if (offsetInSec !== null && Math.abs(newOffsetInSec - offsetInSec) < minOffsetChangeInSec) {
    return;
  }
  setOffsetInSec(newOffsetInSec);
};

const myAsyncPullGit = (
  fs,
  commit,
  setCommit,
) => {
  return async () => {
    await git.fetch({
      fs,
      http,
      dir: window.location.pathname,
      remote: 'origin',
      ref: 'trunk',
    });

    const result = await git.merge({
      fs,
      dir: window.location.pathname,
      theirs: 'remotes/origin/trunk',
      ours: 'trunk',
      author: {
        name: 'Ron Weasley',
        email: 'ron@weasly.com',
      },
    });
    console.log('fetched', result);

    if (commit == null) {
      setCommit(result.oid);
    }

    if (result.alreadyMerged ?? false) {
      return;
    }

    // I'm not entirely sure why isogit requires us to checkout the branch we just
    // updated with the merge...
    await git.checkout({
      fs,
      dir: window.location.pathname,
    });

    // Force the component to re-render
    setCommit(result.oid);
  };
};

function App() {
  const [commit, setCommit] = useState(null);
  const [partyID, setPartyID] = useState(null);
  const [publicKey, setPublicKey] = useState(null);
  // TODO generalize this to more roles
  const [isHost, setIsHost] = useState(false);
  const [wsClient, setWSClient] = useState(null);
  const [connectCount, setConnectCount] = useState(0);
  // a positive offset (> 0) means this device is ahead of the server
  // a negative offset (< 0) means this device is behind the server
  const [offsetInSec, setOffsetInSec] = useState(null);

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

    const fs = new FS('fs');
    const asyncPullGit = myAsyncPullGit(
      fs,
      commit,
      setCommit,
    );

    // TODO set a reasonable interval for pulling git
    const intervalId = setInterval(() => {
      asyncPullGit();
      ping(offsetInSec, setOffsetInSec);
    }, gitIntervalMs);

    if (wsClient == null) {
      return () => {
        window.removeEventListener('popstate', handlePopState);
        clearInterval(intervalId);
      };
    }

    wsClient.onmessage = (e) => {
      const nowInMs = ((new Date()).getTime() / 1000.0) - offsetInSec;
      console.log(`Received websocket message: ${e.data} @ ${nowInMs}`);
      if (e.data === 'please-pull') {
        asyncPullGit();
      }
    };

    // Cleanup event listener on component unmount
    return () => {
      window.removeEventListener('popstate', handlePopState);
      clearInterval(intervalId);
    };
  }, [offsetInSec, commit, wsClient]);

  useEffect(() => {
    if (wsClient !== null) {
      return doNothing;
    }

    let port = '443';
    if (window.location.port.length !== 0) {
      port = window.location.port;
      console.log('overwrote port', port, window.location.port.slice(0, 3));
    }
    console.log('connecting to websockets...', port, window.location.port, window.location.port.length);

    let protocol = 'ws';
    if (window.location.protocol === 'https:') {
      protocol = 'wss';
    }

    const connect = () => {
      const client = new WebSocket(`${protocol}://${window.location.hostname}:${port}/ws`);
      client.onopen = () => {
        console.log('WebSocket Client Connected', client);
        setConnectCount(0);
      };
      client.onclose = () => {
        console.log('WebSocket Client Disconnected');
        setTimeout(connect, reconnectTime * (connectCount + 1));
        setConnectCount(connectCount + 1);
      };
      // don't set client.onmessage here. we need asyncPullGit for that
      setWSClient(client);
    };

    connect();

    return () => {
      // TODO when should we close the websocket? not doing at all will lead to
      // memory leaks
      // client.close();
    };
  }, [wsClient, connectCount]);

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
      setCommit(currentCommit);

      // TODO this doesn't work when window.location isn't in a /party/
      const fileBytes = await fs.promises.readFile(window.location.pathname + '/public_key.txt');
      const publicKeyStr = (new TextDecoder()).decode(fileBytes);
      setPublicKey(publicKeyStr);
    };

    initializeGitRepository();
  }, [partyID]);

  useEffect(() => {
    if (publicKey === null) {
      setIsHost(false);
      return doNothing;
    }
    // Convert raw document.cookie string into a dictionary object
    const cookies = document.cookie.split(';').reduce((acc, cookie) => {
      const [name, value] = cookie.trim().split('=');
      return { ...acc, [name]: value };
    }, {});
    if (!('host' in cookies)) {
      setIsHost(false);
      return doNothing;
    }
    setIsHost(tweetnacl.sign.detached.verify(
      hostIsTrue,
      hexToUint8Array(cookies.host),
      hexToUint8Array(publicKey),
    ));
  }, [publicKey]);

  useEffect(() => {
    if (offsetInSec !== null) {
      return;
    }
    ping(offsetInSec, setOffsetInSec);
  }, [offsetInSec]);

  if (partyID == null) {
    return (
      <div>
        <PartyList partyRedirect={partyRedirect} isHost={isHost} />
      </div>
    );
  }

  return (
    <div>
      <Party
        appOffsetInSec={offsetInSec}
        commit={commit}
        isHost={isHost}
        partyID={partyID}
        partyRedirect={partyRedirect}
        setCommit={setCommit}
      />
    </div>
  );
}

export default App;
