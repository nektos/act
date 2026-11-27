package container

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/docker/cli/cli/config"

	log "github.com/sirupsen/logrus"
	assert "github.com/stretchr/testify/assert"
)

func init() {
	log.SetLevel(log.DebugLevel)
}

func TestCleanImage(t *testing.T) {
	tables := []struct {
		imageIn  string
		imageOut string
	}{
		{"myhost.com/foo/bar", "myhost.com/foo/bar"},
		{"localhost:8000/canonical/ubuntu", "localhost:8000/canonical/ubuntu"},
		{"localhost/canonical/ubuntu:latest", "localhost/canonical/ubuntu:latest"},
		{"localhost:8000/canonical/ubuntu:latest", "localhost:8000/canonical/ubuntu:latest"},
		{"ubuntu", "docker.io/library/ubuntu"},
		{"ubuntu:18.04", "docker.io/library/ubuntu:18.04"},
		{"cibuilds/hugo:0.53", "docker.io/cibuilds/hugo:0.53"},
	}

	for _, table := range tables {
		imageOut := cleanImage(context.Background(), table.imageIn)
		assert.Equal(t, table.imageOut, imageOut)
	}
}

func TestGetImagePullOptions(t *testing.T) {
	ctx := context.Background()

	config.SetDir("/non-existent/docker")

	options, err := getImagePullOptions(ctx, NewDockerPullExecutorInput{})
	assert.Nil(t, err, "Failed to create ImagePullOptions")
	assert.Equal(t, "", options.RegistryAuth, "RegistryAuth should be empty if no username or password is set")

	options, err = getImagePullOptions(ctx, NewDockerPullExecutorInput{
		Image:    "",
		Username: "username",
		Password: "password",
	})
	assert.Nil(t, err, "Failed to create ImagePullOptions")
	assert.Equal(t, "eyJ1c2VybmFtZSI6InVzZXJuYW1lIiwicGFzc3dvcmQiOiJwYXNzd29yZCJ9", options.RegistryAuth, "Username and Password should be provided")

	config.SetDir("testdata/docker-pull-options")

	options, err = getImagePullOptions(ctx, NewDockerPullExecutorInput{
		Image: "nektos/act",
	})
	assert.Nil(t, err, "Failed to create ImagePullOptions")
	assert.Equal(t, "eyJ1c2VybmFtZSI6InVzZXJuYW1lIiwicGFzc3dvcmQiOiJwYXNzd29yZFxuIiwic2VydmVyYWRkcmVzcyI6Imh0dHBzOi8vaW5kZXguZG9ja2VyLmlvL3YxLyJ9", options.RegistryAuth, "RegistryAuth should be taken from local docker config")
}

func TestDockerPullSendsDockerHubAuthForBothImageNames(t *testing.T) {
	previous := config.Dir()
	config.SetDir("testdata/docker-pull-options")
	t.Cleanup(func() { config.SetDir(previous) })

	type pullRequest struct {
		image   string
		hasAuth bool
	}
	requests := make(chan pullRequest, 2)
	docker := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/_ping"):
			w.Header().Set("API-Version", "1.48")
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/images/create"):
			select {
			case requests <- pullRequest{r.URL.Query().Get("fromImage"), r.Header.Get("X-Registry-Auth") != ""}:
			default:
				t.Error("unexpected extra Docker pull")
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte("{\"status\":\"downloaded\"}\n"))
		default:
			t.Errorf("unexpected Docker API call: %s %s", r.Method, r.URL)
			http.NotFound(w, r)
		}
	}))
	defer docker.Close()
	t.Setenv("DOCKER_HOST", docker.URL)
	t.Setenv("DOCKER_TLS_VERIFY", "")
	t.Setenv("DOCKER_CERT_PATH", "")
	for _, image := range []string{"nektos/act", "docker.io/nektos/act"} {
		pull := NewDockerPullExecutor(NewDockerPullExecutorInput{Image: image, ForcePull: true})
		if err := pull(context.Background()); err != nil {
			t.Fatalf("pull %q: %v", image, err)
		}
		select {
		case request := <-requests:
			t.Logf("%s → %s, X-Registry-Auth present=%t", image, request.image, request.hasAuth)
			assert.True(t, request.hasAuth, "%s should use Docker Hub credentials", image)
		default:
			t.Fatalf("no Docker API pull for %q", image)
		}
	}
}
