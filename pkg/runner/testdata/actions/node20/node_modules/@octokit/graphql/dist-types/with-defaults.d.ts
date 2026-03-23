import { request as Request } from "@octokit/request";
import type { graphql as ApiInterface, RequestParameters } from "./types.js";
export declare function withDefaults(request: typeof Request, newDefaults: RequestParameters): ApiInterface;
