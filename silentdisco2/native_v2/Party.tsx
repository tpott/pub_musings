// Party.js (React Native version)
import React, { useEffect, useState } from 'react';
import { View, Text, Button, StyleSheet } from 'react-native';
// TODO There's no .css in React Native. Use StyleSheet or other styling solutions instead.
// import './Party.css';

import AudioList from './AudioList'; // <-- This needs to be adapted to React Native as well

function Party({
  audioCtx,
  appOffsetInSec,
  commit,
  isHost,
  partyID,
  partyRedirect,
  setCommit,
}) {
  const [isValidPartyID, setIsValidPartyID] = useState(false);
  const [participantID, setParticipantID] = useState(null);
  const [listParty, setListParty] = useState(null);

  // TODO nowPlaying, so we know what song/video is playing
  // TODO DJ's name... idk if there's multiple DJs
  // TODO my roles... listener (everyone...), host, DJ
  // TODO my name

  useEffect(() => {
    // TODO move this to a lib to share with server.js (if needed)
    const hexPattern = /^[0-9A-Fa-f]{6}$/i;
    const isValid = partyID.length === 6 && hexPattern.test(partyID);
    setIsValidPartyID(isValid);
  }, [partyID]);

  if (!isValidPartyID) {
    return (
      <View style={styles.container}>
        <Text style={styles.headerText}>Invalid Party ID</Text>
        <Button
          title="Leave Party"
          onPress={partyRedirect(null)} // Make sure partyRedirect is returning a function
        />
      </View>
    );
  } else if (participantID != null) {
    // <Participant> equivalent
    return (
      <View style={styles.container}>
        <Text style={styles.text}>Name: No Name // TODO</Text>
        <Text style={styles.text}>Roles: // TODO</Text>
        <Text style={styles.text}>TODO if (roles.includes "host") "Invite to DJ"</Text>
        <Button
          title="Participants list"
          onPress={() => setParticipantID(null)}
        />
      </View>
    );
  } else if (listParty ?? false) {
    // <ParticipantList> equivalent
    return (
      <View style={styles.container}>
        <Button
          title="No name"
          onPress={() => setParticipantID("abcdef")}
        />
        <Button
          title="Now Playing"
          onPress={() => setListParty(null)}
        />
      </View>
    );
  } else {
    // Show the AudioList
    return (
      <AudioList
        audioCtx={audioCtx}
        appOffsetInSec={appOffsetInSec}
        commit={commit}
        isHost={isHost}
        partyID={partyID}
        partyRedirect={partyRedirect}
        setCommit={setCommit}
        setListParty={setListParty}
      />
    );
  }
}

export default Party;

// Example basic styles for React Native:
const styles = StyleSheet.create({
  container: {
    // TODO Adjust or design your layout
    flex: 1,
    padding: 16,
    justifyContent: 'center',
    alignItems: 'center'
  },
  headerText: {
    fontSize: 24,
    marginBottom: 16,
  },
  text: {
    fontSize: 16,
    marginVertical: 4,
  },
});

