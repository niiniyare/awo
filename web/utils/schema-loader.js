/**
 * SchemaLoader - Loads AMIS JSON schemas via fetch
 * Usage: var loader = new SchemaLoader('../schemas/');
 *        loader.load('pages/dashboard.json').then(schema => ...)
 */
function SchemaLoader(basePath) {
  this.basePath = basePath || '';
  this.cache = {};
}

SchemaLoader.prototype.load = function(path) {
  var url = this.basePath + path;
  if (this.cache[url]) {
    return Promise.resolve(JSON.parse(JSON.stringify(this.cache[url])));
  }
  return fetch(url)
    .then(function(res) {
      if (!res.ok) throw new Error('Failed to load schema: ' + path);
      return res.json();
    })
    .then(function(schema) {
      this.cache[url] = schema;
      return JSON.parse(JSON.stringify(schema));
    }.bind(this));
};
