# JSON with BigInt

JS library that allows you to easily serialize and deserialize data with BigInt values

## Why would I need json-with-bigint?

3 reasons:

1. You need to convert some data to/from JSON and it includes BigInt values
2. Native JSON.stringify() and JSON.parse() methods in JS can't work with BigInt
3. Other libraries and pieces of code that you'll find either can't solve this problem while supporting consistent round-trip operations (meaning, you will not get the same BigInt values if you serialize and then deserialize them) or requires you to specify which properties in JSON include BigInt values, or to change your JSON or the way you want to work with your data

## json-with-bigint advantages

✔️ Supports consistent round-trip operations with JSON

```
const data = { bigNumber: 9007199254740992n };
JSONStringify(data) // '{"bigNumber":9007199254740992}'
JSONParse(JSONStringify(data)).bigNumber === 9007199254740992n // true
```

✔️ No need to specify which properties in JSON include BigInt values. Library will find them itself

✔️ No need to change your JSON or the way you want to work with your data

✔️ You don't have to memorize this library's API, you already know it. Just skip the dot, and that's it (`JSONParse()`, `JSONStringify()`)

✔️ Parsed big number values are just regular BigInt. Parses and stringifies all other values other than big numbers the same way as native JSON methods in JS do. Signatures match too. You can just replace every `JSON.parse()` and `JSON.strinfigy()` in your project with `JSONParse()` and `JSONStringify()`, and it will work

✔️ Supports modern features (context.source, rawJSON, etc.)

✔️ Correctly parses float numbers and negative numbers

✔️ Correctly parses pretty printed JSON (formatted with newline and whitespace characters)

✔️ Does not contaminate your global space (unlike monkey-patching solution)

✔️ Isomorphic (it can run in both the browser and Node.js with the same code)

✔️ Can be used in both JavaScript and TypeScript projects (.d.ts file included)

✔️ Can be used as both ESM and CommonJS module

✔️ No transpilers needed. Runs even in ES5 environments

✔️ Actively supported

✔️ Size: 1180 bytes (minified and gzipped)

✔️ No dependencies. Even the dev ones

✔️ Extensively covered by tests

## Getting Started

This library has no default export. [Why it's a good thing](https://humanwhocodes.com/blog/2019/01/stop-using-default-exports-javascript-module/)

### NPM

Add this library to your project using NPM

```
npm i json-with-bigint
```

and use it

```
import { JSONParse, JSONStringify } from 'json-with-bigint';

const userData = {
  someBigNumber: 9007199254740992n
};

localStorage.setItem('userData', JSONStringify(userData));

const restoredUserData = JSONParse(localStorage.getItem('userData') || '');
```

### CDN

Add this code to your HTML

```
<script src="https://cdn.jsdelivr.net/npm/json-with-bigint/json-with-bigint.min.js"></script>
```

and use it

```
<script>
  const userData = {
    someBigNumber: 9007199254740992n
  };

  localStorage.setItem('userData', JSONStringify(userData));

  const restoredUserData = JSONParse(localStorage.getItem('userData') || '');
</script>
```

### Manually

Download json-with-bigint.min.js from this repository to your project's folder and use it

```
<script src="./json-with-bigint.min.js"></script>
<script>
  const userData = {
    someBigNumber: 9007199254740992n
  };

  localStorage.setItem('userData', JSONStringify(userData));

  const restoredUserData = JSONParse(localStorage.getItem('userData') || '');
</script>
```

## How to use

`JSONParse` - works just like [native JSON.parse](https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/JSON/parse), but supports BigInt

`JSONStringify` - works just like [native JSON.stringify](https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Global_Objects/JSON/stringify), but supports BigInt

Examples:

- `JSONParse('{"someBigNumber":9007199254740992}')`
- `JSONStringify({
someBigNumber: 9007199254740992n
})`
