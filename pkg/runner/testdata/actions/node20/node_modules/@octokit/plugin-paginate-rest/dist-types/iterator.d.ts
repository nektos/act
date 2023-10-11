import { Octokit } from "@octokit/core";
import { RequestInterface, RequestParameters, Route } from "./types";
export declare function iterator(octokit: Octokit, route: Route | RequestInterface, parameters?: RequestParameters): {
    [Symbol.asyncIterator]: () => {
        next(): Promise<{
            done: boolean;
            value?: undefined;
        } | {
            value: import("@octokit/types/dist-types/OctokitResponse").OctokitResponse<any, number>;
            done?: undefined;
        } | {
            value: {
                status: number;
                headers: {};
                data: never[];
            };
            done?: undefined;
        }>;
    };
};
