import { request } from "@octokit/request";
export declare const graphql: import("./types.js").graphql;
export type { GraphQlQueryResponseData } from "./types.js";
export { GraphqlResponseError } from "./error.js";
export declare function withCustomRequest(customRequest: typeof request): import("./types.js").graphql;
