#!/bin/sh -x
lxc-ls -1 --filter="^{{.Name}}" | while read container ; do
   lxc-stop --kill --name="$container"
   umount "/var/lib/lxc/$container/rootfs/{{ .Root }}"
   umount "/var/lib/lxc/$container/rootfs/tmpdir"
   lxc-destroy --force --name="$container"
done
