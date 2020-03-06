package mount // import "github.com/docker/docker/pkg/mount"

// MakeShared ensures a mounted filesystem has the SHARED mount option enabled.
// See the supported options in flags.go for further reference.
func MakeShared(mountPoint string) error {
	return ensureMountedAs(mountPoint, SHARED)
}

// MakeRShared ensures a mounted filesystem has the RSHARED mount option enabled.
// See the supported options in flags.go for further reference.
func MakeRShared(mountPoint string) error {
	return ensureMountedAs(mountPoint, RSHARED)
}

// MakePrivate ensures a mounted filesystem has the PRIVATE mount option enabled.
// See the supported options in flags.go for further reference.
func MakePrivate(mountPoint string) error {
	return ensureMountedAs(mountPoint, PRIVATE)
}

// MakeRPrivate ensures a mounted filesystem has the RPRIVATE mount option
// enabled. See the supported options in flags.go for further reference.
func MakeRPrivate(mountPoint string) error {
	return ensureMountedAs(mountPoint, RPRIVATE)
}

// MakeSlave ensures a mounted filesystem has the SLAVE mount option enabled.
// See the supported options in flags.go for further reference.
func MakeSlave(mountPoint string) error {
	return ensureMountedAs(mountPoint, SLAVE)
}

// MakeRSlave ensures a mounted filesystem has the RSLAVE mount option enabled.
// See the supported options in flags.go for further reference.
func MakeRSlave(mountPoint string) error {
	return ensureMountedAs(mountPoint, RSLAVE)
}

// MakeUnbindable ensures a mounted filesystem has the UNBINDABLE mount option
// enabled. See the supported options in flags.go for further reference.
func MakeUnbindable(mountPoint string) error {
	return ensureMountedAs(mountPoint, UNBINDABLE)
}

// MakeRUnbindable ensures a mounted filesystem has the RUNBINDABLE mount
// option enabled. See the supported options in flags.go for further reference.
func MakeRUnbindable(mountPoint string) error {
	return ensureMountedAs(mountPoint, RUNBINDABLE)
}

// MakeMount ensures that the file or directory given is a mount point,
// bind mounting it to itself it case it is not.
func MakeMount(mnt string) error {
	mounted, err := Mounted(mnt)
	if err != nil {
		return err
	}
	if mounted {
		return nil
	}

	return mount(mnt, mnt, "none", uintptr(BIND), "")
}

func ensureMountedAs(mnt string, flags int) error {
	if err := MakeMount(mnt); err != nil {
		return err
	}

	return mount("", mnt, "none", uintptr(flags), "")
}
