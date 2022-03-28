module github.com/nektos/act

go 1.16

require (
	github.com/AlecAivazis/survey/v2 v2.3.2
	github.com/Masterminds/semver v1.5.0
	github.com/ProtonMail/go-crypto v0.0.0-20210920160938-87db9fbc61c7 // indirect
	github.com/andreaskoch/go-fswatch v1.0.0
	github.com/docker/cli v20.10.13+incompatible
	github.com/docker/distribution v2.8.1+incompatible
	github.com/docker/docker v20.10.14+incompatible
	github.com/go-git/go-billy/v5 v5.3.1
	github.com/go-git/go-git/v5 v5.4.2
	github.com/go-ini/ini v1.66.4
	github.com/google/shlex v0.0.0-20191202100458-e7afc7fbc510
	github.com/joho/godotenv v1.4.0
	github.com/julienschmidt/httprouter v1.3.0
	github.com/kballard/go-shellquote v0.0.0-20180428030007-95032a82bc51
	github.com/kevinburke/ssh_config v1.1.0 // indirect
	github.com/mattn/go-isatty v0.0.14
	github.com/mgutz/ansi v0.0.0-20200706080929-d51e80ef957d // indirect
	github.com/mitchellh/go-homedir v1.1.0
	github.com/moby/buildkit v0.10.0
	github.com/opencontainers/image-spec v1.0.2
	github.com/opencontainers/selinux v1.10.0
	github.com/pkg/errors v0.9.1
	github.com/rhysd/actionlint v1.6.10
	github.com/sabhiram/go-gitignore v0.0.0-20201211210132-54b8a0bf510f
	github.com/sergi/go-diff v1.2.0 // indirect
	github.com/sirupsen/logrus v1.8.1
	github.com/spf13/cobra v1.4.0
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.7.1
	golang.org/x/term v0.0.0-20210916214954-140adaaadfaf
	gopkg.in/yaml.v3 v3.0.0-20210107192922-496545a6307b
)

replace github.com/go-git/go-git/v5 => github.com/ZauberNerd/go-git/v5 v5.4.3-0.20220315170230-29ec1bc1e5db
