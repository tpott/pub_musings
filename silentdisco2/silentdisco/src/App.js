import { BrowserRouter as Router, Route, Routes} from 'react-router-dom';

import Party from './Party';
import PartyList from './PartyList';

function App() {
  return (
    <Router>
      <Routes>
        <Route exact path="/" element={<PartyList />} />
        <Route path="/party/:partyID" element={<Party />} />
      </Routes>
    </Router>
  );
}

export default App;
