# List of Docker images for `act`

**Warning:** Below badges with size for each image are displaying size of **compressed image size in registry. After pulling the image, size can be drastically different due to Docker uncompressing the image layers.**

## Images based on [`buildpack-deps`][hub/_/buildpack-deps]

**Note 1: `node` images are based on Debian root filesystem, while it is extremely similar to Ubuntu, there might be some differences**

**Note 2: `node` `-slim` images don't have `python` installed, if you want to use actions or software that is depending on `python`, you need to specify image manually**

| Image                                | Size                                                     |
| ------------------------------------ | -------------------------------------------------------- |
| [`node:12-buster`][hub/_/node]       | ![`buster-size`][hub/_/node/12-buster/size]              |
| [`node:12-buster-slim`][hub/_/node]  | ![`micro-buster-size`][hub/_/node/12-buster-slim/size]   |
| [`node:12-stretch`][hub/_/node]      | ![`stretch-size`][hub/_/node/12-stretch/size]            |
| [`node:12-stretch-slim`][hub/_/node] | ![`micro-stretch-size`][hub/_/node/12-stretch-slim/size] |

**Note: `catthehacker/ubuntu` images are based on Ubuntu root filesystem**

| Image                                                                | GitHub Repository                                             |
| -------------------------------------------------------------------- | ------------------------------------------------------------- |
| [`ghcr.io/catthehacker/ubuntu:act-latest`][ghcr/catthehacker/ubuntu] | [`catthehacker/docker-images`][gh/catthehacker/docker_images] |
| [`ghcr.io/catthehacker/ubuntu:act-20.04`][ghcr/catthehacker/ubuntu]  | [`catthehacker/docker-images`][gh/catthehacker/docker_images] |
| [`ghcr.io/catthehacker/ubuntu:act-18.04`][ghcr/catthehacker/ubuntu]  | [`catthehacker/docker-images`][gh/catthehacker/docker_images] |
| [`ghcr.io/catthehacker/ubuntu:act-16.04`][ghcr/catthehacker/ubuntu]  | [`catthehacker/docker-images`][gh/catthehacker/docker_images] |

## Images based on [`actions/virtual-environments`][gh/actions/virtual-environments]

**Note: `nektos/act-environments-ubuntu` have been last updated in February, 2020. It's recommended to update the image manually after `docker pull` if you decide to to use it.**

| Image                                                                             | Size                                                                       | GitHub Repository                                       |
| --------------------------------------------------------------------------------- | -------------------------------------------------------------------------- | ------------------------------------------------------- |
| [`nektos/act-environments-ubuntu:18.04`][hub/nektos/act-environments-ubuntu]      | ![`nektos:18.04`][hub/nektos/act-environments-ubuntu/18.04/size]           | [`nektos/act-environments`][gh/nektos/act-environments] |
| [`nektos/act-environments-ubuntu:18.04-lite`][hub/nektos/act-environments-ubuntu] | ![`nektos:18.04-lite`][hub/nektos/act-environments-ubuntu/18.04-lite/size] | [`nektos/act-environments`][gh/nektos/act-environments] |
| [`nektos/act-environments-ubuntu:18.04-full`][hub/nektos/act-environments-ubuntu] | ![`nektos:18.04-full`][hub/nektos/act-environments-ubuntu/18.04-full/size] | [`nektos/act-environments`][gh/nektos/act-environments] |

| Image                                                                 | GitHub Repository                                                                     |
| --------------------------------------------------------------------- | ------------------------------------------------------------------------------------- |
| [`ghcr.io/catthehacker/ubuntu:full-latest`][ghcr/catthehacker/ubuntu] | [`catthehacker/virtual-environments-fork`][gh/catthehacker/virtual-environments-fork] |
| [`ghcr.io/catthehacker/ubuntu:full-20.04`][ghcr/catthehacker/ubuntu]  | [`catthehacker/virtual-environments-fork`][gh/catthehacker/virtual-environments-fork] |
| [`ghcr.io/catthehacker/ubuntu:full-18.04`][ghcr/catthehacker/ubuntu]  | [`catthehacker/virtual-environments-fork`][gh/catthehacker/virtual-environments-fork] |

Feel free to make a pull request with your image added here

[hub/_/buildpack-deps]: https://hub.docker.com/_/buildpack-deps
[hub/_/node]: https://hub.docker.com/r/_/node
[hub/_/node/12-buster/size]: https://img.shields.io/docker/image-size/_/node/12-buster
[hub/_/node/12-buster-slim/size]: https://img.shields.io/docker/image-size/_/node/12-buster-slim
[hub/_/node/12-stretch/size]: https://img.shields.io/docker/image-size/_/node/12-stretch
[hub/_/node/12-stretch-slim/size]: https://img.shields.io/docker/image-size/_/node/12-stretch-slim
[ghcr/catthehacker/ubuntu]: https://github.com/catthehacker/docker_images/pkgs/container/ubuntu
[hub/nektos/act-environments-ubuntu]: https://hub.docker.com/r/nektos/act-environments-ubuntu
[hub/nektos/act-environments-ubuntu/18.04/size]: https://img.shields.io/docker/image-size/nektos/act-environments-ubuntu/18.04
[hub/nektos/act-environments-ubuntu/18.04-lite/size]: https://img.shields.io/docker/image-size/nektos/act-environments-ubuntu/18.04-lite
[hub/nektos/act-environments-ubuntu/18.04-full/size]: https://img.shields.io/docker/image-size/nektos/act-environments-ubuntu/18.04-full

<!--
[hub/<username>/<image>]: https://hub.docker.com/r/[username]/[image]
[hub/<username>/<image>/<tag>/size]: https://img.shields.io/docker/image-size/[username]/[image]/[tag]
-->

<!-- GitHub repository links -->

[gh/nektos/act-environments]: https://github.com/nektos/act-environments
[gh/actions/virtual-environments]: https://github.com/actions/virtual-environments
[gh/catthehacker/docker_images]: https://github.com/catthehacker/docker_images
[gh/catthehacker/virtual-environments-fork]: https://github.com/catthehacker/virtual-environments-fork
