// App.js (React Native version)
import React, { useEffect, useState } from 'react';
import { View, Text } from 'react-native'; 

// If you're using a navigation library, you'd import something like:
// import { NavigationContainer } from '@react-navigation/native';
// import { createStackNavigator } from '@react-navigation/stack';
import * as git from 'isomorphic-git';
// TODO For React Native, you need a different "fs" solution for isomorphic-git:
// import RNFS from 'react-native-fs'; // Then configure isomorphic-git with that
// import { http } from 'isomorphic-git'; // Might need a specific RN adapter
// Also consider how to handle "dir" in React Native.

import tweetnacl from 'tweetnacl';

import Party from './Party';       // <-- You will need to rewrite these for RN
import PartyList from './PartyList'; // <-- You will need to rewrite these for RN

// TODO There's no global `window` in RN. If isomorphic-git or libraries require window.Buffer,
// you may have to polyfill Buffer or remove usage. 
// import { Buffer } from 'buffer';
// global.Buffer = Buffer; 

const serverHost = '192.168.1.51:8080';
const server = `http://${serverHost}`;

const gitIntervalMs = 16000; // 16 seconds
const minOffsetChangeInSec = 0.05; // 50 milliseconds
const reconnectTimeout = 2000; // 2 seconds * connectCount

// TODO don't hardcode this from server.js
const hostIsTrue = new Uint8Array([104, 111, 115, 116, 58, 116, 114, 117, 101]);

