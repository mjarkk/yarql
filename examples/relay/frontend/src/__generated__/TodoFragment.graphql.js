/**
 * @flow
 */

/* eslint-disable */

'use strict';

/*::
import type { ReaderFragment } from 'relay-runtime';
import type { FragmentReference } from "relay-runtime";
declare export opaque type TodoFragment$ref: FragmentReference;
declare export opaque type TodoFragment$fragmentType: TodoFragment$ref;
export type TodoFragment = {|
  +id: string,
  +title: string,
  +done: boolean,
  +$refType: TodoFragment$ref,
|};
export type TodoFragment$data = TodoFragment;
export type TodoFragment$key = {
  +$data?: TodoFragment$data,
  +$fragmentRefs: TodoFragment$ref,
  ...
};
*/


const node/*: ReaderFragment*/ = {
  "argumentDefinitions": [],
  "kind": "Fragment",
  "metadata": null,
  "name": "TodoFragment",
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
  "type": "Todo",
  "abstractKey": null
};
// prettier-ignore
(node/*: any*/).hash = 'b80f9cfa2c9113fc4e0fd3802c259047';

module.exports = node;
