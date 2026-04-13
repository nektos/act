import { endpointWithDefaults } from "./endpoint-with-defaults.js";
import { merge } from "./merge.js";
import { parse } from "./parse.js";
function withDefaults(oldDefaults, newDefaults) {
  const DEFAULTS = merge(oldDefaults, newDefaults);
  const endpoint = endpointWithDefaults.bind(null, DEFAULTS);
  return Object.assign(endpoint, {
    DEFAULTS,
    defaults: withDefaults.bind(null, DEFAULTS),
    merge: merge.bind(null, DEFAULTS),
    parse
  });
}
export {
  withDefaults
};
