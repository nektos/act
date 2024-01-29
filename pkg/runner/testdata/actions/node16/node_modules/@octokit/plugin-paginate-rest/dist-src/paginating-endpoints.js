import { paginatingEndpoints, } from "./generated/paginating-endpoints";
export { paginatingEndpoints } from "./generated/paginating-endpoints";
export function isPaginatingEndpoint(arg) {
    if (typeof arg === "string") {
        return paginatingEndpoints.includes(arg);
    }
    else {
        return false;
    }
}
