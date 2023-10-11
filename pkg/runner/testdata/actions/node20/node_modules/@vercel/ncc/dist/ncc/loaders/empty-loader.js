// returns the base-level package folder based on detecting "node_modules"
// package name boundaries
const pkgNameRegEx = /^(@[^\\\/]+[\\\/])?[^\\\/]+/;
function getPackageBase(id) {
  const pkgIndex = id.lastIndexOf('node_modules');
  if (pkgIndex !== -1 &&
      (id[pkgIndex - 1] === '/' || id[pkgIndex - 1] === '\\') &&
      (id[pkgIndex + 12] === '/' || id[pkgIndex + 12] === '\\')) {
    const pkgNameMatch = id.substr(pkgIndex + 13).match(pkgNameRegEx);
    if (pkgNameMatch)
      return id.substr(0, pkgIndex + 13 + pkgNameMatch[0].length);
  }
}

const emptyModules = { 'uglify-js': true, 'uglify-es': true };

module.exports = function (input, map) {
  const id = this.resourcePath;
  const pkgBase = getPackageBase(id);
  if (pkgBase) {
    const baseParts = pkgBase.split('/');
    if (baseParts[baseParts.length - 2] === 'node_modules') {
      const pkgName = baseParts[baseParts.length - 1];
      if (pkgName in emptyModules) {
        console.warn(`ncc: Ignoring build of ${pkgName}, as it is not statically analyzable. Build with "--external ${pkgName}" if this package is needed.`);
        return '';
      }
    }
  }
  this.callback(null, input, map);
};
