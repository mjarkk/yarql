/**
 * @flow
 */

/* eslint-disable */

'use strict';

/*::
import type { ConcreteRequest } from 'relay-runtime';
export type TodoDeleteMutationVariables = {|
  id: string
|};
export type TodoDeleteMutationResponse = {|
  +deleteTodo: ?$ReadOnlyArray<{|
    +id: string
  |}>
|};
export type TodoDeleteMutation = {|
  variables: TodoDeleteMutationVariables,
  response: TodoDeleteMutationResponse,
|};
*/


/*
mutation TodoDeleteMutation(
  $id: ID!
) {
  deleteTodo(id: $id) {
    id
  }
}
*/

const node/*: ConcreteRequest*/ = (function(){
var v0 = [
  {
    "defaultValue": null,
    "kind": "LocalArgument",
    "name": "id"
  }
],
v1 = [
  {
    "alias": null,
    "args": [
      {
        "kind": "Variable",
        "name": "id",
        "variableName": "id"
      }
    ],
    "concreteType": "Todo",
    "kind": "LinkedField",
    "name": "deleteTodo",
    "plural": true,
    "selections": [
      {
        "alias": null,
        "args": null,
        "kind": "ScalarField",
        "name": "id",
        "storageKey": null
      }
    ],
    "storageKey": null
  }
];
return {
  "fragment": {
    "argumentDefinitions": (v0/*: any*/),
    "kind": "Fragment",
    "metadata": null,
    "name": "TodoDeleteMutation",
    "selections": (v1/*: any*/),
    "type": "MethodRoot",
    "abstractKey": null
  },
  "kind": "Request",
  "operation": {
    "argumentDefinitions": (v0/*: any*/),
    "kind": "Operation",
    "name": "TodoDeleteMutation",
    "selections": (v1/*: any*/)
  },
  "params": {
    "cacheID": "ab295834469c2bd92c58ef80860083bd",
    "id": null,
    "metadata": {},
    "name": "TodoDeleteMutation",
    "operationKind": "mutation",
    "text": "mutation TodoDeleteMutation(\n  $id: ID!\n) {\n  deleteTodo(id: $id) {\n    id\n  }\n}\n"
  }
};
})();
// prettier-ignore
(node/*: any*/).hash = 'cd5b282bef51c07c308977e5889b00ae';

module.exports = node;
