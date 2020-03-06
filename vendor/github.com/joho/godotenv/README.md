# GoDotEnv [![Build Status](https://travis-ci.org/joho/godotenv.svg?branch=master)](https://travis-ci.org/joho/godotenv) [![Build status](https://ci.appveyor.com/api/projects/status/9v40vnfvvgde64u4?svg=true)](https://ci.appveyor.com/project/joho/godotenv) [![Go Report Card](https://goreportcard.com/badge/github.com/joho/godotenv)](https://goreportcard.com/report/github.com/joho/godotenv)

A Go (golang) port of the Ruby dotenv project (which loads env vars from a .env file)

From the original Library:

> Storing configuration in the environment is one of the tenets of a twelve-factor app. Anything that is likely to change between deployment environments–such as resource handles for databases or credentials for external services–should be extracted from the code into environment variables.
>
> But it is not always practical to set environment variables on development machines or continuous integration servers where multiple projects are run. Dotenv load variables from a .env file into ENV when the environment is bootstrapped.

It can be used as a library (for loading in env for your own daemons etc) or as a bin command.

There is test coverage and CI for both linuxish and windows environments, but I make no guarantees about the bin version working on windows.

## Installation

As a library

```shell
go get github.com/joho/godotenv
```

or if you want to use it as a bin command
```shell
go get github.com/joho/godotenv/cmd/godotenv
```

## Usage

Add your application configuration to your `.env` file in the root of your project:

```shell
S3_BUCKET=YOURS3BUCKET
SECRET_KEY=YOURSECRETKEYGOESHERE
```

Then in your Go app you can do something like

```go
package main

import (
    "github.com/joho/godotenv"
    "log"
    "os"
)

func main() {
  err := godotenv.Load()
  if err != nil {
    log.Fatal("Error loading .env file")
  }

  s3Bucket := os.Getenv("S3_BUCKET")
  secretKey := os.Getenv("SECRET_KEY")

  // now do something with s3 or whatever
}
```

If you're even lazier than that, you can just take advantage of the autoload package which will read in `.env` on import

```go
import _ "github.com/joho/godotenv/autoload"
```

While `.env` in the project root is the default, you don't have to be constrained, both examples below are 100% legit

```go
_ = godotenv.Load("somerandomfile")
_ = godotenv.Load("filenumberone.env", "filenumbertwo.env")
```

If you want to be really fancy with your env file you can do comments and exports (below is a valid env file)

```shell
# I am a comment and that is OK
SOME_VAR=someval
FOO=BAR # comments at line end are OK too
export BAR=BAZ
```

Or finally you can do YAML(ish) style

```yaml
FOO: bar
BAR: baz
```

as a final aside, if you don't want godotenv munging your env you can just get a map back instead

```go
var myEnv map[string]string
myEnv, err := godotenv.Read()

s3Bucket := myEnv["S3_BUCKET"]
```

... or from an `io.Reader` instead of a local file

```go
reader := getRemoteFile()
myEnv, err := godotenv.Parse(reader)
```

... or from a `string` if you so desire

```go
content := getRemoteFileContent()
myEnv, err := godotenv.Unmarshal(content)
```

### Command Mode

Assuming you've installed the command as above and you've got `$GOPATH/bin` in your `$PATH`

```
godotenv -f /some/path/to/.env some_command with some args
```

If you don't specify `-f` it will fall back on the default of loading `.env` in `PWD`

### Writing Env Files

Godotenv can also write a map representing the environment to a correctly-formatted and escaped file

```go
env, err := godotenv.Unmarshal("KEY=value")
err := godotenv.Write(env, "./.env")
```

... or to a string

```go
env, err := godotenv.Unmarshal("KEY=value")
content, err := godotenv.Marshal(env)
```

## Contributing

Contributions are most welcome! The parser itself is pretty stupidly naive and I wouldn't be surprised if it breaks with edge cases.

*code changes without tests will not be accepted*

1. Fork it
2. Create your feature branch (`git checkout -b my-new-feature`)
3. Commit your changes (`git commit -am 'Added some feature'`)
4. Push to the branch (`git push origin my-new-feature`)
5. Create new Pull Request

## Releases

Releases should follow [Semver](http://semver.org/) though the first couple of releases are `v1` and `v1.1`.

Use [annotated tags for all releases](https://github.com/joho/godotenv/issues/30). Example `git tag -a v1.2.1`

## CI

Linux: [![Build Status](https://travis-ci.org/joho/godotenv.svg?branch=master)](https://travis-ci.org/joho/godotenv) Windows: [![Build status](https://ci.appveyor.com/api/projects/status/9v40vnfvvgde64u4)](https://ci.appveyor.com/project/joho/godotenv)

## Who?

The original library [dotenv](https://github.com/bkeepers/dotenv) was written by [Brandon Keepers](http://opensoul.org/), and this port was done by [John Barton](https://johnbarton.co/) based off the tests/fixtures in the original library.
