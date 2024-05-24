import * as git from 'isomorphic-git';
import http from 'isomorphic-git/http/web';
import FS from '@isomorphic-git/lightning-fs';
import { useEffect, useState } from 'react';

import './Party.css';

const clickDelayMs = 300; // 300 milliseconds
const gitIntervalMs = 16000; // 16 seconds
const endingBufferSec = 0.1; // 100 milliseconds

// doNothing is an empty cleanup function to make react useEffect happy
const doNothing = () => {};

const fetchAudioObjects = async (
  fs,
  setAudioList,
  setPlayingList,
) => {
  const fileBytes = await fs.promises.readFile(window.location.pathname + '/objects.txt');
  const audios = (new TextDecoder()).decode(fileBytes)
    .split('\n')
    .filter(line => line !== '')
    .map(line => {
      const audioObj = JSON.parse(line)
      return `${audioObj['sha256']}.${audioObj['filetype']}`;
    });
  setAudioList(audios);
  setPlayingList(audios.map(() => false));
};

const myAsyncPullGit = (
  fs,
  commit,
  setCommit,
  audioList,
  setAudioList,
  playingList,
  setPlayingList,
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
    // Don't setCommit(result.oid) just yet.. wait till we update other state

    // I'm not entirely sure why isogit requires us to checkout the branch we just
    // updated with the merge...
    await git.checkout({
      fs,
      dir: window.location.pathname,
    });

    console.log('!alreadyMerged, need to schedule something?');

    fetchAudioObjects(fs, setAudioList, setPlayingList);

    const fileBytes = await fs.promises.readFile(window.location.pathname + '/now_playing.txt');
    console.log('read', fileBytes);
    // TODO don't decode the entire file?
    const nowPlaying = (new TextDecoder()).decode(fileBytes);
    // TODO do we need to handle more lines?
    const lines = nowPlaying.split('\n');
    if (lines.length === 0 || lines[0].length === 0) {
      console.log('empty lines or empty first line', lines);
      return;
    }
    const line = lines[0];
    console.log(nowPlaying);
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
    const nowInSec = (new Date()).getTime() / 1000;
    const diff = parseFloat(fields[4]) - nowInSec;

    // TODO use the react dom elements from state?
    const audios = document.getElementsByTagName('audio');
    const updatePlaying = () => {
      if (actionType === 'play' && diff <= 0) {
        audios[i].currentTime = parseFloat(fields[3]) - diff;
      } else {
        audios[i].currentTime = parseFloat(fields[3]);
      }
      console.log('going to', actionType, i, audioList[i]);
      setPlayingList(playingList.map((_, k) => (actionType === 'play' && i === k)));
      if (actionType === 'play') {
        audios[i].play();
      } else {
        audios[i].pause();
      }
    };

    if (diff > 0) {
      console.log('scheduling action for future', diff, nowInSec, fields[4]);
      setTimeout(updatePlaying, diff * 1000,);
    } else {
      console.log('TODO scheduled action from past', diff);
      updatePlaying();
    }

    // Force the component to re-render
    setCommit(result.oid);
  };
};


