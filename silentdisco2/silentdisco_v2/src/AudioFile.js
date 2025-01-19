import React, { useEffect, useRef, useState } from 'react';

function AudioFile({
  audioCtx,
  url,
  onEnded,
 }) {
  const [audioBuffer, setAudioBuffer] = useState(null);
  const [isPlaying, setIsPlaying] = useState(false);
  
  // We'll store our current playing AudioBufferSourceNode here (so we can stop it).
  const sourceRef = useRef(null);

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

  // Start playback
  const handlePlay = () => {
    console.log('handlePlay', audioBuffer, audioCtx);
    if (audioBuffer === null) {
      return;
    }

    // Create a new BufferSource
    console.log('creating buffer source');
    const source = audioCtx.createBufferSource();
    console.log('created buffer source', source);
    source.buffer = audioBuffer;
    source.connect(audioCtx.destination);

    source.onended = () => {
      setIsPlaying(false);
      sourceRef.current = null;
      // Call onEnded so the parent component can play the next track
      onEnded();
    };

    // TODO get offset from appOffsetInSec
    // Start now (zero delay)
    source.start(0);
  
    // Keep a reference in case we want to stop manually
    sourceRef.current = source;
    setIsPlaying(true);
  };

  // Stop playback
  const handleStop = () => {
    console.log('handleStop', sourceRef.current);
    if (sourceRef.current !== null) {
      sourceRef.current.stop();
      sourceRef.current = null;
    }
    setIsPlaying(false);
  };
  
  return (
    <button onClick={isPlaying ? handleStop : handlePlay}>
      {isPlaying ? "⏸️" : "▶️"}
    </button>
  );
}

export default AudioFile;
