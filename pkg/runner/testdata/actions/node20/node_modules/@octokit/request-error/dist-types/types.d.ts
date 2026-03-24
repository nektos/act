import type { RequestOptions, OctokitResponse } from "@octokit/types";
export interface RequestErrorOptions extends ErrorOptions {
    response?: OctokitResponse<unknown> | undefined;
    request: RequestOptions;
}
