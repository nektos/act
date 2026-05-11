import { DEFAULTS } from "./defaults.js";
import { merge } from "./merge.js";
import { parse } from "./parse.js";
function endpointWithDefaults(defaults, route, options) {
  return parse(merge(defaults, route, options));
}
export {
  endpointWithDefaults
};
