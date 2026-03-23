import type { RequestOptions, OctokitResponse } from "@octokit/types";
import type { RequestErrorOptions } from "./types.js";
/**
 * Error with extra properties to help with debugging
 */
export declare class RequestError extends Error {
    name: "HttpError";
    /**
     * http status code
     */
    status: number;
    /**
     * Request options that lead to the error.
     */
    request: RequestOptions;
    /**
     * Response object if a response was received
     */
    response?: OctokitResponse<unknown> | undefined;
    constructor(message: string, statusCode: number, options: RequestErrorOptions);
}
