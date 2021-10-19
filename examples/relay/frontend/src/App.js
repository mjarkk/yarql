import { usePreloadedQuery, commitMutation, useRelayEnvironment } from 'react-relay'
import graphql from 'babel-plugin-relay/macro'
import { Todo } from './Todo'
import { useState } from 'react'

export const AppQuery = graphql`
  query AppQuery {
    todos {
      ...TodoFragment
    }
  }
`;

export const AppCreateTodoMutation = graphql`
  mutation AppCreateTodoMutation($title: String!) {
    createTodo(title: $title) {
      ...TodoFragment
    }
  }
`

export function App({ queryRef, refresh }) {
  const environment = useRelayEnvironment()
  const data = usePreloadedQuery(
    AppQuery,
    queryRef,
  );
  const [newTodo, setNewTodo] = useState('');

  const submitCreateNewTodo = e => {
    e.preventDefault()
    commitMutation(environment, {
      mutation: AppCreateTodoMutation,
      variables: { title: newTodo },
    })
    setNewTodo('')
    refresh()
  }

  return (
    <div className="App">
      <form onSubmit={submitCreateNewTodo}>
        <button type="submit">Add Todo</button>
        <input
          value={newTodo}
          onChange={e => setNewTodo(e.target.value)}
        />
      </form>

      <div className="todos">
        {data.todos.map(todo =>
          <Todo key={todo.__id} todo={todo} />
        )}
      </div>
    </div>
  );
};
