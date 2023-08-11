import { EndpointInterface } from "@octokit/types";
export default function fetchWrapper(requestOptions: ReturnType<EndpointInterface> & {
    redirect?: "error" | "follow" | "manual";
}): Promise<{
    status: number;
    url: string;
    headers: {
        [header: string]: string;
    };
    data: any;
}>;
