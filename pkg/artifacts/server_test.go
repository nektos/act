package artifacts

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/julienschmidt/httprouter"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	"github.com/nektos/act/pkg/model"
	"github.com/nektos/act/pkg/runner"
)

type writableMapFile struct {
	fstest.MapFile
}

func (f *writableMapFile) Write(data []byte) (int, error) {
	f.Data = data
	return len(data), nil
}

func (f *writableMapFile) Close() error {
	return nil
}

type writeMapFS struct {
	fstest.MapFS
}

func (fsys writeMapFS) OpenWritable(name string) (WritableFile, error) {
	var file = &writableMapFile{
		MapFile: fstest.MapFile{
			Data: []byte("content2"),
		},
	}
	fsys.MapFS[name] = &file.MapFile

	return file, nil
}

func (fsys writeMapFS) OpenAppendable(name string) (WritableFile, error) {
	var file = &writableMapFile{
		MapFile: fstest.MapFile{
			Data: []byte("content2"),
		},
	}
	fsys.MapFS[name] = &file.MapFile

	return file, nil
}

func TestNewArtifactUploadPrepare(t *testing.T) {
	assert := assert.New(t)

	var memfs = fstest.MapFS(map[string]*fstest.MapFile{})

	router := httprouter.New()
	uploads(router, "artifact/server/path", writeMapFS{memfs})

	req, _ := http.NewRequest("POST", "http://localhost/_apis/pipelines/workflows/1/artifacts", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		assert.Fail("Wrong status")
	}

	response := FileContainerResourceURL{}
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	if err != nil {
		panic(err)
	}

	assert.Equal("http://localhost/upload/1", response.FileContainerResourceURL)
}

func TestArtifactUploadBlob(t *testing.T) {
	assert := assert.New(t)

	var memfs = fstest.MapFS(map[string]*fstest.MapFile{})

	router := httprouter.New()
	uploads(router, "artifact/server/path", writeMapFS{memfs})

	req, _ := http.NewRequest("PUT", "http://localhost/upload/1?itemPath=some/file", strings.NewReader("content"))
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		assert.Fail("Wrong status")
	}

	response := ResponseMessage{}
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	if err != nil {
		panic(err)
	}

	assert.Equal("success", response.Message)
	assert.Equal("content", string(memfs["artifact/server/path/1/some/file"].Data))
}

func TestFinalizeArtifactUpload(t *testing.T) {
	assert := assert.New(t)

	var memfs = fstest.MapFS(map[string]*fstest.MapFile{})

	router := httprouter.New()
	uploads(router, "artifact/server/path", writeMapFS{memfs})

	req, _ := http.NewRequest("PATCH", "http://localhost/_apis/pipelines/workflows/1/artifacts", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		assert.Fail("Wrong status")
	}

	response := ResponseMessage{}
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	if err != nil {
		panic(err)
	}

	assert.Equal("success", response.Message)
}

func TestListArtifacts(t *testing.T) {
	assert := assert.New(t)

	var memfs = fstest.MapFS(map[string]*fstest.MapFile{
		"artifact/server/path/1/file.txt": {
			Data: []byte(""),
		},
	})

	router := httprouter.New()
	downloads(router, "artifact/server/path", memfs)

	req, _ := http.NewRequest("GET", "http://localhost/_apis/pipelines/workflows/1/artifacts", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		assert.FailNow(fmt.Sprintf("Wrong status: %d", status))
	}

	response := NamedFileContainerResourceURLResponse{}
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	if err != nil {
		panic(err)
	}

	assert.Equal(1, response.Count)
	assert.Equal("file.txt", response.Value[0].Name)
	assert.Equal("http://localhost/download/1", response.Value[0].FileContainerResourceURL)
}

func TestListArtifactContainer(t *testing.T) {
	assert := assert.New(t)

	var memfs = fstest.MapFS(map[string]*fstest.MapFile{
		"artifact/server/path/1/some/file": {
			Data: []byte(""),
		},
	})

	router := httprouter.New()
	downloads(router, "artifact/server/path", memfs)

	req, _ := http.NewRequest("GET", "http://localhost/download/1?itemPath=some/file", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		assert.FailNow(fmt.Sprintf("Wrong status: %d", status))
	}

	response := ContainerItemResponse{}
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	if err != nil {
		panic(err)
	}

	assert.Equal(1, len(response.Value))
	assert.Equal("some/file", response.Value[0].Path)
	assert.Equal("file", response.Value[0].ItemType)
	assert.Equal("http://localhost/artifact/1/some/file/.", response.Value[0].ContentLocation)
}

func TestDownloadArtifactFile(t *testing.T) {
	assert := assert.New(t)

	var memfs = fstest.MapFS(map[string]*fstest.MapFile{
		"artifact/server/path/1/some/file": {
			Data: []byte("content"),
		},
	})

	router := httprouter.New()
	downloads(router, "artifact/server/path", memfs)

	req, _ := http.NewRequest("GET", "http://localhost/artifact/1/some/file", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		assert.FailNow(fmt.Sprintf("Wrong status: %d", status))
	}

	data := rr.Body.Bytes()

	assert.Equal("content", string(data))
}

