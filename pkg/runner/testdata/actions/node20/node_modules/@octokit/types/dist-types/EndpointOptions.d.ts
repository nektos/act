import type { RequestMethod } from "./RequestMethod.js";
import type { Url } from "./Url.js";
import type { RequestParameters } from "./RequestParameters.js";
export interface EndpointOptions extends RequestParameters {
    method: RequestMethod;
    url: Url;
}
