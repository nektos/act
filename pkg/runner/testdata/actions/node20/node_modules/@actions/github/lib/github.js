import * as Context from './context.js';
import { GitHub, getOctokitOptions } from './utils.js';
export const context = new Context.Context();
/**
 * Returns a hydrated octokit ready to use for GitHub Actions
 *
 * @param     token    the repo PAT or GITHUB_TOKEN
 * @param     options  other options to set
 */
export function getOctokit(token, options, ...additionalPlugins) {
    const GitHubWithPlugins = GitHub.plugin(...additionalPlugins);
    return new GitHubWithPlugins(getOctokitOptions(token, options));
}
//# sourceMappingURL=github.js.map