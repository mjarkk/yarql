import { useState } from 'react'
import { useFragment, commitMutation, useRelayEnvironment } from 'react-relay'
import graphql from 'babel-plugin-relay/macro'

export const TodoFragment = graphql`
  fragment TodoFragment on Todo {
    id
    title
    done
  }
`;

const TodoUpdateMutation = graphql`
  mutation TodoUpdateMutation($id: ID!, $done: Boolean, $title: String) {
    updateTodo(id: $id, done: $done, title: $title) {
        ...TodoFragment
    }
  }
`;

const TodoDeleteMutation = graphql`
  mutation TodoDeleteMutation($id: ID!) {
    deleteTodo(id: $id) {
        id
    }
  }
`;


export function Todo({ todo }) {
    const data = useFragment(TodoFragment, todo)
    const environment = useRelayEnvironment()
    const [updateTitle, setUpdateTitle] = useState(undefined)

    const update = (done, title) => {
        commitMutation(environment, {
            mutation: TodoUpdateMutation,
            variables: {
                id: data.id,
                done,
                title,
            },
        })
    }

    const delete_ = () => {
        const { id } = data
        commitMutation(environment, {
            mutation: TodoDeleteMutation,
            variables: { id },
            updater: store => {
                const storeRoot = store.getRoot()
                storeRoot.setLinkedRecords(
                    storeRoot.getLinkedRecords('todos')
                        .filter(x => x.getDataID() !== id),
                    'todos',
                )
            },
        })
    }

    const updateTitleSubmit = e => {
        e.preventDefault()
        update(data.done, updateTitle)
        setUpdateTitle(undefined)
    }

    if (!data) return undefined;

    return (
        updateTitle === undefined ?
            <div className="todo">
                <input
                    type="checkbox"
                    checked={data.done}
                    onChange={() => update(!data.done, data.title)}
                />
                <button onClick={() => setUpdateTitle(data.title)}>Change</button>
                <button onClick={() => delete_()}>Delete</button>
                {data.title}
            </div>
            :
            <form className="todo" onSubmit={updateTitleSubmit}>
                <button type="submit">Update</button>
                <input
                    onChange={e => setUpdateTitle(e.target.value)}
                    value={updateTitle}
                />
            </form>
    )
}
