console.log('pre');

var cache = process.env['INPUT_CACHE']
try {
    var cache = JSON.parse(cache)
} catch {

}
if(typeof cache !== 'boolean') {
    console.log("Input Polluted boolean true/false expected, got " + cache)
    process.exit(1);
}