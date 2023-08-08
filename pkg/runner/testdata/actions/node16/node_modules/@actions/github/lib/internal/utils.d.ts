/// <reference types="node" />
import * as http from 'http';
import { OctokitOptions } from '@octokit/core/dist-types/types';
export declare function getAuthString(token: string, options: OctokitOptions): string | undefined;
export declare function getProxyAgent(destinationUrl: string): http.Agent;
export declare function getApiBaseUrl(): string;
