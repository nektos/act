const { appendFileSync } = require('fs');
const env = process.env['STEP_OUTPUT_TEST'];
const step = process.env['INPUT_STEP'];
appendFileSync(process.env['GITHUB_ENV'], `;${step}`, { encoding:'utf-8' })
