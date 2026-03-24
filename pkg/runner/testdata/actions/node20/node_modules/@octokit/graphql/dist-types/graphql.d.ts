import { request as Request } from "@octokit/request";
import type { RequestParameters, GraphQlQueryResponseData } from "./types.js";
export declare function graphql<ResponseData = GraphQlQueryResponseData>(request: typeof Request, query: string | RequestParameters, options?: RequestParameters): Promise<ResponseData>;
