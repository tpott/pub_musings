import * as git from 'isomorphic-git';
import http from 'isomorphic-git/http/web';
import FS from '@isomorphic-git/lightning-fs';
import { useEffect, useState } from 'react';

import AudioFile from './AudioFile';
import './Party.css';

const clickDelaySec = 1.9; // 1900 milliseconds

// doNothing is an empty cleanup function to make react useEffect happy
const doNothing = () => {};

const fetchAudioObjects = async (fs) => {
  const fileBytes = await fs.promises.readFile(window.location.pathname + '/objects.txt');
  return (new TextDecoder()).decode(fileBytes)
    .split('\n')
    .filter(line => line !== '')
    .map(line => {
      const audioObj = JSON.parse(line)
      return `${audioObj['sha256']}.${audioObj['filetype']}`;
    });
};

const updateAudio = (
  appOffsetInSec,
  commit,
  fs,
  audioCtx,
  setAudioList,
  setIsPlaying,
  setNowPlayingI,
  setCurrentTime,
  setAudioCtxOffset,
) => {
  return async () => {
    // appOffsetInSec or commit may have changed

    const audioList = await fetchAudioObjects(fs);
    setAudioList(audioList);

    const fileBytes = await fs.promises.readFile(window.location.pathname + '/now_playing.txt');
    // TODO don't decode the entire file?
    const nowPlaying = (new TextDecoder()).decode(fileBytes);
    let nowInSec = (new Date()).getTime() / 1000.0 - (appOffsetInSec ?? 0.0);
    console.log('read now_playing', nowPlaying, `nowInSec=${nowInSec}`);
    // TODO do we need to handle more lines?
    const lines = nowPlaying.split('\n');
    if (lines.length === 0 || lines[0].length === 0) {
      console.log('empty lines or empty first line', lines, `nowInSec=${nowInSec}`);
      return;
    }

    // TODO add support for scheduling multiple lines
    const line = lines[0];
    if (line.length < 2) {
      console.error('now_playing line shorter than expected', line, `nowInSec=${nowInSec}`);
      return;
    }
    if (line[0] !== '(' || line[line.length - 1] !== ')') {
      console.error('now_playing line missing leading or trailing parenthesis', line, `nowInSec=${nowInSec}`);
      return;
    }
    // slice is to remove the leading and trailing paranthesis
    const fields = line.slice(1, -1).split(', ');
    if (fields.length !== 5) {
      console.error('now_playing line incorrect number of fields', line, `nowInSec=${nowInSec}`);
      return;
    }

    const i = parseInt(fields[1]);
    if (i < 0) {
      console.error('now_playing line with negative i', line, i, `nowInSec=${nowInSec}`);
      return;
    }
    if (i >= audioList.length) {
      console.error('now_playing line i out of bounds', line, i, audioList.length, `nowInSec=${nowInSec}`);
      return;
    }
    if (audioList[i] !== fields[2]) {
      console.error('now_playing line unknown audio vs expected', line, audioList[i], `nowInSec=${nowInSec}`);
      return;
    }

    // TODO move this into a function
    const actionType = fields[0];
    const startTime = parseFloat(fields[3]);
    // TODO iterate on appOffsetInSec some more
    const startAsOf = parseFloat(fields[4]);
    nowInSec = ((new Date()).getTime() / 1000.0) - appOffsetInSec;
    // diff > 0 means startAsOf is in the future, and we should schedule audio
    // in the future.
    // diff < 0 means startAsOf was in the past, and we should skip ahead,
    // meaning setCurrentTime should shorten the audio, and we should start now
    const diff = startAsOf - nowInSec;

    if (diff > 0) {
      console.log('scheduling action for future', diff, nowInSec, startAsOf, `nowInSec=${nowInSec}`);
    } else {
      console.log('scheduled action from past', diff);
    }
    console.log('going to', actionType, i, audioList[i], 'at', startTime);

    // TODO why do we have this diff?
    if (actionType === 'play' && diff <= 0) {
      setCurrentTime(startTime - diff);
    } else {
      setCurrentTime(startTime);
    }
    // TODO should audioCtxOffset be - diff or + diff?
    setAudioCtxOffset(audioCtx.currentTime + diff);
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
  // TODO use the react dom elements from state?
  const [audioList, setAudioList] = useState([]);
  const [isPlaying, setIsPlaying] = useState(false);
  const [nowPlayingI, setNowPlayingI] = useState(-1);
  // currentTime is seconds offset into now playing audio
  const [currentTime, setCurrentTime] = useState(0.0);
  // audioCtxOffset is seconds offset from audioContext.currentTime
  // when the current now playing audio should start for real
  const [audioCtxOffset, setAudioCtxOffset] = useState(0.0);

  useEffect(() => {
    // window.location.pathname == '/party/:partyID'
    const myFs = new FS('fs');
    setFS(myFs);
    return doNothing;
  }, []);

  useEffect(() => {
    if (fs == null) {
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
      setAudioCtxOffset,
    );
    myUpdateAudio();

    return doNothing;
  }, [appOffsetInSec, commit, fs, audioCtx]);

  // playOrPause takes a click and writes to git what song to play
  const playOrPause = (actionType, i) => {
    return async (childCurrentTime) => {
      // TODO use the react dom elements from state?
      const nowInSec = ((new Date()).getTime() / 1000.0) - appOffsetInSec;
      console.log('clicked', actionType, audioList[i], childCurrentTime, nowInSec);
      if (fs == null) {
        return;
      }

      const targetInSec = nowInSec + clickDelaySec;
      // TODO figure out time skew for scheduling in the future...
      await fs.promises.writeFile(
        window.location.pathname + '/now_playing.txt',
        `(${actionType}, ${i}, ${audioList[i]}, ${childCurrentTime}, ${targetInSec})\n`,
      );

      await git.add({ fs, dir: window.location.pathname, filepath: 'now_playing.txt'});
      const sha = await git.commit({
        fs,
        dir: window.location.pathname,
        author: {
          name: 'Ron Weasley',
          email: 'ron@weasly.com',
        },
        message: 'dj click',
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
      // TODO if (!pushResult.ok) { ... }

      setCommit(sha);
    };
  };

  const onEnded = (i) => {
    return () => {
      // TODO check if a i+1 should be playing. note it's possible audio i just
      // got paused
      console.log(`onEnded called for track ${i}`);
    };
  };

  // TODO audioCtxOffset
  const audioElemList = audioList.map((filename, i) => (
    <li>
      <AudioFile
        audioCtx={audioCtx}
        audioCtxOffset={audioCtxOffset}
        url={`/objects/${filename}`}
        isPlaying={isPlaying && (nowPlayingI === i)}
        currentTime={(nowPlayingI === i) ? currentTime : 0.0}
        parentPlay={playOrPause('play', i)}
        parentPause={playOrPause('pause', i)}
        onEnded={onEnded(i)}
      />
    </li>
  ));

  const uploadFile = async (e) => {
    e.preventDefault();
    const formData = new FormData();
    // TODO use the react state instead of finding the html element...
    const elem = document.getElementById('fileUpload');
    console.log('uploading', elem);
    formData.append('file', elem.files[0]);
    formData.append('party_id', partyID);
    const response = await fetch('/upload', {
      method: 'POST',
      body: formData,
    });
    if (!response.ok) {
      console.error('failed to upload file', response);
    }
    elem.value = ''; // clear the selected file
  };

  // TODO if roles includes 'dj' then replace 'Leave Party' button with 'Stop DJ'
  return (
    <div className='Party'>
      <header className='Party-header'>
        <p>Welcome to {partyID}</p>
        <p>Now playing: TODO, {isPlaying}</p>
        <ul>
          {audioElemList}
        </ul>
        <div>
          {isHost && <form onSubmit={uploadFile}>
            <input type='file' id='fileUpload' />
            <button type='submit'>Upload</button>
          </form>}
        </div>
        <p>My name: TODO</p>
        <p><button onClick={() => setListParty(true)}>Participants list</button></p>
        <p><button onClick={partyRedirect(null)}>Leave Party</button></p>
        <p>Commit: {commit}</p>
      </header>
    </div>
  );
}

export default AudioList;
