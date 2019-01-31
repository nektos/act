package model

import (
	"fmt"
)

type Uses interface {
	fmt.Stringer
	isUses()
}

// UsesDockerImage represents `uses = "docker://<image>"`
type UsesDockerImage struct {
	Image string
}

// UsesRepository represents `uses = "<owner>/<repo>[/<path>]@<ref>"`
type UsesRepository struct {
	Repository string
	Path       string
	Ref        string
}

// UsesPath represents `uses = "./<path>"`
type UsesPath struct {
	Path string
}

// UsesInvalid represents any invalid `uses = "<raw>"` value
type UsesInvalid struct {
	Raw string
}

func (u *UsesDockerImage) isUses() {}
func (u *UsesRepository) isUses()  {}
func (u *UsesPath) isUses()        {}
func (u *UsesInvalid) isUses()     {}

func (u *UsesDockerImage) String() string {
	return fmt.Sprintf("docker://%s", u.Image)
}

func (u *UsesRepository) String() string {
	if u.Path == "" {
		return fmt.Sprintf("%s@%s", u.Repository, u.Ref)
	}

	return fmt.Sprintf("%s/%s@%s", u.Repository, u.Path, u.Ref)
}

func (u *UsesPath) String() string {
	return fmt.Sprintf("./%s", u.Path)
}

func (u *UsesInvalid) String() string {
	return u.Raw
}
