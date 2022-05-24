const output = process.env['STEP_OUTPUT_TEST'];
const expected = 'empty;step1;step2;step2-post;step1-post';

console.log(output);
if (output !== expected) {
  throw new Error(`Expected '${expected}' but got '${output}'`);
}
