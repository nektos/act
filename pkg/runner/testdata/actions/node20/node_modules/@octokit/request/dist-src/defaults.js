import { getUserAgent } from "universal-user-agent";
import { VERSION } from "./version.js";
var defaults_default = {
  headers: {
    "user-agent": `octokit-request.js/${VERSION} ${getUserAgent()}`
  }
};
export {
  defaults_default as default
};
