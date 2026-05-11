import { expectType } from "tsd";

import { getUserAgent } from "./index.js";

expectType<string>(getUserAgent());
