/**
 * @flow
 */

/* eslint-disable */

'use strict';

/*::
import type { ConcreteRequest } from 'relay-runtime';
type TodoFragment$ref = any;
export type TodoUpdateMutationVariables = {|
  id: string,
  done?: ?boolean,
  title?: ?string,
|};
export type TodoUpdateMutationResponse = {|
  +updateTodo: {|
    +$fragmentRefs: TodoFragment$ref
  |}
|};
export type TodoUpdateMutation = {|
  variables: TodoUpdateMutationVariables,
  response: TodoUpdateMutationResponse,
|};
*/


/*
mutation TodoUpdateMutation(
  $id: ID!
  $done: Boolean
  $title: String
) {
  updateTodo(id: $id, done: $done, title: $title) {
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
var v0 = {
  "defaultValue": null,
  "kind": "LocalArgument",
  "name": "done"
},
v1 = {
  "defaultValue": null,
  "kind": "LocalArgument",
  "name": "id"
},
v2 = {
  "defaultValue": null,
  "kind": "LocalArgument",
  "name": "title"
},
v3 = [
  {
    "kind": "Variable",
    "name": "done",
    "variableName": "done"
  },
  {
    "kind": "Variable",
    "name": "id",
    "variableName": "id"
  },
  {
    "kind": "Variable",
    "name": "title",
    "variableName": "title"
  }
];
return {
  "fragment": {
    "argumentDefinitions": [
      (v0/*: any*/),
      (v1/*: any*/),
      (v2/*: any*/)
    ],
    "kind": "Fragment",
    "metadata": null,
    "name": "TodoUpdateMutation",
    "selections": [
      {
        "alias": null,
        "args": (v3/*: any*/),
        "concreteType": "Todo",
        "kind": "LinkedField",
        "name": "updateTodo",
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
    "argumentDefinitions": [
      (v1/*: any*/),
      (v0/*: any*/),
      (v2/*: any*/)
    ],
    "kind": "Operation",
    "name": "TodoUpdateMutation",
    "selections": [
      {
        "alias": null,
        "args": (v3/*: any*/),
        "concreteType": "Todo",
        "kind": "LinkedField",
        "name": "updateTodo",
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
    "cacheID": "ba868358442cfbc21acefaa4ee106dc0",
    "id": null,
    "metadata": {},
    "name": "TodoUpdateMutation",
    "operationKind": "mutation",
    "text": "mutation TodoUpdateMutation(\n  $id: ID!\n  $done: Boolean\n  $title: String\n) {\n  updateTodo(id: $id, done: $done, title: $title) {\n    ...TodoFragment\n    id\n  }\n}\n\nfragment TodoFragment on Todo {\n  id\n  title\n  done\n}\n"
  }
};
})();
// prettier-ignore
(node/*: any*/).hash = '9a5204b230425bc65f5bea936e8b77df';

module.exports = node;
