/**
 * @flow
 */

/* eslint-disable */

'use strict';

/*::
import type { ConcreteRequest } from 'relay-runtime';
type TodoFragment$ref = any;
export type AppCreateTodoMutationVariables = {|
  title: string
|};
export type AppCreateTodoMutationResponse = {|
  +createTodo: {|
    +$fragmentRefs: TodoFragment$ref
  |}
|};
export type AppCreateTodoMutation = {|
  variables: AppCreateTodoMutationVariables,
  response: AppCreateTodoMutationResponse,
|};
*/


/*
mutation AppCreateTodoMutation(
  $title: String!
) {
  createTodo(title: $title) {
    ...TodoFragment
    id
  }
}

fragment TodoFragment on Todo {
  id
  title
  done
}
*/

const node/*: ConcreteRequest*/ = (function(){
var v0 = [
  {
    "defaultValue": null,
    "kind": "LocalArgument",
    "name": "title"
  }
],
v1 = [
  {
    "kind": "Variable",
    "name": "title",
    "variableName": "title"
  }
];
return {
  "fragment": {
    "argumentDefinitions": (v0/*: any*/),
    "kind": "Fragment",
    "metadata": null,
    "name": "AppCreateTodoMutation",
    "selections": [
      {
        "alias": null,
        "args": (v1/*: any*/),
        "concreteType": "Todo",
        "kind": "LinkedField",
        "name": "createTodo",
        "plural": false,
        "selections": [
          {
            "args": null,
            "kind": "FragmentSpread",
            "name": "TodoFragment"
          }
        ],
        "storageKey": null
      }
    ],
    "type": "MethodRoot",
    "abstractKey": null
  },
  "kind": "Request",
  "operation": {
    "argumentDefinitions": (v0/*: any*/),
    "kind": "Operation",
    "name": "AppCreateTodoMutation",
    "selections": [
      {
        "alias": null,
        "args": (v1/*: any*/),
        "concreteType": "Todo",
        "kind": "LinkedField",
        "name": "createTodo",
        "plural": false,
        "selections": [
          {
            "alias": null,
            "args": null,
            "kind": "ScalarField",
            "name": "id",
            "storageKey": null
          },
          {
            "alias": null,
            "args": null,
            "kind": "ScalarField",
            "name": "title",
            "storageKey": null
          },
          {
            "alias": null,
            "args": null,
            "kind": "ScalarField",
            "name": "done",
            "storageKey": null
          }
        ],
        "storageKey": null
      }
    ]
  },
  "params": {
    "cacheID": "98f73e54df3110673f78fbab6f8017f1",
    "id": null,
    "metadata": {},
    "name": "AppCreateTodoMutation",
    "operationKind": "mutation",
    "text": "mutation AppCreateTodoMutation(\n  $title: String!\n) {\n  createTodo(title: $title) {\n    ...TodoFragment\n    id\n  }\n}\n\nfragment TodoFragment on Todo {\n  id\n  title\n  done\n}\n"
  }
};
})();
// prettier-ignore
(node/*: any*/).hash = '392c4e637c6c2e4a26107d120a738312';

module.exports = node;
