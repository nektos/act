const { appendFileSync } = require('fs');
const step = process.env['INPUT_STEP'];
appendFileSync(process.env['GITHUB_ENV'], `;${step}`, { encoding:'utf-8' })
