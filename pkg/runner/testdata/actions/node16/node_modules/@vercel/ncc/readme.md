# ncc

[![CI Status](https://github.com/vercel/ncc/workflows/CI/badge.svg)](https://github.com/vercel/ncc/actions?workflow=CI)

Simple CLI for compiling a Node.js module into a single file,
together with all its dependencies, gcc-style.

## Motivation

- Publish minimal packages to npm
- Only ship relevant app code to serverless environments
- Don't waste time configuring bundlers
- Generally faster bootup time and less I/O overhead
- Compiled language-like experience (e.g.: `go`)

## Design goals

- Zero configuration
- TypeScript built-in
- Only supports Node.js programs as input / output
- Support all Node.js patterns and npm modules

## Usage

### Installation
```bash
npm i -g @vercel/ncc
```

### Usage

```bash
$ ncc <cmd> <opts>
```
Eg:
```bash
$ ncc build input.js -o dist
```

If building an `.mjs` or `.js` module inside a `"type": "module"` [package boundary](https://nodejs.org/dist/latest-v16.x/docs/api/packages.html#packages_package_json_and_file_extensions), an ES module output will be created automatically.

Outputs the Node.js compact build of `input.js` into `dist/index.js`.

> Note: If the input file is using a `.cjs` extension, then so will the corresponding output file.
> This is useful for packages that want to use `.js` files as modules in native Node.js using
> a `"type": "module"` in the package.json file.

#### Commands:
```
  build <input-file> [opts]
  run <input-file> [opts]
  cache clean|dir|size
  help
  version
```

#### Options:
```
  -o, --out [dir]          Output directory for build (defaults to dist)
  -m, --minify             Minify output
  -C, --no-cache           Skip build cache population
  -s, --source-map         Generate source map
  -a, --asset-builds       Build nested JS assets recursively, useful for
                           when code is loaded as an asset eg for workers.
  --no-source-map-register Skip source-map-register source map support
  -e, --external [mod]     Skip bundling 'mod'. Can be used many times
  -q, --quiet              Disable build summaries / non-error outputs
  -w, --watch              Start a watched build
  -t, --transpile-only     Use transpileOnly option with the ts-loader
  --v8-cache               Emit a build using the v8 compile cache
  --license [file]         Adds a file containing licensing information to the output
  --stats-out [file]       Emit webpack stats as json to the specified output file
  --target [es]            ECMAScript target to use for output (default: es2015)
                           Learn more: https://webpack.js.org/configuration/target
  -d, --debug              Show debug logs
```

### Execution Testing

For testing and debugging, a file can be built into a temporary directory and executed with full source maps support with the command:

```bash
$ ncc run input.js
```

### With TypeScript

The only requirement is to point `ncc` to `.ts` or `.tsx` files. A `tsconfig.json`
file is necessary. Most likely you want to indicate `es2015` support:

```json
{
  "compilerOptions": {
    "target": "es2015",
    "moduleResolution": "node"
  }
}
```

If typescript is found in `devDependencies`, that version will be used.

### Package Support

Some packages may need some extra options for ncc support in order to better work with the static analysis.

See [package-support.md](package-support.md) for some common packages and their usage with ncc.

### Programmatically From Node.js

```js
require('@vercel/ncc')('/path/to/input', {
  // provide a custom cache path or disable caching
  cache: "./custom/cache/path" | false,
  // externals to leave as requires of the build
  externals: ["externalpackage"],
  // directory outside of which never to emit assets
  filterAssetBase: process.cwd(), // default
  minify: false, // default
  sourceMap: false, // default
  assetBuilds: false, // default
  sourceMapBasePrefix: '../', // default treats sources as output-relative
  // when outputting a sourcemap, automatically include
  // source-map-support in the output file (increases output by 32kB).
  sourceMapRegister: true, // default
  watch: false, // default
  license: '', // default does not generate a license file
  target: 'es2015', // default
  v8cache: false, // default
  quiet: false, // default
  debugLog: false // default
}).then(({ code, map, assets }) => {
  console.log(code);
  // Assets is an object of asset file names to { source, permissions, symlinks }
  // expected relative to the output code (if any)
})
```

When `watch: true` is set, the build object is not a promise, but has the following signature:

```js
{
  // handler re-run on each build completion
  // watch errors are reported on "err"
  handler (({ err, code, map, assets }) => { ... })
  // handler re-run on each rebuild start
  rebuild (() => {})
  // close the watcher
  void close ();
}
```

## Caveats

- Files / assets are relocated based on a [static evaluator](https://github.com/vercel/webpack-asset-relocator-loader#how-it-works). Dynamic non-statically analyzable asset loads may not work out correctly
