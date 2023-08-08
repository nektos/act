import { Octokit } from "@octokit/core";
import { EndpointsDefaultsAndDecorations } from "./types";
import { RestEndpointMethods } from "./generated/method-types";
export declare function endpointsToMethods(octokit: Octokit, endpointsMap: EndpointsDefaultsAndDecorations): RestEndpointMethods;
