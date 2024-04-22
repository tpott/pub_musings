import { useEffect, useState } from 'react';

import './Party.css';

function NowPlaying({ partyID, partyRedirect, setListParty }) {
  // TODO use the react dom elements from state?
  const [audioList, setAudioList] = useState([]);
  const [playingList, setPlayingList] = useState([]);

  useEffect(() => {
    setAudioList(["e_J14fbBluE.mp3"]);
    setPlayingList([false]);
  }, []);

  // TODO DJ's name... idk if there's multiple DJs
  // TODO my roles... listener (everyone...), host, DJ
  // TODO my name

  const playOrPause = (actionType, i) => {
    return () => {
      // TODO use the react dom elements from state?
      const audios = document.getElementsByTagName('audio');
      console.log('clicked', actionType, audioList[i], audios[i].currentTime, (new Date()).getTime() / 1000);
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
