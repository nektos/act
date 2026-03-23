import type { EndpointInterface, OctokitResponse } from "@octokit/types";
export default function fetchWrapper(requestOptions: ReturnType<EndpointInterface>): Promise<OctokitResponse<any>>;
