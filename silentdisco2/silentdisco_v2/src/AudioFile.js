import React, { useEffect, useRef, useState } from 'react';

// from https://stackoverflow.com/questions/55187563/determine-which-dependency-array-variable-caused-useeffect-hook-to-fire
const usePrevious = (value, initialValue) => {
  const ref = useRef(initialValue);
  useEffect(() => {
    ref.current = value;
  });
  return ref.current;
};

const useEffectDebugger = (effectHook, dependencies, dependencyNames = []) => {
  const previousDeps = usePrevious(dependencies, []);

  const changedDeps = dependencies.reduce((accum, dependency, index) => {
    if (dependency !== previousDeps[index]) {
      const keyName = dependencyNames[index] || index;
      return {
        ...accum,
        [keyName]: {
          before: previousDeps[index],
          after: dependency
        }
      };
    }

    return accum;
  }, {});

  if (Object.keys(changedDeps).length) {
    console.log('[use-effect-debugger] ', changedDeps);
  }

  useEffect(effectHook, dependencies);
};
// done from stackoverflow

// Start playback
const realPlay = (
  audioCtx,
  audioBuffer,
  startTimeRef,
  sourceRef,
  songCtxOffset,
  songCurrentTime,
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

  // songCurrentTime is the number of seconds since the beginning of the song
  // songCtxOffset is the number of seconds since audioCtx.currentTime for `when`
  // the song should start playing. If its <= audioCtx.currentTime, it will play
  // *now*. If it's > audioCtx.currentTime, then it will wait until
  // audioCtx.currentTime == songCtxOffset.
  try {
    source.start(songCtxOffset, songCurrentTime);
  } catch (e) {
    console.log('failed to start audio in realPlay!', e);
  }

  source.onended = () => {
    sourceRef.current = null;
    // TODO onEnded was causing unnecessary react rerenders
    // Call onEnded so the parent component can play the next track?...
    // onEnded();
  };

  // Keep a reference in case we want to stop manually
  sourceRef.current = source;
};

// Stop playback
const handlePause = (
  audioCtx,
  songCurrentTime,
  startTimeRef,
  sourceRef,
) => {
  console.log('handlePause', sourceRef.current, songCurrentTime);
  if (sourceRef.current === null) {
    return;
  }

  const elapsed = audioCtx.currentTime - startTimeRef.current;
  const newPausedAt = songCurrentTime + elapsed;

  try {
    sourceRef.current.stop();
  } catch (e) {
    console.log('failed to stop', e);
  }
  sourceRef.current = null;
  return newPausedAt;
};

function AudioFile({
  audioCtx,
  url,
  audioCtxOffset,
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

  useEffectDebugger(() => {
    // TODO onEnded was causing unnecessary react rerenders
    console.log(`useEffectDebugger currentTime=${currentTime}, isPlaying=${isPlaying}, audioCtxOffset=${audioCtxOffset}, audioCtx=${audioCtx}, audioBuffer=${audioBuffer}`);
    if (!isPlaying) {
      handlePause(audioCtx, currentTime, startTimeRef, sourceRef)
      return () => {}; // do nothing
    }
    realPlay(
      audioCtx,
      audioBuffer,
      startTimeRef,
      sourceRef,
      audioCtxOffset,
      currentTime,
    );
    return () => {}; // do nothing
  }, [currentTime, isPlaying, audioCtxOffset, audioCtx, audioBuffer]);

  const handlePlay = () => {
    console.log('handlePlay', audioBuffer, audioCtx, currentTime);
    parentPlay(currentTime);
  };

  const handlePauseWithCallback = () => {
    const newPausedAt = handlePause(audioCtx, currentTime, startTimeRef, sourceRef);
    parentPause(newPausedAt);
  };

  // TODO show the current time progress by polling or via requestAnimationFrame

  return (
    <div>
      {currentTime?.toFixed(2)} / {audioBuffer && (audioBuffer.duration.toFixed(2))} &nbsp;
      <button onClick={isPlaying ? handlePauseWithCallback : handlePlay}>
        {isPlaying ? "⏸️" : "▶️"}
      </button>
    </div>
  );
}

export default AudioFile;
