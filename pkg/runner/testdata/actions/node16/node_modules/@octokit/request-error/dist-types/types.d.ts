import { RequestOptions, ResponseHeaders, OctokitResponse } from "@octokit/types";
export declare type RequestErrorOptions = {
    /** @deprecated set `response` instead */
    headers?: ResponseHeaders;
    request: RequestOptions;
} | {
    response: OctokitResponse<unknown>;
    request: RequestOptions;
};
