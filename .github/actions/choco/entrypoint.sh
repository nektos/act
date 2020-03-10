#!/bin/bash

set -e

function choco {
  mono /opt/chocolatey/choco.exe "$@" --allow-unofficial --nocolor
}

function get_version {
  local version=${INPUT_VERSION:-$(git describe --tags)}
  version=(${version//[!0-9.-]/})
  local version_parts=(${version//-/ })
  version=${version_parts[0]}
  if [ ${#version_parts[@]} -gt 1 ]; then
    version=${version_parts}.${version_parts[1]}
  fi
  echo "$version"
}

## Determine the version to pack
VERSION=$(get_version)
echo "Packing version ${VERSION} of act"
rm -f act-cli.*.nupkg
mkdir -p tools
cp LICENSE tools/LICENSE.txt
cp VERIFICATION tools/VERIFICATION.txt
cp dist/act_windows_amd64/act.exe tools/
choco pack act-cli.nuspec --version ${VERSION}
choco push act-cli.${VERSION}.nupkg --api-key ${INPUT_APIKEY} -s https://push.chocolatey.org/ --timeout 180
