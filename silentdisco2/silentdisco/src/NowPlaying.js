import * as git from 'isomorphic-git';
import http from 'isomorphic-git/http/web';
import FS from '@isomorphic-git/lightning-fs';
import { useEffect, useState } from 'react';

import './Party.css';

const acceptableDiffSec = 0.1; // 100 milliseconds
const clickDelaySec = 0.2; // 200 milliseconds
const correctionsEnabled = false;
const endingBufferSec = 0.1; // 100 milliseconds

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
  setPlayState,
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
    const nowInSec = ((new Date()).getTime() / 1000) - appOffsetInSec;
    const diff = startAsOf - nowInSec;

    // TODO use the react dom elements from state?
    const audios = document.getElementsByTagName('audio');
    const updatePlaying = () => {
      // TODO why do we have this diff?
      if (actionType === 'play' && diff <= 0) {
        // audios[i].currentTime = startTime;
        // audios[i].currentTime = startTime + diff;
        audios[i].currentTime = startTime - diff;
      } else {
        audios[i].currentTime = startTime;
      }
      console.log('going to', actionType, i, audioList[i]);
      if (actionType === 'play') {
        setNowPlayingI(i);
        setPlayState([startTime, startAsOf]);
      } else {
        setNowPlayingI(-1);
        setPlayState([null, null]);
      }
      audioList.map((_, k) => {
        if (actionType === 'play' && i === k) {
          audios[k].play();
        } else {
          audios[k].pause();
        }
      });
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
  appOffsetInSec,
  commit,
  isHost,
  partyID,
  partyRedirect,
  setCommit,
  setListParty,
}) {

  // TODO use the react dom elements from state?
  const [audioList, setAudioList] = useState([]);
  const [nowPlayingI, setNowPlayingI] = useState(-1);
  const [currentTime, setCurrentTime] = useState(null);
  // playState[0] is seconds offset into now playing audio
  // playState[1] is seconds offset from unix epoch, i.e. UTC
  const [playState, setPlayState] = useState([null, null]);
  const [numCorrections, setNumCorrections] = useState(0);
  const [fs, setFS] = useState(null);

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
      setPlayState,
    );
    myUpdateAudio();

    return doNothing;
  }, [appOffsetInSec, fs, commit]);

  // TODO DJ's name... idk if there's multiple DJs
  // TODO my roles... listener (everyone...), host, DJ
  // TODO my name

  const playOrPause = (actionType, i) => {
    return async () => {
      // TODO use the react dom elements from state?
      const audios = document.getElementsByTagName('audio');
      const nowInSec = ((new Date()).getTime() / 1000) - appOffsetInSec;
      console.log('clicked', actionType, audioList[i], audios[i].currentTime, nowInSec);
      if (fs == null) {
        return;
      }

      const currentTime = audios[i].currentTime;
      const targetInSec = nowInSec + clickDelaySec;
      // TODO figure out time skew for scheduling in the future...
      await fs.promises.writeFile(
        window.location.pathname + '/now_playing.txt',
        `(${actionType}, ${i}, ${audioList[i]}, ${currentTime}, ${targetInSec})\n`,
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

  const accident = (i) => {
    return (evt) => {
      // TODO use the react dom elements from state?
      const audios = document.getElementsByTagName('audio');
      if (evt.type === 'play' && i !== nowPlayingI) {
        console.log('accidental play', audios[i].currentTime, audios[i].duration);
        audios[i].pause();
      } else if (evt.type === 'pause' && i === nowPlayingI) {
        if (Math.abs(audios[i].currentTime - audios[i].duration) < endingBufferSec) {
          console.log('song ended', audios[i].currentTime, audios[i].duration);
          // TODO play next song
          setNowPlayingI(-1);
          setPlayState([null, null]);
          return; // skip, this wasn't an accident
        }
        console.log('accidental pause', audios[i].currentTime, audios[i].duration);
        audios[i].play();
      }
    };
  };

  const updateCurrentTime = (i) => {
    return () => {
      // TODO use the react dom elements from state?
      const audios = document.getElementsByTagName('audio');
      if (nowPlayingI === -1 || nowPlayingI >= audios.length) {
        return;
      }

      const currentTime = audios[nowPlayingI].currentTime;
      setCurrentTime(currentTime);

      const nowInSec = ((new Date()).getTime() / 1000) - appOffsetInSec;
      const expectedTime = nowInSec - playState[1] + playState[0];

      if (Math.abs(expectedTime - currentTime) < acceptableDiffSec) {
        return;
      }
      setNumCorrections(numCorrections + 1);
      console.log(`expected ${expectedTime} vs actual ${currentTime}`);

      if (!correctionsEnabled) {
        return;
      }

      if (expectedTime > currentTime) {
        // jump ahead to catch up
        audios[nowPlayingI].currentTime = expectedTime;
        setCurrentTime(expectedTime);
      } else {
        setNowPlayingI(-1);
        audios[nowPlayingI].pause();
        setTimeout(currentTime - expectedTime, () => {
          setNowPlayingI(i);
          audios[nowPlayingI].play();
        });
      }
    };
  };

  const audioElemList = audioList.map((filename, i) => (
    <li>
      <audio controls preload='auto' onPlay={accident(i)} onPause={accident(i)} onTimeUpdate={updateCurrentTime(i)}>
        <source src={`/objects/${filename}`} />
      </audio>
      {nowPlayingI === i ? <button onClick={playOrPause('pause', i)}>⏸️</button> : <button onClick={playOrPause('play', i)}>▶️</button> }
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
        <p>{commit}</p>
      </header>
    </div>
  );
}

export default NowPlaying;
