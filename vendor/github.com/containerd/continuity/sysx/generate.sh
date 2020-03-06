#!/bin/bash

#   Copyright The containerd Authors.

#   Licensed under the Apache License, Version 2.0 (the "License");
#   you may not use this file except in compliance with the License.
#   You may obtain a copy of the License at

#       http://www.apache.org/licenses/LICENSE-2.0

#   Unless required by applicable law or agreed to in writing, software
#   distributed under the License is distributed on an "AS IS" BASIS,
#   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
#   See the License for the specific language governing permissions and
#   limitations under the License.


set -e

mksyscall="$(go env GOROOT)/src/syscall/mksyscall.pl"

fix() {
	sed 's,^package syscall$,package sysx,' \
		| sed 's,^import "unsafe"$,import (\n\t"syscall"\n\t"unsafe"\n),' \
		| gofmt -r='BytePtrFromString -> syscall.BytePtrFromString' \
		| gofmt -r='Syscall6 -> syscall.Syscall6' \
		| gofmt -r='Syscall -> syscall.Syscall' \
		| gofmt -r='SYS_GETXATTR -> syscall.SYS_GETXATTR' \
		| gofmt -r='SYS_LISTXATTR -> syscall.SYS_LISTXATTR' \
		| gofmt -r='SYS_SETXATTR -> syscall.SYS_SETXATTR' \
		| gofmt -r='SYS_REMOVEXATTR -> syscall.SYS_REMOVEXATTR' \
		| gofmt -r='SYS_LGETXATTR -> syscall.SYS_LGETXATTR' \
		| gofmt -r='SYS_LLISTXATTR -> syscall.SYS_LLISTXATTR' \
		| gofmt -r='SYS_LSETXATTR -> syscall.SYS_LSETXATTR' \
		| gofmt -r='SYS_LREMOVEXATTR -> syscall.SYS_LREMOVEXATTR'
}

if [ "$GOARCH" == "" ] || [ "$GOOS" == "" ]; then
	echo "Must specify \$GOARCH and \$GOOS"
	exit 1
fi

mkargs=""

if [ "$GOARCH" == "386" ] || [ "$GOARCH" == "arm" ]; then
	mkargs="-l32"
fi

for f in "$@"; do
	$mksyscall $mkargs "${f}_${GOOS}.go" | fix > "${f}_${GOOS}_${GOARCH}.go"
done

