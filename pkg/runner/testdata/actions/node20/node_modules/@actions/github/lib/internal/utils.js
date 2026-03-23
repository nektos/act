var __awaiter = (this && this.__awaiter) || function (thisArg, _arguments, P, generator) {
    function adopt(value) { return value instanceof P ? value : new P(function (resolve) { resolve(value); }); }
    return new (P || (P = Promise))(function (resolve, reject) {
        function fulfilled(value) { try { step(generator.next(value)); } catch (e) { reject(e); } }
        function rejected(value) { try { step(generator["throw"](value)); } catch (e) { reject(e); } }
        function step(result) { result.done ? resolve(result.value) : adopt(result.value).then(fulfilled, rejected); }
        step((generator = generator.apply(thisArg, _arguments || [])).next());
    });
};
import * as httpClient from '@actions/http-client';
import { fetch } from 'undici';
export function getAuthString(token, options) {
    if (!token && !options.auth) {
        throw new Error('Parameter token or opts.auth is required');
    }
    else if (token && options.auth) {
        throw new Error('Parameters token and opts.auth may not both be specified');
    }
    return typeof options.auth === 'string' ? options.auth : `token ${token}`;
}
export function getProxyAgent(destinationUrl) {
    const hc = new httpClient.HttpClient();
    return hc.getAgent(destinationUrl);
}
export function getProxyAgentDispatcher(destinationUrl) {
    const hc = new httpClient.HttpClient();
    return hc.getAgentDispatcher(destinationUrl);
}
export function getProxyFetch(destinationUrl) {
    const httpDispatcher = getProxyAgentDispatcher(destinationUrl);
    const proxyFetch = (url, opts) => __awaiter(this, void 0, void 0, function* () {
        return fetch(url, Object.assign(Object.assign({}, opts), { dispatcher: httpDispatcher }));
    });
    return proxyFetch;
}
export function getApiBaseUrl() {
    return process.env['GITHUB_API_URL'] || 'https://api.github.com';
}
//# sourceMappingURL=utils.js.map