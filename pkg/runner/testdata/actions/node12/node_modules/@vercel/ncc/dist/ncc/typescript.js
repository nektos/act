
const { Module } = require('module');
const m = new Module('', null);
m.paths = Module._nodeModulePaths(process.env.TYPESCRIPT_LOOKUP_PATH || (process.cwd() + '/'));
let typescript;
try {
  typescript = m.require('typescript');
  console.log("ncc: Using typescript@" + typescript.version + " (local user-provided)");
}
catch (e) {
  typescript = require('./loaders/ts-loader.js').typescript;
  console.log("ncc: Using typescript@" + typescript.version + " (ncc built-in)");
}
module.exports = typescript;
