// AudioList.js (React Native version)

import React, { useEffect, useState } from 'react';
import {
  View,
  Text,
  Button,
  StyleSheet,
  TouchableOpacity,
  Alert,
} from 'react-native';

// TODO isomorphic-git in React Native may require a custom FS. 
// You might need react-native-fs or an in-memory approach:
import * as git from 'isomorphic-git';
// import http from 'isomorphic-git/http/web'; // TODO might not work as-is in RN
// import FS from '@isomorphic-git/lightning-fs';

import AudioFile from './AudioFile'; // <-- This also needs adaptation for RN
// import './Party.css'; // <-- Not applicable in React Native

const clickDelaySec = 1.9; // 1900 milliseconds

// doNothing is an empty cleanup function to keep React happy
const doNothing = () => {};

// TODO RN does not have `window.location`, nor `indexedDB`. We must adapt logic accordingly.
// This function is a placeholder for fetching your audio objects:
const fetchAudioObjects = async (fs) => {
  // In a web environment, we read a file from window.location.pathname + '/objects.txt'
  // For RN, either fetch from a server or use a local RNFS path, etc.
  // For now, just return an empty array or some mock data:
  return [];
};

// TODO This function references "window.location.pathname" and "TextDecoder".
// In React Native, you'd likely fetch from a server or a local path. 
// This is a partial placeholder demonstrating the concept of updating audio state.
const updateAudio = (
  appOffsetInSec,
  commit,
  fs,
  audioCtx,
  setAudioList,
  setIsPlaying,
  setNowPlayingI,
  setCurrentTime,
  setAudioCtxOffset
) => {
  return async () => {
    // appOffsetInSec or commit may have changed
    const audioList = await fetchAudioObjects(fs);
    setAudioList(audioList);

    // TODO read now_playing.txt from your filesystem or server
    // This code uses "fs" and "TextDecoder" from the web. 
    // In RN, adapt to your storage solution or fetch logic.
    const nowPlaying = ""; // placeholder for reading the file
    let nowInSec = new Date().getTime() / 1000.0 - (appOffsetInSec ?? 0.0);
    console.log('read now_playing', nowPlaying, `nowInSec=${nowInSec}`);

    // The rest of the logic remains conceptually the same, 
    // but you'll need to adapt to how you retrieve now_playing data in RN.
    const lines = nowPlaying.split('\n');
    if (lines.length === 0 || lines[0].length === 0) {
      return;
    }
    // Example of parsing the line:
    const line = lines[0];
    if (line[0] !== '(' || line[line.length - 1] !== ')') {
      return;
    }
    const fields = line.slice(1, -1).split(', ');
    if (fields.length !== 5) {
      return;
    }

    const actionType = fields[0];
    const i = parseInt(fields[1], 10);
    const fileName = fields[2];
    const startTime = parseFloat(fields[3]);
    const startAsOf = parseFloat(fields[4]);

    nowInSec = new Date().getTime() / 1000.0 - (appOffsetInSec ?? 0);
    const diff = startAsOf - nowInSec;

    if (actionType === 'play' && diff <= 0) {
      setCurrentTime(startTime - diff);
    } else {
      setCurrentTime(startTime);
    }

    // audioCtx might be replaced by something else in RN
    setAudioCtxOffset((audioCtx?.currentTime ?? 0) + diff);
    setNowPlayingI(i);
    setIsPlaying(actionType === 'play');
  };
};

