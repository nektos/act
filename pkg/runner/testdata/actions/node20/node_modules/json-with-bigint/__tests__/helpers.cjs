const originalParse = JSON.parse;

/*
  Function to test the V1 (implementation without the JSON.parse's context.source feature support)
*/
const imitateJSONParseWithoutContext = (text, reviver) => {
  return originalParse(text, (key, value) => reviver(key, value));
};

module.exports = { originalParse, imitateJSONParseWithoutContext };
module.exports.default = { originalParse, imitateJSONParseWithoutContext };
