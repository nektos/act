
const { Module } = require('module');
const m = new Module('', null);
const { quiet, typescriptLookupPath } = JSON.parse(process.env.__NCC_OPTS || '{}');
m.paths = Module._nodeModulePaths(process.env.TYPESCRIPT_LOOKUP_PATH || typescriptLookupPath || (process.cwd() + '/'));
let typescript;
try {
  typescript = m.require('typescript');
  if (!quiet) console.log("ncc: Using typescript@" + typescript.version + " (local user-provided)");
}
catch (e) {
  typescript = require('typescript');
  if (!quiet) console.log("ncc: Using typescript@" + typescript.version + " (ncc built-in)");
}
module.exports = typescript;
