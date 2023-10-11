module.exports = function (input, map) {
  if (this.cacheable)
    this.cacheable();
  const id = this.resourceQuery.substr(1);
  input = input.replace('\'UNKNOWN\'', JSON.stringify(id));
  this.callback(null, input, map);
};
