import './PartyList.css';

function PartyList() {
  // TODO require roles.includes "host"
  const handleCreateParty = async () => {
    try {
      const response = await fetch('/create-party', { method: 'POST' });
      if (!response.ok) {
        console.error('Failed to create party');
        return;
      }
      // Redirect to the newly created party
      const partyId = await response.text();
      window.location.href = `/party/${partyId}`;
    } catch (err) {
      console.error('Error creating party:', err);
    }
  };

  const redirect = () => {
    window.location.href = "/party/todoXY";
  };

  // TODO only include create new party button if (roles.includes "host")
  return (
    <div className="PartyList">
      <header className="PartyList-header">
        <p>Party List</p>
        <p>* <button onClick={redirect}>TODO</button></p>
        <p><button onClick={handleCreateParty}>Create New Party</button></p>
      </header>
    </div>
  );
}

export default PartyList;
