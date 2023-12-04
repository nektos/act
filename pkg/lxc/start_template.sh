#!/bin/sh -xe
lxc-create --name="{{.Name}}" --template={{.Template}} -- --release {{.Release}} $packages
tee -a /var/lib/lxc/{{.Name}}/config <<'EOF'
security.nesting = true
lxc.cap.drop =
lxc.apparmor.profile = unconfined
#
# /dev/net (docker won't work without /dev/net/tun)
#
lxc.cgroup2.devices.allow = c 10:200 rwm
lxc.mount.entry = /dev/net dev/net none bind,create=dir 0 0
#
# /dev/kvm (libvirt / kvm won't work without /dev/kvm)
#
lxc.cgroup2.devices.allow = c 10:232 rwm
lxc.mount.entry = /dev/kvm dev/kvm none bind,create=file 0 0
#
# /dev/loop
#
lxc.cgroup2.devices.allow = c 10:237 rwm
lxc.cgroup2.devices.allow = b 7:* rwm
lxc.mount.entry = /dev/loop-control dev/loop-control none bind,create=file 0 0
#
# /dev/mapper
#
lxc.cgroup2.devices.allow = c 10:236 rwm
lxc.mount.entry = /dev/mapper dev/mapper none bind,create=dir 0 0
#
# /dev/fuse
#
lxc.cgroup2.devices.allow = b 10:229 rwm
lxc.mount.entry = /dev/fuse dev/fuse none bind,create=file 0 0
EOF

mkdir -p /var/lib/lxc/{{.Name}}/rootfs/{{ .Root }}
mount --bind {{ .Root }} /var/lib/lxc/{{.Name}}/rootfs/{{ .Root }}

mkdir /var/lib/lxc/{{.Name}}/rootfs/tmpdir
mount --bind {{.TmpDir}} /var/lib/lxc/{{.Name}}/rootfs/tmpdir

lxc-start {{.Name}}
lxc-wait --name {{.Name}} --state RUNNING

#
# Wait for the network to come up
#
cat > /var/lib/lxc/{{.Name}}/rootfs/tmpdir/networking.sh <<'EOF'
#!/bin/sh -xe
for d in $(seq 60); do
  getent hosts wikipedia.org > /dev/null && break
  sleep 1
done
getent hosts wikipedia.org
EOF
chmod +x /var/lib/lxc/{{.Name}}/rootfs/tmpdir/networking.sh

lxc-attach --name {{.Name}} -- /tmpdir/networking.sh

cat > /var/lib/lxc/{{.Name}}/rootfs/tmpdir/node.sh <<'EOF'
#!/bin/sh -xe
# https://github.com/nodesource/distributions#debinstall
apt-get install -y curl git
curl -fsSL https://deb.nodesource.com/setup_16.x | bash -
apt-get install -y nodejs
EOF
chmod +x /var/lib/lxc/{{.Name}}/rootfs/tmpdir/node.sh

lxc-attach --name {{.Name}} -- /tmpdir/node.sh