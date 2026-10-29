### Updating

This fixture reads `who-to-greet`, writes the `time` output, and prints the workflow event payload. It uses the Node standard library, so the sample action has no npm dependencies.

`action.yml` runs `dist/index.js` as an ES module. After editing `index.js`, refresh that file with `npm run build`. `dist/package.json` marks the directory as a module.
