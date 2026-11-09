'use strict';

// Runner fixture. Reads who-to-greet, sets the time output, and prints the
// event payload. Node built-ins only, so this sample stays on the Node
// version in action.yml without third-party packages.
const crypto = require('crypto');
const fs = require('fs');
const os = require('os');

function getInput(name) {
  const key = 'INPUT_' + String(name).replace(/ /g, '_').toUpperCase();
  return (process.env[key] || '').trim();
}

function escapeData(value) {
  return String(value).replace(/%/g, '%25').replace(/\r/g, '%0D').replace(/\n/g, '%0A');
}

function escapeProperty(value) {
  return escapeData(value).replace(/:/g, '%3A').replace(/,/g, '%2C');
}

function setOutput(name, value) {
  const out = value == null ? '' : String(value);
  const filePath = process.env.GITHUB_OUTPUT;
  if (filePath) {
    if (!fs.existsSync(filePath)) {
      throw new Error('Missing file at path: ' + filePath);
    }
    const delimiter = 'ghadelimiter_' + crypto.randomBytes(16).toString('hex');
    if (String(name).includes(delimiter) || out.includes(delimiter)) {
      throw new Error('Unexpected input: name or value contains the output delimiter');
    }
    const message = name + '<<' + delimiter + os.EOL + out + os.EOL + delimiter;
    fs.appendFileSync(filePath, message + os.EOL, 'utf8');
    return;
  }
  process.stdout.write(os.EOL);
  process.stdout.write('::set-output name=' + escapeProperty(name) + '::' + escapeData(out) + os.EOL);
}

function setFailed(message) {
  process.exitCode = 1;
  process.stdout.write('::error::' + escapeData(message == null ? '' : message) + os.EOL);
}

function readEventPayload() {
  const eventPath = process.env.GITHUB_EVENT_PATH;
  if (!eventPath) {
    return {};
  }
  if (!fs.existsSync(eventPath)) {
    process.stdout.write('GITHUB_EVENT_PATH ' + eventPath + ' does not exist' + os.EOL);
    return {};
  }
  return JSON.parse(fs.readFileSync(eventPath, 'utf8'));
}

try {
  const nameToGreet = getInput('who-to-greet');
  console.log(`Hello ${nameToGreet}!`);
  const time = (new Date()).toTimeString();
  setOutput('time', time);
  const payload = JSON.stringify(readEventPayload(), undefined, 2);
  console.log(`The event payload: ${payload}`);
} catch (error) {
  setFailed(error && error.message);
}
