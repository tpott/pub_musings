import * as git from 'isomorphic-git';
import http from "isomorphic-git/http/web";
import FS from '@isomorphic-git/lightning-fs';
import { useEffect, useState } from 'react';

import './Party.css';

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

    // TODO set a reasonable interval for pulling git
    const intervalId = setInterval(async () => {
	  // slice is because iso git apparently wants relative paths
      const gitStatus = await git.status({
        fs: myFs,
        dir: window.location.pathname,
        filepath: window.location.pathname.slice(1),
      });
      console.log('pulling', gitStatus);
      await git.pull({
        fs: myFs,
        http,
        dir: window.location.pathname,
        remote: 'origin',
        ref: 'trunk',
        author: {
          name: 'Ron Weasley',
          email: 'ron@weasly.com',
        },
      });
      console.log('done pulling');
    }, 2000);

    return () => clearInterval(intervalId);
  }, []);

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
      // TODO figure out time skew for scheduling in the future...
      await fs.promises.writeFile(
        window.location.pathname + '/now_playing.txt',
        `(${actionType}, ${audioList[i]}, ${audios[i].currentTime}, ${(new Date()).getTime() / 1000})\n` + fileBytes,
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

      setTimeout(
        () => {
          setPlayingList(playingList.map((_, k) => (actionType === "play" && i === k)));
          if (actionType === 'play') {
            audios[i].play();
          } else {
            audios[i].pause();
          }
        },
        2000, // 2 seconds
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
