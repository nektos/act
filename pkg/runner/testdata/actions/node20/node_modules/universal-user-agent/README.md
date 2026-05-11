# universal-user-agent

> Get a user agent string across all JavaScript Runtime Environments

[![@latest](https://img.shields.io/npm/v/universal-user-agent.svg)](https://www.npmjs.com/package/universal-user-agent)
[![Build Status](https://github.com/gr2m/universal-user-agent/workflows/Test/badge.svg)](https://github.com/gr2m/universal-user-agent/actions/workflows/test.yml?query=workflow%3ATest)

```js
import { getUserAgent } from "universal-user-agent";

const userAgent = getUserAgent();
// userAgent will look like this
// in browser: "Mozilla/5.0 (Macintosh; Intel Mac OS X 10.13; rv:61.0) Gecko/20100101 Firefox/61.0"
// in node: Node.js/v8.9.4 (macOS High Sierra; x64)
```

## License

[ISC](LICENSE.md)
