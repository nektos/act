import * as Context from './context.js';
import { GitHub } from './utils.js';
import type { OctokitOptions, OctokitPlugin } from '@octokit/core/types';
export declare const context: Context.Context;
/**
 * Returns a hydrated octokit ready to use for GitHub Actions
 *
 * @param     token    the repo PAT or GITHUB_TOKEN
 * @param     options  other options to set
 */
export declare function getOctokit(token: string, options?: OctokitOptions, ...additionalPlugins: OctokitPlugin[]): InstanceType<typeof GitHub>;
