const { appendFileSync } = require('fs');
const step = process.env['INPUT_STEP'];
appendFileSync(process.env['GITHUB_ENV'], `TEST=${step}`, { encoding:'utf-8' })

var cache = process.env['INPUT_CACHE']
try {
    var cache = JSON.parse(cache)
} catch {

}
if(typeof cache !== 'boolean') {
    console.log("Input Polluted boolean true/false expected, got " + cache)
    process.exit(1);
}