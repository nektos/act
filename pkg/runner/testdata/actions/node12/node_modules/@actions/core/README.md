# `@actions/core`

> Core functions for setting results, logging, registering secrets and exporting variables across actions

## Usage

### Import the package

```js
// javascript
const core = require('@actions/core');

// typescript
import * as core from '@actions/core';
```

#### Inputs/Outputs

Action inputs can be read with `getInput` which returns a `string` or `getBooleanInput` which parses a boolean based on the [yaml 1.2 specification](https://yaml.org/spec/1.2/spec.html#id2804923). If `required` set to be false, the input should have a default value in `action.yml`.

Outputs can be set with `setOutput` which makes them available to be mapped into inputs of other actions to ensure they are decoupled.

```js
const myInput = core.getInput('inputName', { required: true });
const myBooleanInput = core.getBooleanInput('booleanInputName', { required: true });
const myMultilineInput = core.getMultilineInput('multilineInputName', { required: true });
core.setOutput('outputKey', 'outputVal');
```

#### Exporting variables

Since each step runs in a separate process, you can use `exportVariable` to add it to this step and future steps environment blocks.

```js
core.exportVariable('envVar', 'Val');
```

#### Setting a secret

Setting a secret registers the secret with the runner to ensure it is masked in logs.

```js
core.setSecret('myPassword');
```

#### PATH Manipulation

To make a tool's path available in the path for the remainder of the job (without altering the machine or containers state), use `addPath`.  The runner will prepend the path given to the jobs PATH.

```js
core.addPath('/path/to/mytool');
```

#### Exit codes

You should use this library to set the failing exit code for your action.  If status is not set and the script runs to completion, that will lead to a success.

```js
const core = require('@actions/core');

try {
  // Do stuff
}
catch (err) {
  // setFailed logs the message and sets a failing exit code
  core.setFailed(`Action failed with error ${err}`);
}
```

Note that `setNeutral` is not yet implemented in actions V2 but equivalent functionality is being planned.

#### Logging

Finally, this library provides some utilities for logging. Note that debug logging is hidden from the logs by default. This behavior can be toggled by enabling the [Step Debug Logs](../../docs/action-debugging.md#step-debug-logs).

```js
const core = require('@actions/core');

const myInput = core.getInput('input');
try {
  core.debug('Inside try block');
  
  if (!myInput) {
    core.warning('myInput was not set');
  }
  
  if (core.isDebug()) {
    // curl -v https://github.com
  } else {
    // curl https://github.com
  }

  // Do stuff
  core.info('Output to the actions build log')

  core.notice('This is a message that will also emit an annotation')
}
catch (err) {
  core.error(`Error ${err}, action may still succeed though`);
}
```

This library can also wrap chunks of output in foldable groups.

```js
const core = require('@actions/core')

// Manually wrap output
core.startGroup('Do some function')
doSomeFunction()
core.endGroup()

// Wrap an asynchronous function call
const result = await core.group('Do something async', async () => {
  const response = await doSomeHTTPRequest()
  return response
})
```

#### Annotations

This library has 3 methods that will produce [annotations](https://docs.github.com/en/rest/reference/checks#create-a-check-run). 
```js
core.error('This is a bad error. This will also fail the build.')

core.warning('Something went wrong, but it\'s not bad enough to fail the build.')

core.notice('Something happened that you might want to know about.')
```

These will surface to the UI in the Actions page and on Pull Requests. They look something like this:

![Annotations Image](../../docs/assets/annotations.png)

These annotations can also be attached to particular lines and columns of your source files to show exactly where a problem is occuring. 

These options are: 
```typescript
export interface AnnotationProperties {
  /**
   * A title for the annotation.
   */
  title?: string

  /**
   * The name of the file for which the annotation should be created.
   */
  file?: string

  /**
   * The start line for the annotation.
   */
  startLine?: number

  /**
   * The end line for the annotation. Defaults to `startLine` when `startLine` is provided.
   */
  endLine?: number

  /**
   * The start column for the annotation. Cannot be sent when `startLine` and `endLine` are different values.
   */
  startColumn?: number

  /**
   * The start column for the annotation. Cannot be sent when `startLine` and `endLine` are different values.
   * Defaults to `startColumn` when `startColumn` is provided.
   */
  endColumn?: number
}
```

#### Styling output

Colored output is supported in the Action logs via standard [ANSI escape codes](https://en.wikipedia.org/wiki/ANSI_escape_code). 3/4 bit, 8 bit and 24 bit colors are all supported.

Foreground colors:

```js
// 3/4 bit
core.info('\u001b[35mThis foreground will be magenta')

// 8 bit
core.info('\u001b[38;5;6mThis foreground will be cyan')

// 24 bit
core.info('\u001b[38;2;255;0;0mThis foreground will be bright red')
```

Background colors:

```js
// 3/4 bit
core.info('\u001b[43mThis background will be yellow');

// 8 bit
core.info('\u001b[48;5;6mThis background will be cyan')

// 24 bit
core.info('\u001b[48;2;255;0;0mThis background will be bright red')
```

Special styles:

```js
core.info('\u001b[1mBold text')
core.info('\u001b[3mItalic text')
core.info('\u001b[4mUnderlined text')
```

ANSI escape codes can be combined with one another:

```js
core.info('\u001b[31;46mRed foreground with a cyan background and \u001b[1mbold text at the end');
```

> Note: Escape codes reset at the start of each line

```js
core.info('\u001b[35mThis foreground will be magenta')
core.info('This foreground will reset to the default')
```

Manually typing escape codes can be a little difficult, but you can use third party modules such as [ansi-styles](https://github.com/chalk/ansi-styles).

```js
const style = require('ansi-styles');
core.info(style.color.ansi16m.hex('#abcdef') + 'Hello world!')
```

#### Action state

You can use this library to save state and get state for sharing information between a given wrapper action:

**action.yml**:

```yaml
name: 'Wrapper action sample'
inputs:
  name:
    default: 'GitHub'
runs:
  using: 'node12'
  main: 'main.js'
  post: 'cleanup.js'
```

In action's `main.js`:

```js
const core = require('@actions/core');

core.saveState("pidToKill", 12345);
```

In action's `cleanup.js`:

```js
const core = require('@actions/core');

var pid = core.getState("pidToKill");

process.kill(pid);
```

#### OIDC Token

You can use these methods to interact with the GitHub OIDC provider and get a JWT ID token which would help to get access token from third party cloud providers.

**Method Name**: getIDToken()

**Inputs**

audience : optional

**Outputs**

A [JWT](https://jwt.io/) ID Token

In action's `main.ts`:
```js
const core = require('@actions/core');
async function getIDTokenAction(): Promise<void> {
  
   const audience = core.getInput('audience', {required: false})
   
   const id_token1 = await core.getIDToken()            // ID Token with default audience
   const id_token2 = await core.getIDToken(audience)    // ID token with custom audience
   
   // this id_token can be used to get access token from third party cloud providers
}
getIDTokenAction()
```

In action's `actions.yml`:

```yaml
name: 'GetIDToken'
description: 'Get ID token from Github OIDC provider'
inputs:
  audience:  
    description: 'Audience for which the ID token is intended for'
    required: false
outputs:
  id_token1: 
    description: 'ID token obtained from OIDC provider'
  id_token2: 
    description: 'ID token obtained from OIDC provider'
runs:
  using: 'node12'
  main: 'dist/index.js'
```

#### Filesystem path helpers

You can use these methods to manipulate file paths across operating systems.

The `toPosixPath` function converts input paths to Posix-style (Linux) paths.
The `toWin32Path` function converts input paths to Windows-style paths. These
functions work independently of the underlying runner operating system.

```js
toPosixPath('\\foo\\bar') // => /foo/bar
toWin32Path('/foo/bar') // => \foo\bar
```

The `toPlatformPath` function converts input paths to the expected value on the runner's operating system.

```js
// On a Windows runner.
toPlatformPath('/foo/bar') // => \foo\bar

// On a Linux runner.
toPlatformPath('\\foo\\bar') // => /foo/bar
```
