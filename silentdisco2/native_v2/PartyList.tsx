// PartyList.js (React Native version)
import React, { useEffect, useState } from 'react';
import { View, Text, Button, StyleSheet } from 'react-native';

// import './PartyList.css'; // <-- Not applicable in React Native
// TODO If you need local FS, you might use 'react-native-fs' or another library.

const server = 'http://192.168.1.51:8080';

function PartyList({ partyRedirect, isHost }) {
  const [parties, setParties] = useState([]);

  // TODO: read parties from the local device FS or from an API if needed
  // For now, this example still tries an HTTP fetch.
  const fetchParties = async () => {
    try {
      // Replace '/parties' with your real endpoint
      const response = await fetch(`${server}/parties`);
      if (!response.ok) {
        console.warn('Failed to fetch parties');
        return;
      }
      const partiesArr = await response.json();
      // TODO validate each partyID with the same logic as in Party.js
      // i.e. hexPattern and length 6. Must also match logic in node_server_v1/server.js
      setParties(partiesArr);
    } catch (err) {
      console.error('Error fetching parties:', err);
    }
  };

  // TODO require roles.includes "host" (if you have roles logic in RN)
  const handleCreateParty = async () => {
    try {
      // Replace '/create-party' with your real endpoint
      const response = await fetch(`${server}/create-party`, { method: 'POST' });
      if (!response.ok) {
        console.error('Failed to create party');
        return;
      }
      const partyID = await response.text();
      // If your partyRedirect expects a function, do:
      // partyRedirect(partyID)();
      // Otherwise, if it expects to be called directly, do:
      partyRedirect(partyID);
    } catch (err) {
      console.error('Error creating party:', err);
    }
  };

  useEffect(() => {
    fetchParties();
  }, []);

  return (
    <View style={styles.container}>
      <Text style={styles.header}>Party List</Text>
      {parties.map((partyID) => (
        <View key={partyID} style={styles.partyItem}>
          <Button
            title={`Join ${partyID}`}
            // If partyRedirect returns a function: onPress={partyRedirect(partyID)}
            // Otherwise, if you can call it directly:
            onPress={() => partyRedirect(partyID)}
          />
        </View>
      ))}
      {isHost && (
        <View style={styles.buttonContainer}>
          <Button
            title="Create New Party"
            onPress={handleCreateParty}
          />
        </View>
      )}
    </View>
  );
}

export default PartyList;

const styles = StyleSheet.create({
  container: {
    // TODO Adjust this styling to suit your layout.
    flex: 1,
    padding: 16,
    justifyContent: 'center',
    alignItems: 'center',
  },
  header: {
    fontSize: 24,
    marginBottom: 16,
  },
  partyItem: {
    marginVertical: 8,
  },
  buttonContainer: {
    marginTop: 20,
  },
});

