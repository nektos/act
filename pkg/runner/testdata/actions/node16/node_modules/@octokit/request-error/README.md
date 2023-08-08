# http-error.js

> Error class for Octokit request errors

[![@latest](https://img.shields.io/npm/v/@octokit/request-error.svg)](https://www.npmjs.com/package/@octokit/request-error)
[![Build Status](https://github.com/octokit/request-error.js/workflows/Test/badge.svg)](https://github.com/octokit/request-error.js/actions?query=workflow%3ATest)

## Usage

<table>
<tbody valign=top align=left>
<tr><th>
Browsers
</th><td width=100%>
Load <code>@octokit/request-error</code> directly from <a href="https://cdn.skypack.dev">cdn.skypack.dev</a>
        
```html
<script type="module">
import { RequestError } from "https://cdn.skypack.dev/@octokit/request-error";
</script>
```

</td></tr>
<tr><th>
Node
</th><td>

Install with <code>npm install @octokit/request-error</code>

```js
const { RequestError } = require("@octokit/request-error");
// or: import { RequestError } from "@octokit/request-error";
```

</td></tr>
</tbody>
</table>

```js
const error = new RequestError("Oops", 500, {
  headers: {
    "x-github-request-id": "1:2:3:4",
  }, // response headers
  request: {
    method: "POST",
    url: "https://api.github.com/foo",
    body: {
      bar: "baz",
    },
    headers: {
      authorization: "token secret123",
    },
  },
});

error.message; // Oops
error.status; // 500
error.request.method; // POST
error.request.url; // https://api.github.com/foo
error.request.body; // { bar: 'baz' }
error.request.headers; // { authorization: 'token [REDACTED]' }
error.response; // { url, status, headers, data }
```

## LICENSE

[MIT](LICENSE)
