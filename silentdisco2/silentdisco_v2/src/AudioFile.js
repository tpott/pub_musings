import React, { useEffect, useRef, useState } from 'react';


// Start playback
const realPlay = (
  audioCtx,
  audioBuffer,
  startTimeRef,
  sourceRef,
  onEnded,
  currentTime,
) => {
  console.log('realPlay', audioBuffer, audioCtx);
  if (audioBuffer === null) {
    return;
  }

  // This feels like a hack because realPlay takes too many args
  // and the useEffect has too many dependencies
  if (sourceRef.current !== null) {
    sourceRef.current.stop();
    sourceRef.current = null;
  }

  // Create a new BufferSource
  const source = audioCtx.createBufferSource();
  source.buffer = audioBuffer;
  source.connect(audioCtx.destination);

  // Mark the time we started playing
  startTimeRef.current = audioCtx.currentTime;

  // TODO get offset from appOffsetInSec
  // Start now (zero delay) from the last pausedAt offset
  source.start(0, currentTime);

  source.onended = () => {
    sourceRef.current = null;
    // Call onEnded so the parent component can play the next track?...
    onEnded();
  };

  // Keep a reference in case we want to stop manually
  sourceRef.current = source;
};

// Stop playback
const handlePause = (
  audioCtx,
  currentTime,
  startTimeRef,
  sourceRef,
) => {
  console.log('handlePause', sourceRef.current);
  if (sourceRef.current === null) {
    return;
  }

  const elapsed = audioCtx.currentTime - startTimeRef.current;
  const newPausedAt = currentTime + elapsed;

  sourceRef.current.stop();
  sourceRef.current = null;
  return newPausedAt;
};

function AudioFile({
  audioCtx,
  url,
  currentTime,
  isPlaying,
  parentPlay,
  parentPause,
  onEnded,
 }) {
  const [audioBuffer, setAudioBuffer] = useState(null);

  // We'll store our current playing AudioBufferSourceNode here (so we can stop it).
  const sourceRef = useRef(null);

  // Keep track of when we started (AudioContext currentTime)
  const startTimeRef = useRef(0);

  // Fetch and decode audio when `url` changes
  useEffect(() => {
    let isCancelled = false;

    async function loadAudio() {
      console.log('loadAudio', url);
      try {
        const response = await fetch(url);
        const arrayBuffer = await response.arrayBuffer();
        const decodedData = await audioCtx.decodeAudioData(arrayBuffer);
        if (!isCancelled) {
          setAudioBuffer(decodedData);
        }
      } catch (error) {
        console.error('Error loading audio:', error);
      }
    }
    
    loadAudio();

    // Cleanup
    return () => { isCancelled = true; };
  }, [url, audioCtx]);

  useEffect(() => {
	console.log('FML', currentTime, audioCtx, audioBuffer, onEnded, isPlaying);
    if (!isPlaying) {
      handlePause(audioCtx, currentTime, startTimeRef, sourceRef)
      return () => {}; // do nothing
    }
    realPlay(
      audioCtx,
      audioBuffer,
      startTimeRef,
      sourceRef,
      onEnded,
      currentTime,
    );
    return () => {}; // do nothing
  }, [currentTime, audioCtx, audioBuffer, onEnded, isPlaying]);

  const handlePlay = () => {
    console.log('handlePlay', audioBuffer, audioCtx, currentTime);
    if (currentTime === null) {
      currentTime = 0.0;
    }
    parentPlay(currentTime);
  };

  const handlePauseWithCallback = () => {
    const newPausedAt = handlePause(audioCtx, currentTime, startTimeRef, sourceRef);
    parentPause(newPausedAt);
  };

  // You can show the current time or progress by polling or via requestAnimationFrame
  // For a simple example, let's just compute it on each render:
  let myCurrentTime = currentTime ?? 0.0;
  if (currentTime !== null) {
    // TODO using the audioCtx.currentTime here blindly is incorrect. It
    // causes the numerator to show as the audioCtx.currentTime the first time
    // someone clicks pause, which is really just how long the page has been loaded
    myCurrentTime += audioCtx.currentTime - startTimeRef.current;
  }

  myCurrentTime = Math.min(myCurrentTime, audioBuffer?.duration || Infinity);
  
  return (
    <div>
      {myCurrentTime.toFixed(2)} / {audioBuffer && (audioBuffer.duration.toFixed(2))} &nbsp;
      <button onClick={currentTime !== null ? handlePauseWithCallback : handlePlay}>
        {currentTime !== null ? "⏸️" : "▶️"}
      </button>
    </div>
  );
}

export default AudioFile;
