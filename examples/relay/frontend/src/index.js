import React, { Suspense } from 'react';
import ReactDOM from 'react-dom';
import './index.css';
import { App, AppQuery } from './App';
import reportWebVitals from './reportWebVitals';
import { RelayEnvironmentProvider, loadQuery } from 'react-relay/hooks';
import RelayEnvironment from "./RelayEnvironment"

const preloadedQuery = loadQuery(RelayEnvironment, AppQuery, {});

ReactDOM.render(
  <React.StrictMode>
    <RelayEnvironmentProvider environment={RelayEnvironment}>
      <Suspense fallback={'Loading...'}>
        <App preloadedQuery={preloadedQuery} />
      </Suspense>
    </RelayEnvironmentProvider>
  </React.StrictMode>,
  document.getElementById('root')
);

// If you want to start measuring performance in your app, pass a function
// to log results (for example: reportWebVitals(console.log))
// or send to an analytics endpoint. Learn more: https://bit.ly/CRA-vitals
reportWebVitals();