function NowPlaying({ isHost, partyID, partyRedirect, setListParty }) {
  // TODO use the react dom elements from state?
  const [audioList, setAudioList] = useState([]);
  const [playingList, setPlayingList] = useState([]);
  const [fs, setFS] = useState(null);
  const [wsClient, setWSClient] = useState(null);
  const [commit, setCommit] = useState(null);

  useEffect(() => {
    // window.location.pathname == '/party/:partyID'
    const myFs = new FS('fs');
    setFS(myFs);
    fetchAudioObjects(myFs, setAudioList, setPlayingList);

    // const getCurrent = async () => {
      // return await git.resolveRef({ fs, dir: window.location.pathname, ref: 'HEAD' });
    // };
    // const currentCommit = getCurrent();
    // setCommit(currentCommit);
  }, []);

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
    const client = new WebSocket(`${protocol}://${window.location.hostname}:${port}/ws`);
    client.onopen = () => {
      console.log('WebSocket Client Connected', client);
    };
    client.onclose = () => {
      console.log('WebSocket Client Disconnected');
    };

    // don't set client.onmessage here. we need asyncPullGit for that
    setWSClient(client);
    return () => {
      // TODO when should we close the websocket? not doing at all will lead to
      // memory leaks
      // client.close();
    };
  }, [wsClient]);

  useEffect(() => {
    if (fs == null) {
      return doNothing;
    }

    const asyncPullGit = myAsyncPullGit(
      fs,
      commit,
      setCommit,
      audioList,
      setAudioList,
      playingList,
      setPlayingList,
    );

    // TODO set a reasonable interval for pulling git
    const intervalId = setInterval(asyncPullGit, gitIntervalMs);

    if (wsClient == null) {
      return () => {
        clearInterval(intervalId);
      };
    }

    wsClient.onmessage = (e) => {
      console.log('Received websocket message: ', e.data);
      if (e.data === 'please-pull') {
        asyncPullGit();
      }
    };

    return () => {
      clearInterval(intervalId);
    };
  }, [audioList, playingList, fs, commit, wsClient]);

  // TODO DJ's name... idk if there's multiple DJs
  // TODO my roles... listener (everyone...), host, DJ
  // TODO my name

  const playOrPause = (actionType, i) => {
    return async () => {
      // TODO use the react dom elements from state?
      const audios = document.getElementsByTagName('audio');
      console.log('clicked', actionType, audioList[i], audios[i].currentTime, (new Date()).getTime() / 1000);
      if (fs == null) {
        return;
      }

      const currentTime = audios[i].currentTime;
      const fileBytes = await fs.promises.readFile(window.location.pathname + '/now_playing.txt');
      const nowInSec = (new Date()).getTime() / 1000;
      const targetInSec = nowInSec + (clickDelayMs / 1000);
      // TODO figure out time skew for scheduling in the future...
      await fs.promises.writeFile(
        window.location.pathname + '/now_playing.txt',
        `(${actionType}, ${i}, ${audioList[i]}, ${currentTime}, ${targetInSec})\n` + fileBytes,
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

      // TODO move this into a function
      const updatedNow = (new Date()).getTime() / 1000;
      const diff = targetInSec - updatedNow;
      const updatePlaying = () => {
        setPlayingList(playingList.map((_, k) => (actionType === 'play' && i === k)));
        audios[i].currentTime = currentTime;
        if (actionType === 'play') {
          audios[i].play();
        } else {
          audios[i].pause();
        }
      };

      if (diff > 0) {
        console.log('self scheduling action for future', diff, updatedNow);
        setTimeout(
          updatePlaying,
          diff * 1000,
        );

      } else {
        console.log('self scheduled action from past', diff);
        // TODO calc diff in audios[i].currentTime and fields[3]
        updatePlaying();
      }

      setTimeout(
        () => {
          setPlayingList(playingList.map((_, k) => (actionType === 'play' && i === k)));
          if (actionType === 'play') {
            audios[i].play();
          } else {
            audios[i].pause();
          }
        },
        clickDelayMs,
      );
      setCommit(sha);
    };
  };

  const accident = (i) => {
    return (evt) => {
      // TODO use the react dom elements from state?
      const audios = document.getElementsByTagName('audio');
      if (evt.type === 'play' && !playingList[i]) {
        console.log('accidental play', audios[i].currentTime, audios[i].duration);
        audios[i].pause();
      } else if (evt.type === 'pause' && playingList[i]) {
        if (Math.abs(audios[i].currentTime - audios[i].duration) < endingBufferSec) {
          console.log('song ended', audios[i].currentTime, audios[i].duration);
          // TODO play next song
          setPlayingList(playingList.map(() => false));
          return; // skip, this wasn't an accident
        }
        console.log('accidental pause', audios[i].currentTime, audios[i].duration);
        audios[i].play();
      }
    };
  };

  const audioElemList = audioList.map((filename, i) => (
    <li>
      <audio controls preload='auto' onPlay={accident(i)} onPause={accident(i)}>
        <source src={`/objects/${filename}`} />
      </audio>
      {playingList[i] ? <button onClick={playOrPause('pause', i)}>⏸️</button> : <button onClick={playOrPause('play', i)}>▶️</button> }
    </li>
  ));

  const uploadFile = async (e) => {
    e.preventDefault();
    const formData = new FormData();
    // TODO use the react state instead of finding the html element...
    const elem = document.getElementById('fileUpload');
    console.log('uploading', elem);
    formData.append('file', elem.files[0]);
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
        <p>Now playing: TODO</p>
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
        <p>{commit}</p>
      </header>
    </div>
  );

}

export default NowPlaying;
