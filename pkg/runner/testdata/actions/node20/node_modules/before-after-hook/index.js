// @ts-check

import { register } from "./lib/register.js";
import { addHook } from "./lib/add.js";
import { removeHook } from "./lib/remove.js";

// bind with array of arguments: https://stackoverflow.com/a/21792913
const bind = Function.bind;
const bindable = bind.bind(bind);

function bindApi(hook, state, name) {
  const removeHookRef = bindable(removeHook, null).apply(
    null,
    name ? [state, name] : [state]
  );
  hook.api = { remove: removeHookRef };
  hook.remove = removeHookRef;
  ["before", "error", "after", "wrap"].forEach((kind) => {
    const args = name ? [state, kind, name] : [state, kind];
    hook[kind] = hook.api[kind] = bindable(addHook, null).apply(null, args);
  });
}

function Singular() {
  const singularHookName = Symbol("Singular");
  const singularHookState = {
    registry: {},
  };
  const singularHook = register.bind(null, singularHookState, singularHookName);
  bindApi(singularHook, singularHookState, singularHookName);
  return singularHook;
}

function Collection() {
  const state = {
    registry: {},
  };

  const hook = register.bind(null, state);
  bindApi(hook, state);

  return hook;
}

export default { Singular, Collection };
