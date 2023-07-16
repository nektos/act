package runner

import (
	"archive/tar"
	"bytes"
	"context"
	"io"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestActionCache(t *testing.T) {
	a := assert.New(t)
	cache := &GoGitActionCache{
		Path: os.TempDir(),
	}
	ctx := context.Background()
	sha, err := cache.Fetch(ctx, "christopherhx/script", "https://github.com/christopherhx/script", "main", "")
	a.NoError(err)
	a.NotEmpty(sha)
	atar, err := cache.GetTarArchive(ctx, "christopherhx/script", sha, "node_modules")
	a.NoError(err)
	a.NotEmpty(atar)
	mytar := tar.NewReader(atar)
	th, err := mytar.Next()
	a.NoError(err)
	a.NotEqual(0, th.Size)
	buf := &bytes.Buffer{}
	_, err = io.Copy(buf, mytar)
	a.NoError(err)
	str := buf.String()
	a.NotEmpty(str)
}
