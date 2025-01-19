import * as git from 'isomorphic-git';
import http from 'isomorphic-git/http/web';
import FS from '@isomorphic-git/lightning-fs';
import { useEffect, useState } from 'react';

import AudioFile from './AudioFile';
import './Party.css';

const clickDelaySec = 0.2; // 200 milliseconds
const endingBufferSec = 0.1; // 100 milliseconds

const correctionsEnabled = true;
const coeffPosition = 0.25;
const coeffIntegral = 0.1;
const coeffDerivative = 1.0;
const playbackErrorLimit = 30; // record at most the last 30 events
const minCorrectionForce = 0.3; // 300 milliseconds
const minNumPlaybackErrors = 15;

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
  setAudioList,
  setNowPlayingI,
  setCurrentTime,
  setPlayState,
  setPlaybackErrors,
) => {
  return async () => {
    // appOffsetInSec or commit may have changed

    const audioList = await fetchAudioObjects(fs);
    setAudioList(audioList);

    const fileBytes = await fs.promises.readFile(window.location.pathname + '/now_playing.txt');
    // TODO don't decode the entire file?
    const nowPlaying = (new TextDecoder()).decode(fileBytes);
    console.log('read now_playing', nowPlaying);
    // TODO do we need to handle more lines?
    const lines = nowPlaying.split('\n');
    if (lines.length === 0 || lines[0].length === 0) {
      console.log('empty lines or empty first line', lines);
      return;
    }

    const line = lines[0];
    if (line.length < 2) {
      console.error('now_playing line shorter than expected', line);
      return;
    }
    if (line[0] !== '(' || line[line.length - 1] !== ')') {
      console.error('now_playing line missing leading or trailing parenthesis', line);
      return;
    }
    // slice is to remove the leading and trailing paranthesis
    const fields = line.slice(1, -1).split(', ');
    if (fields.length !== 5) {
      console.error('now_playing line incorrect number of fields', line);
      return;
    }

    const i = parseInt(fields[1]);
    if (i < 0) {
      console.error('now_playing line with negative i', line, i);
      return;
    }
    if (i >= audioList.length) {
      console.error('now_playing line i out of bounds', line, i, audioList.length);
      return;
    }
    if (audioList[i] !== fields[2]) {
      console.error('now_playing line unknown audio vs expected', line, audioList[i]);
      return;
    }

    // TODO move this into a function
    const actionType = fields[0];
    const startTime = parseFloat(fields[3]);
    // TODO iterate on appOffsetInSec some more
    const startAsOf = parseFloat(fields[4]);
    const nowInSec = ((new Date()).getTime() / 1000.0) - appOffsetInSec;
    const diff = startAsOf - nowInSec;

    const updatePlaying = () => {
      // TODO why do we have this diff?
      if (actionType === 'play' && diff <= 0) {
        setCurrentTime(startTime - diff);
      } else {
        setCurrentTime(startTime);
      }
      console.log('going to', actionType, i, audioList[i]);
      if (actionType === 'play') {
        setNowPlayingI(i);
        setPlayState(startAsOf);
      } else {
        setNowPlayingI(-1);
        setPlayState(null);
        setPlaybackErrors([]);
      }
    };

    if (diff > 0) {
      console.log('scheduling action for future', diff, nowInSec, startAsOf);
      setTimeout(updatePlaying, diff * 1000);
    } else {
      console.log('scheduled action from past', diff);
      updatePlaying();
    }

  };
};

function NowPlaying({
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
  const [nowPlayingI, setNowPlayingI] = useState(-1);
  // playState is seconds offset from unix epoch, i.e. UTC
  const [playState, setPlayState] = useState(null);
  // currentTime is seconds offset into now playing audio
  const [currentTime, setCurrentTime] = useState(0.0);
  const [playbackErrors, setPlaybackErrors] = useState([]);
  const [numCorrections, setNumCorrections] = useState(0);

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
      setAudioList,
      setNowPlayingI,
      setCurrentTime,
      setPlayState,
      setPlaybackErrors,
    );
    myUpdateAudio();

    return doNothing;
  }, [appOffsetInSec, fs, commit]);

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
      setCurrentTime(childCurrentTime);
    };
  };

  const onEnded = (i) => {
    return () => {
      // TODO check if a i+1 should be playing. note it's possible audio i just
      // got paused
      console.log(`onEnded called for track ${i}`);
    };
  };

  const audioElemList = audioList.map((filename, i) => (
    <li>
      <AudioFile
        audioCtx={audioCtx}
        url={`/objects/${filename}`}
        currentTime={(nowPlayingI === i) ? currentTime : null}
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
        <p>Now playing: TODO, @ {currentTime}</p>
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
        <p>Num corrections: {numCorrections}</p>
        <p>Commit: {commit}</p>
      </header>
    </div>
  );
}

export default NowPlaying;
