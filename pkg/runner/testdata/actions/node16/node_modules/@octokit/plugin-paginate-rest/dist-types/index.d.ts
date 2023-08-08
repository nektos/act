import { Octokit } from "@octokit/core";
import { PaginateInterface } from "./types";
export { PaginateInterface } from "./types";
export { PaginatingEndpoints } from "./types";
export { composePaginateRest } from "./compose-paginate";
export { isPaginatingEndpoint, paginatingEndpoints, } from "./paginating-endpoints";
/**
 * @param octokit Octokit instance
 * @param options Options passed to Octokit constructor
 */
export declare function paginateRest(octokit: Octokit): {
    paginate: PaginateInterface;
};
export declare namespace paginateRest {
    var VERSION: string;
}