// TODO move this into a lib because it's shared with server.js
function hexToUint8Array(str) {
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

// TODO There's no `window.fetch` if you’re not on an environment that provides fetch.
// React Native 0.60+ should have a global fetch. Otherwise, consider `node-fetch`.
const ping = async (offsetInSec, setOffsetInSec) => {
  try {
    // TODO There's no concept of "window.location", so change this to your server address
    const startInMs = new Date().getTime();
    const resp = await fetch(`${server}/ping?nowInMs=${startInMs}`);
    const nowInMs = new Date().getTime();
    if (!resp.ok) {
      console.log(`ping failed rawNowInMs=${nowInMs}`);
      return;
    }
    const result = await resp.json();
    const rtt = nowInMs - startInMs;
    const newOffsetInSec = (nowInMs - result.serverNowInMs - rtt / 2) / 1000.0;
    console.log(`ping results, rtt=${rtt}, offset=${newOffsetInSec}, old offset=${offsetInSec}, rawNowInMs=${nowInMs}`);
    if (offsetInSec !== null && Math.abs(newOffsetInSec - offsetInSec) < minOffsetChangeInSec) {
      return;
    }
    setOffsetInSec(newOffsetInSec);
  } catch (err) {
    console.log('Error in ping:', err);
  }
};

// TODO isomorphic-git usage in React Native might differ. 
const myAsyncPullGit = (fs, commit, setCommit) => {
  return async () => {
    // TODO replace `dir: window.location.pathname` with something relevant to your RN setup

    // Example (very rough) usage:
    /*
    await git.fetch({ fs, http, dir: 'someLocalPath', remote: 'origin', ref: 'trunk' });
    const result = await git.merge({
      fs,
      dir: 'someLocalPath',
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

    await git.checkout({
      fs,
      dir: 'someLocalPath',
    });
    setCommit(result.oid);
    */
  };
};

export default function App() {
  const [commit, setCommit] = useState(null);
  const [partyID, setPartyID] = useState(null);
  const [publicKey, setPublicKey] = useState(null);
  const [isHost, setIsHost] = useState(false);
  const [wsClient, setWSClient] = useState(null);
  const [connectCount, setConnectCount] = useState(0);
  const [offsetInSec, setOffsetInSec] = useState(null);

  // TODO Web Audio API doesn't exist natively in React Native. If you need audio, use a RN library.
  const [audioCtx, setAudioCtx] = useState(null);

  // In React Native, you’d typically navigate between screens instead of pushing 
  // to window.history. This is just a placeholder.
  const partyRedirect = (partyIDValue) => {
    // TODO implement your navigation solution
    setPartyID(partyIDValue);
  };

  useEffect(() => {
    // This effect would handle your "routing" in a web app, 
    // but in RN, you need a navigation solution:
    // TODO Use a react-navigation param or something similar to get partyID
    setPartyID(null);
  }, []);

  useEffect(() => {
    // If you have logic that depends on the WS client:
    if (!wsClient) {
      return;
    }

    wsClient.onmessage = (e) => {
      // offsetInSec could be null on first run
      const now = new Date().getTime() / 1000.0 - (offsetInSec ?? 0);
      console.log(`Received websocket message: ${e.data} @ ${now}`);
      if (e.data === 'please-pull') {
        // TODO pass the correct fs reference
        myAsyncPullGit(null, commit, setCommit)();
      }
    };
  }, [wsClient, offsetInSec, commit]);

  useEffect(() => {
    // Instead of window.location, build your server URL and connect to websockets
    if (wsClient !== null) {
      return;
    }

    // Example (replace `wss://yourserver.com/ws` with your actual endpoint)
    const client = new WebSocket(`ws://${serverHost}/ws`);
    client.onopen = () => {
      console.log('WebSocket Client Connected');
      setConnectCount(0);
    };
    client.onclose = () => {
      console.log('WebSocket Client Disconnected');
      setTimeout(() => {
        setWSClient(null);
      }, connectCount * reconnectTimeout);
      setConnectCount(connectCount + 1);
    };

    // Note that in RN, you set onmessage in a separate effect or here:
    // client.onmessage = (e) => { ... }
    setWSClient(client);

    return () => {
      // TODO handle cleanup if necessary
      // client.close();
    };
  }, [wsClient, connectCount]);

  useEffect(() => {
    // This is the logic that initially sets up your Git repository in the web code.
    // In React Native, you need a different approach for filesystem & isomorphic-git.
    const initializeGitRepository = async () => {
      if (partyID === null) {
        return;
      }

      console.log('Clearing or setting up FS (React Native)');

      // TODO no indexedDB in React Native. Possibly use react-native-fs or async-storage.
      // This is purely an example. The path & usage will vary.
      // Example:
      /*
      const fs = {
        promises: {
          // see react-native-fs or create a custom "fs.promises" interface
        }
      };

      await git.init({ fs, dir: 'someLocalPath' });
      await git.clone({
        fs,
        http,
        dir: 'someLocalPath',
        url: 'https://yourserver.com/party.git', // or wherever
        singleBranch: true,
        depth: 1,
      });

      const currentCommit = await git.resolveRef({ fs, dir: 'someLocalPath', ref: 'HEAD' });
      const files = await git.listFiles({ fs, dir: 'someLocalPath' });
      console.log('done cloning', currentCommit, files);
      setCommit(currentCommit);

      // read public_key.txt
      const fileBytes = await fs.promises.readFile('someLocalPath/public_key.txt');
      const publicKeyStr = new TextDecoder().decode(fileBytes);
      setPublicKey(publicKeyStr);
      */
    };
    initializeGitRepository();
  }, [partyID]);

  useEffect(() => {
    // There's no document.cookie in React Native. 
    // You may need AsyncStorage or a custom solution to store "host" state.
    if (publicKey === null) {
      setIsHost(false);
      return;
    }

    // TODO Read "host" from your persistent storage or server token
    // Example: let hostCookieHex = await AsyncStorage.getItem('host');
    // For now, just pretend we don't have it:
    const hostCookieHex = null; // pretend we retrieved nothing

    if (!hostCookieHex) {
      setIsHost(false);
      return;
    }
    // same logic with tweetnacl
    setIsHost(tweetnacl.sign.detached.verify(
      hostIsTrue,
      hexToUint8Array(hostCookieHex),
      hexToUint8Array(publicKey),
    ));
  }, [publicKey]);

  useEffect(() => {
    // Kick off the initial ping if we don't have offset
    if (offsetInSec !== null) {
      return;
    }
    ping(offsetInSec, setOffsetInSec);
  }, [offsetInSec]);

  useEffect(() => {
    // In RN, there's no window.AudioContext. You need a different library 
    // (e.g., 'react-native-sound' or 'expo-av'). 
    // TODO implement or remove the audio context concept for React Native
    if (audioCtx !== null) {
      return;
    }
    // Example placeholder:
    setAudioCtx({});
  }, [audioCtx]);

  // In React Native, instead of returning DOM elements, you return <View>, <Text>, etc.
  if (partyID == null) {
    return (
      <View>
        <PartyList 
          // Example usage, but you'll need to rewrite PartyList
          partyRedirect={partyRedirect} 
          isHost={isHost} 
        />
      </View>
    );
  }

  return (
    <View>
      <Party
        audioCtx={audioCtx}
        appOffsetInSec={offsetInSec}
        commit={commit}
        isHost={isHost}
        partyID={partyID}
        partyRedirect={partyRedirect}
        setCommit={setCommit}
      />
    </View>
  );
}

