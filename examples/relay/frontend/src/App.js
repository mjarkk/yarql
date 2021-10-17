import logo from './logo.svg';
import './App.css';
import { usePreloadedQuery } from 'react-relay';
import graphql from 'babel-plugin-relay/macro';

export const AppQuery = graphql`
  query AppQuery {
    users {
      email
      id
      name
    }
  }
`;

export function App({ preloadedQuery }) {
  const data = usePreloadedQuery(
    AppQuery,
    preloadedQuery,
  );

  console.log(data)

  return (
    <div className="App">
      <header className="App-header">
        <img src={logo} className="App-logo" alt="logo" />
        <p>
          Edit <code>src/App.js</code> and save to reload.
        </p>
      </header>
    </div>
  );
};
