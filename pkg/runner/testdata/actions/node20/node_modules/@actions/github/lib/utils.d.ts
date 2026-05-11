import * as Context from './context.js';
import type { OctokitOptions } from '@octokit/core/types';
import { Octokit } from '@octokit/core';
export declare const context: Context.Context;
export declare const defaults: OctokitOptions;
export declare const GitHub: typeof Octokit & import("@octokit/core/types").Constructor<import("@octokit/plugin-rest-endpoint-methods").Api & {
    paginate: import("@octokit/plugin-paginate-rest").PaginateInterface;
}>;
/**
 * Convience function to correctly format Octokit Options to pass into the constructor.
 *
 * @param     token    the repo PAT or GITHUB_TOKEN
 * @param     options  other options to set
 */
export declare function getOctokitOptions(token: string, options?: OctokitOptions): OctokitOptions;
