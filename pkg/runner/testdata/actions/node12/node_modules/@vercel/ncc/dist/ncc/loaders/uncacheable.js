module.exports = function (input, map) {
  this.cacheable(false);
  return this.callback(null, input, map);
};