function AudioList({
  audioCtx,
  appOffsetInSec,
  commit,
  isHost,
  partyID,
  partyRedirect,
  setCommit,
  setListParty,
}) {
  const [fs, setFS] = useState(null);
  const [audioList, setAudioList] = useState([]);
  const [isPlaying, setIsPlaying] = useState(false);
  const [nowPlayingI, setNowPlayingI] = useState(-1);
  const [currentTime, setCurrentTime] = useState(0.0);
  const [audioCtxOffset, setAudioCtxOffset] = useState(0.0);

  useEffect(() => {
    // In the browser, you'd do:
    //    const myFs = new FS('fs');
    // For React Native, you must use a different approach (react-native-fs, custom).
    // TODO set up or replace with a suitable FS in RN
    setFS(null);

    return doNothing;
  }, []);

  useEffect(() => {
    if (!fs) {
      return doNothing;
    }
    const myUpdateAudio = updateAudio(
      appOffsetInSec,
      commit,
      fs,
      audioCtx,
      setAudioList,
      setIsPlaying,
      setNowPlayingI,
      setCurrentTime,
      setAudioCtxOffset
    );
    myUpdateAudio();

    return doNothing;
  }, [appOffsetInSec, commit, fs, audioCtx]);

  // playOrPause would schedule a play or pause by writing to now_playing.txt
  // and pushing to Git. In RN, isomorphic-git might work differently.
  const playOrPause = (actionType, i) => {
    return async (childCurrentTime) => {
      const nowInSec = new Date().getTime() / 1000.0 - (appOffsetInSec ?? 0);
      console.log('clicked', actionType, audioList[i], childCurrentTime, nowInSec);

      if (!fs) {
        return;
      }

      const targetInSec = nowInSec + clickDelaySec;

      // TODO In RN, writing this file to the correct place is tricky.
      // Also you can't rely on "window.location.pathname".
      // You may also need to figure out how to push with isomorphic-git in RN.
      // The following lines are placeholders for your Git logic:
      /*
      await fs.promises.writeFile(
        someRNPath + '/now_playing.txt',
        `(${actionType}, ${i}, ${audioList[i]}, ${childCurrentTime}, ${targetInSec})\n`
      );

      await git.add({ fs, dir: someRNPath, filepath: 'now_playing.txt' });
      const sha = await git.commit({
        fs,
        dir: someRNPath,
        author: {
          name: 'Ron Weasley',
          email: 'ron@weasly.com',
        },
        message: 'dj click',
      });

      const pushResult = await git.push({
        fs,
        http,
        dir: someRNPath,
        remote: 'origin',
        ref: 'trunk',
      });
      */

      // TODO handle pushResult or commit
      // For demonstration, call setCommit:
      setCommit('newShaPlaceholder');
    };
  };

  const onEnded = (i) => {
    return () => {
      // TODO decide if i+1 should be played automatically
      console.log(`onEnded called for track ${i}`);
    };
  };

  // This would be how we render each audio item in web React. For RN, <AudioFile /> must also be adapted.
  const audioElemList = audioList.map((filename, i) => (
    <View key={filename} style={styles.audioItem}>
      <AudioFile
        audioCtx={audioCtx}
        audioCtxOffset={audioCtxOffset}
        url={`/objects/${filename}`} // TODO adapt for RN, possibly a real URL or local file
        isPlaying={isPlaying && nowPlayingI === i}
        currentTime={nowPlayingI === i ? currentTime : 0.0}
        parentPlay={playOrPause('play', i)}
        parentPause={playOrPause('pause', i)}
        onEnded={onEnded(i)}
      />
    </View>
  ));

  // In React Native, there's no <form> or <input type="file" />. 
  // Instead, you might use react-native-document-picker or expo-document-picker.
  // This function is just a placeholder to show the concept:
  const uploadFile = async () => {
    // TODO implement file picking and uploading in RN:
    Alert.alert(
      'Upload File',
      'File uploads need a special approach in React Native (no HTML form).'
    );
    // Once a file is picked, you could do:
    // const response = await fetch('https://yourserver.com/upload', {
    //   method: 'POST',
    //   body: formData,
    // });
  };

  return (
    <View style={styles.container}>
      <Text style={styles.header}>
        Welcome to {partyID}
      </Text>
      <Text style={styles.text}>
        Now playing: TODO, {isPlaying ? 'Playing' : 'Paused'}
      </Text>

      {/* Audio list items */}
      {audioElemList}

      {/* Upload button if isHost */}
      {isHost && (
        <Button
          title="Upload"
          onPress={uploadFile}
        />
      )}

      <Text style={styles.text}>My name: TODO</Text>

      <View style={styles.buttonsRow}>
        <Button
          title="Participants list"
          onPress={() => setListParty(true)}
        />
        <Button
          title="Leave Party"
          onPress={() => partyRedirect(null)}
        />
      </View>

      <Text style={styles.text}>Commit: {commit}</Text>
    </View>
  );
}

export default AudioList;

const styles = StyleSheet.create({
  container: {
    // TODO Adjust as needed for your layout
    flex: 1,
    padding: 16,
    justifyContent: 'flex-start',
    alignItems: 'stretch',
    backgroundColor: '#fff',
  },
  header: {
    fontSize: 20,
    marginBottom: 16,
  },
  text: {
    marginVertical: 8,
  },
  audioItem: {
    marginVertical: 4,
  },
  buttonsRow: {
    flexDirection: 'row',
    justifyContent: 'space-around',
    marginVertical: 16,
  },
});

