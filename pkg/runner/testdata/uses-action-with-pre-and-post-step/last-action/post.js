const pre = process.env['ACTION_OUTPUT_PRE'];
const main = process.env['ACTION_OUTPUT_MAIN'];
const post = process.env['ACTION_OUTPUT_POST'];

console.log({pre, main, post});

if (pre !== 'pre') {
  throw new Error(`Expected 'pre' but got '${pre}'`);
}

if (main !== 'main') {
  throw new Error(`Expected 'main' but got '${main}'`);
}

if (post !== 'post') {
  throw new Error(`Expected 'post' but got '${post}'`);
}
