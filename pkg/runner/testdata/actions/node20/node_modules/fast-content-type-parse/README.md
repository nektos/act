# fast-content-type-parse

<div align="center">

[![NPM version](https://img.shields.io/npm/v/fast-content-type-parse.svg?style=flat)](https://www.npmjs.com/package/fast-content-type-parse)
[![NPM downloads](https://img.shields.io/npm/dm/fast-content-type-parse.svg?style=flat)](https://www.npmjs.com/package/fast-content-type-parse)
[![CI](https://github.com/fastify/fast-content-type-parse/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/fastify/fast-content-type-parse/actions/workflows/ci.yml)
[![neostandard javascript style](https://img.shields.io/badge/code_style-neostandard-brightgreen?style=flat)](https://github.com/neostandard/neostandard)
[![Security Responsible Disclosure](https://img.shields.io/badge/Security-Responsible%20Disclosure-yellow.svg)](https://github.com/fastify/.github/blob/main/SECURITY.md)

</div>

Parse HTTP Content-Type header according to RFC 7231.

## Installation

```sh
$ npm install fast-content-type-parse
```

## Usage

```js
const fastContentTypeParse = require('fast-content-type-parse')
```

### fastContentTypeParse.parse(string)

```js
const contentType = fastContentTypeParse.parse('application/json; charset=utf-8')
```

Parse a `Content-Type` header. Throws a `TypeError` if the string is invalid.

It will return an object with the following properties (examples are shown for
the string `'application/json; charset=utf-8'`):

 - `type`: The media type (the type and subtype, always lowercase).
   Example: `'application/json'`

 - `parameters`: An object of the parameters in the media type (name of parameter
   always lowercase). Example: `{charset: 'utf-8'}`

### fastContentTypeParse.safeParse(string)

```js
const contentType = fastContentTypeParse.safeParse('application/json; charset=utf-8')
```

Parse a `Content-Type` header. It will not throw an Error if the header is invalid.

This will return an object with the following
properties (examples are shown for the string `'application/json; charset=utf-8'`):

 - `type`: The media type (the type and subtype, always lowercase).
   Example: `'application/json'`

 - `parameters`: An object of the parameters in the media type (name of parameter
   always lowercase). Example: `{charset: 'utf-8'}`

In case the header is invalid, it will return an object
with an empty string `''` as type and an empty Object for `parameters`.

## Benchmarks

```sh
node benchmarks/index.js
util#MIMEType x 1,206,781 ops/sec ±0.22% (96 runs sampled)
fast-content-type-parse#parse x 3,752,236 ops/sec ±0.42% (96 runs sampled)
fast-content-type-parse#safeParse x 3,675,645 ops/sec ±1.09% (94 runs sampled)
content-type#parse x 1,452,582 ops/sec ±0.37% (95 runs sampled)
busboy#parseContentType x 924,306 ops/sec ±0.43% (94 runs sampled)
Fastest is fast-content-type-parse#parse
```

## Credits

Based on the npm package `content-type`.

## License

Licensed under [MIT](./LICENSE).