type TestJobFileInfo struct {
	workdir               string
	workflowPath          string
	eventName             string
	errorMessage          string
	platforms             map[string]string
	containerArchitecture string
}

var (
	artifactsPath = path.Join(os.TempDir(), "test-artifacts")
	artifactsAddr = "127.0.0.1"
	artifactsPort = "12345"
)

func TestArtifactFlow(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()

	cancel := Serve(ctx, artifactsPath, artifactsAddr, artifactsPort)
	defer cancel()

	platforms := map[string]string{
		"ubuntu-latest": "node:16-buster", // Don't use node:16-buster-slim because it doesn't have curl command, which is used in the tests
	}

	tables := []TestJobFileInfo{
		{"testdata", "upload-and-download", "push", "", platforms, ""},
		{"testdata", "GHSL-2023-004", "push", "", platforms, ""},
		{"testdata", "v4", "push", "", platforms, ""},
	}
	log.SetLevel(log.DebugLevel)

	for _, table := range tables {
		runTestJobFile(ctx, t, table)
	}
}

func runTestJobFile(ctx context.Context, t *testing.T, tjfi TestJobFileInfo) {
	t.Run(tjfi.workflowPath, func(t *testing.T) {
		fmt.Printf("::group::%s\n", tjfi.workflowPath)

		if err := os.RemoveAll(artifactsPath); err != nil {
			panic(err)
		}

		workdir, err := filepath.Abs(tjfi.workdir)
		assert.Nil(t, err, workdir)
		fullWorkflowPath := filepath.Join(workdir, tjfi.workflowPath)
		runnerConfig := &runner.Config{
			Workdir:               workdir,
			BindWorkdir:           false,
			EventName:             tjfi.eventName,
			Platforms:             tjfi.platforms,
			ReuseContainers:       false,
			ContainerArchitecture: tjfi.containerArchitecture,
			GitHubInstance:        "github.com",
			ArtifactServerPath:    artifactsPath,
			ArtifactServerAddr:    artifactsAddr,
			ArtifactServerPort:    artifactsPort,
		}

		runner, err := runner.New(runnerConfig)
		assert.Nil(t, err, tjfi.workflowPath)

		planner, err := model.NewWorkflowPlanner(fullWorkflowPath, true)
		assert.Nil(t, err, fullWorkflowPath)

		plan, err := planner.PlanEvent(tjfi.eventName)
		if err == nil {
			err = runner.NewPlanExecutor(plan)(ctx)
			if tjfi.errorMessage == "" {
				assert.Nil(t, err, fullWorkflowPath)
			} else {
				assert.Error(t, err, tjfi.errorMessage)
			}
		} else {
			assert.Nil(t, plan)
		}

		fmt.Println("::endgroup::")
	})
}

func TestMkdirFsImplSafeResolve(t *testing.T) {
	baseDir := "/foo/bar"

	tests := map[string]struct {
		input string
		want  string
	}{
		"simple":         {input: "baz", want: "/foo/bar/baz"},
		"nested":         {input: "baz/blue", want: "/foo/bar/baz/blue"},
		"dots in middle": {input: "baz/../../blue", want: "/foo/bar/blue"},
		"leading dots":   {input: "../../parent", want: "/foo/bar/parent"},
		"root path":      {input: "/root", want: "/foo/bar/root"},
		"root":           {input: "/", want: "/foo/bar"},
		"empty":          {input: "", want: "/foo/bar"},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			assert := assert.New(t)
			assert.Equal(tc.want, safeResolve(baseDir, tc.input))
		})
	}
}

func TestDownloadArtifactFileUnsafePath(t *testing.T) {
	assert := assert.New(t)

	var memfs = fstest.MapFS(map[string]*fstest.MapFile{
		"artifact/server/path/some/file": {
			Data: []byte("content"),
		},
	})

	router := httprouter.New()
	downloads(router, "artifact/server/path", memfs)

	req, _ := http.NewRequest("GET", "http://localhost/artifact/2/../../some/file", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		assert.FailNow(fmt.Sprintf("Wrong status: %d", status))
	}

	data := rr.Body.Bytes()

	assert.Equal("content", string(data))
}

func TestArtifactUploadBlobUnsafePath(t *testing.T) {
	assert := assert.New(t)

	var memfs = fstest.MapFS(map[string]*fstest.MapFile{})

	router := httprouter.New()
	uploads(router, "artifact/server/path", writeMapFS{memfs})

	req, _ := http.NewRequest("PUT", "http://localhost/upload/1?itemPath=../../some/file", strings.NewReader("content"))
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		assert.Fail("Wrong status")
	}

	response := ResponseMessage{}
	err := json.Unmarshal(rr.Body.Bytes(), &response)
	if err != nil {
		panic(err)
	}

	assert.Equal("success", response.Message)
	assert.Equal("content", string(memfs["artifact/server/path/1/some/file"].Data))
}
