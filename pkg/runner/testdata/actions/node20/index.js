// Runner fixture. Reads who-to-greet, sets the time output, and prints the
// event payload. Node built-ins only, so this sample stays on the Node
// version in action.yml without third-party packages.
import { randomBytes } from 'node:crypto';
import { appendFileSync, existsSync, readFileSync } from 'node:fs';
import { EOL } from 'node:os';

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
    if (!existsSync(filePath)) {
      throw new Error('Missing file at path: ' + filePath);
    }
    const delimiter = 'ghadelimiter_' + randomBytes(16).toString('hex');
    if (String(name).includes(delimiter) || out.includes(delimiter)) {
      throw new Error('Unexpected input: name or value contains the output delimiter');
    }
    const message = name + '<<' + delimiter + EOL + out + EOL + delimiter;
    appendFileSync(filePath, message + EOL, 'utf8');
    return;
  }
  process.stdout.write(EOL);
  process.stdout.write('::set-output name=' + escapeProperty(name) + '::' + escapeData(out) + EOL);
}

function setFailed(message) {
  process.exitCode = 1;
  process.stdout.write('::error::' + escapeData(message == null ? '' : message) + EOL);
}

function readEventPayload() {
  const eventPath = process.env.GITHUB_EVENT_PATH;
  if (!eventPath) {
    return {};
  }
  if (!existsSync(eventPath)) {
    process.stdout.write('GITHUB_EVENT_PATH ' + eventPath + ' does not exist' + EOL);
    return {};
  }
  return JSON.parse(readFileSync(eventPath, 'utf8'));
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
