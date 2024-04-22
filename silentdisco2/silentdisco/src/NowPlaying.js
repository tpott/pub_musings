import * as git from 'isomorphic-git';
import http from "isomorphic-git/http/web";
import FS from '@isomorphic-git/lightning-fs';
import { useEffect, useState } from 'react';

import './Party.css';

const clickDelayMs = 3000; // 3 seconds

// doNothing is an empty cleanup function to make react useEffect happy
const doNothing = () => {};

function NowPlaying({ partyID, partyRedirect, setListParty }) {
  // TODO use the react dom elements from state?
  const [audioList, setAudioList] = useState([]);
  const [playingList, setPlayingList] = useState([]);
  const [fs, setFS] = useState(null);

  useEffect(() => {
    setAudioList(["e_J14fbBluE.mp3"]);
    setPlayingList([false]);

    // window.location.pathname == '/party/:partyID'
    const myFs = new FS('fs');
    console.log('done initializing fs');
    setFS(myFs);
  }, []);

  useEffect(() => {
    if (fs == null) {
      return doNothing;
    }

    // TODO set a reasonable interval for pulling git
    const intervalId = setInterval(async () => {
      // slice is because iso git apparently wants relative paths
      const gitStatus = await git.status({
        fs,
        dir: window.location.pathname,
        filepath: window.location.pathname.slice(1),
      });
      console.log('fetching', gitStatus);
      await git.fetch({
        fs,
        http,
        dir: window.location.pathname,
        remote: 'origin',
        ref: 'trunk',
      });
      console.log('done fetching');
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
      console.log(result);

      if (result.alreadyMerged) {
        return;
      }

      const fileBytes = await fs.promises.readFile(window.location.pathname + '/now_playing.txt');
      // TODO don't decode the entire file?
      const nowPlaying = (new TextDecoder()).decode(fileBytes);
      // TODO do we need to handle more lines?
      const lines = nowPlaying.split('\n');
      if (lines.length === 0 || lines[0].length === 0) {
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
      const fields = line.split(', ');
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
      if (diff > 0) {
        console.log('scheduling action for future', diff, nowInSec, fields[4]);

        setTimeout(
          () => {
            // TODO use the react dom elements from state?
            const audios = document.getElementsByTagName('audio');
            audios[i].currentTime = parseFloat(fields[3]);
            setPlayingList(playingList.map((_, k) => (actionType === "play" && i === k)));
            if (actionType === 'play') {
              audios[i].play();
            } else {
              audios[i].pause();
            }
          },
          diff * 1000,
        );

      } else {
        console.log('TODO scheduled action from past', diff);
        // TODO calc diff in audios[i].currentTime and fields[3]
      }

    }, 2000);

    return () => clearInterval(intervalId);
  }, [fs]);


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

      const fileBytes = await fs.promises.readFile(window.location.pathname + '/now_playing.txt');
      const nowInSec = (new Date()).getTime() / 1000;
      // TODO figure out time skew for scheduling in the future...
      await fs.promises.writeFile(
        window.location.pathname + '/now_playing.txt',
        `(${actionType}, ${i}, ${audioList[i]}, ${audios[i].currentTime}, ${nowInSec + (clickDelayMs / 1000)})\n` + fileBytes,
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

      // TODO move this into a function
      const updatedNow = (new Date()).getTime() / 1000;
      const diff = (nowInSec + (clickDelayMs / 1000)) - updatedNow;
      if (diff > 0) {
        console.log('self scheduling action for future', diff, updatedNow);

        setTimeout(
          () => {
            setPlayingList(playingList.map((_, k) => (actionType === "play" && i === k)));
            audios[i].currentTime = audios[i].currentTime;
            if (actionType === 'play') {
              audios[i].play();
            } else {
              audios[i].pause();
            }
          },
          diff * 1000,
        );

      } else {
        console.log('TODO self scheduled action from past', diff);
        // TODO calc diff in audios[i].currentTime and fields[3]
      }



      setTimeout(
        () => {
          setPlayingList(playingList.map((_, k) => (actionType === "play" && i === k)));
          if (actionType === 'play') {
            audios[i].play();
          } else {
            audios[i].pause();
          }
        },
        clickDelayMs,
      );
    };
  };

  const accident = (i) => {
    return (evt) => {
      // TODO use the react dom elements from state?
      const audios = document.getElementsByTagName('audio');
      if (evt.type === 'play' && !playingList[i]) {
        console.log('accidental play');
        audios[i].pause();
      } else if (evt.type === 'pause' && playingList[i]) {
        console.log('accidental pause');
        audios[i].play();
      }
    };
  };

  const audioElemList = audioList.map((filename, i) => (
    <>
      <audio controls preload="auto" onPlay={accident(i)} onPause={accident(i)}>
        <source src={`/${filename}`} />
      </audio>
      {playingList[i] ? <span onClick={playOrPause("pause", i)}>⏸️</span> : <span onClick={playOrPause("play", i)}>▶️</span> }
    </>
  ));

  // TODO if roles includes "dj" then replace "Leave Party" button with "Stop DJ"
  return (
    <div className="Party">
      <header className="Party-header">
        <p>Welcome to {partyID}</p>
        <p>Now playing: TODO</p>
        {audioElemList}
        <p>My name: TODO</p>
        <p><button onClick={() => setListParty(true)}>Participants list</button></p>
        <p><button onClick={partyRedirect(null)}>Leave Party</button></p>
      </header>
    </div>
  );

}

export default NowPlaying;
