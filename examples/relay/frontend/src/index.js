import React, { Suspense, useCallback } from 'react';
import ReactDOM from 'react-dom';
import './index.css';
import { App, AppQuery } from './App';
import { RelayEnvironmentProvider, useQueryLoader, loadQuery } from 'react-relay/hooks';
import RelayEnvironment from "./RelayEnvironment"

const initialQueryRef = loadQuery(
  RelayEnvironment,
  AppQuery,
  null,
);

ReactDOM.render(
  <React.StrictMode>
    <RelayEnvironmentProvider environment={RelayEnvironment}>
      <AppWrapper initialQueryRef={initialQueryRef} />
    </RelayEnvironmentProvider>
  </React.StrictMode >,
  document.getElementById('root')
);

function AppWrapper({ initialQueryRef }) {
  const [queryRef, loadQuery] = useQueryLoader(
    AppQuery,
    initialQueryRef,
  );

  const refresh = useCallback(() => {
    loadQuery({}, { fetchPolicy: 'network-only' });
  }, [loadQuery]);

  return (
    <Suspense fallback={'Loading...'}>
      <App
        queryRef={queryRef}
        refresh={refresh}
      />
    </Suspense>
  )
}
