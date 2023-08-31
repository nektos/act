import { RequestMethod } from "./RequestMethod";
import { Url } from "./Url";
import { RequestParameters } from "./RequestParameters";
export declare type EndpointOptions = RequestParameters & {
    method: RequestMethod;
    url: Url;
};
