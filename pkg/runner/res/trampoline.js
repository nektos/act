const { spawnSync } = require('child_process')
const spawnArguments = {
  cwd: process.env.INPUT_CWD,
  stdio: [
    process.stdin,
    process.stdout,
    process.stderr
  ]
}
const child = spawnSync(
  '/bin/sh',
  ['-c'].concat(process.env.INPUT_COMMAND),
  spawnArguments)
process.exit(child.status)
