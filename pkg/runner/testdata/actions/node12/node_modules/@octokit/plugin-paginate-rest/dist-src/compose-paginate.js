import { paginate } from "./paginate";
import { iterator } from "./iterator";
export const composePaginateRest = Object.assign(paginate, {
    iterator,
});
