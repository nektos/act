package artifacts

import (
	"bytes"
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
	"text/template"

	"github.com/julienschmidt/httprouter"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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

type artifactActionVersionPair struct {
	UploadVersion   string
	DownloadVersion string
	platforms       map[string]string
}

func (pair artifactActionVersionPair) name() string {
	return fmt.Sprintf("upload-%s-download-%s", pair.UploadVersion, pair.DownloadVersion)
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

	node16platforms := map[string]string{
		"ubuntu-latest": "node:16-buster", // Don't use node:16-buster-slim because it doesn't have curl command, which is used in the tests
	}
	node20Platforms := map[string]string{
		"ubuntu-latest": "node:20-bookworm-slim",
	}
	node24Platforms := map[string]string{
		"ubuntu-latest": "node:24-bookworm-slim",
	}

	tables := []TestJobFileInfo{
		{"testdata", "upload-and-download", "push", "", node16platforms, ""},
		{"testdata", "GHSL-2023-004", "push", "", node16platforms, ""},
		{"testdata", "versions/direct-artifacts", "push", "", node24Platforms, ""},
	}

	templateSource, err := os.ReadFile("testdata/versions/versioned-artifacts.yml.tmpl")
	require.NoError(t, err)
	workflowTemplate, err := template.New("versioned-artifacts").
		Delims("<<", ">>").
		Option("missingkey=error").
		Parse(string(templateSource))
	require.NoError(t, err)

	versionPairs := []artifactActionVersionPair{
		{UploadVersion: "v4", DownloadVersion: "v4", platforms: node20Platforms},
		{UploadVersion: "v4", DownloadVersion: "v5", platforms: node20Platforms},
		{UploadVersion: "v5", DownloadVersion: "v6", platforms: node20Platforms},
		{UploadVersion: "v6", DownloadVersion: "v7", platforms: node24Platforms},
		{UploadVersion: "v7", DownloadVersion: "v8", platforms: node24Platforms},
	}
	generatedWorkflows := t.TempDir()
	for _, pair := range versionPairs {
		workflowPath := pair.name()
		workflowDir := filepath.Join(generatedWorkflows, workflowPath)
		require.NoError(t, os.MkdirAll(workflowDir, 0o700))

		var workflow bytes.Buffer
		require.NoError(t, workflowTemplate.Execute(&workflow, pair))
		require.NoError(t, os.WriteFile(filepath.Join(workflowDir, "artifacts.yml"), workflow.Bytes(), 0o600))

		tables = append(tables, TestJobFileInfo{generatedWorkflows, workflowPath, "push", "", pair.platforms, ""})
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

		planner, err := model.NewWorkflowPlanner(fullWorkflowPath, true, false)
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

func TestArtifactV4AcceptsCurrentClientRequests(t *testing.T) {
	memfs := fstest.MapFS{}
	router := httprouter.New()
	RoutesV4(router, "artifact/server/path", writeMapFS{memfs}, memfs)

	createBody := `{"workflow_run_backend_id":"1","workflow_job_run_backend_id":"1","name":"direct.txt","version":7,"mime_type":"text/plain"}`
	createReq, _ := http.NewRequest(http.MethodPost, "http://localhost"+path.Join(ArtifactV4RouteBase, "CreateArtifact"), strings.NewReader(createBody))
	createResp := httptest.NewRecorder()
	router.ServeHTTP(createResp, createReq)
	assert.Equal(t, http.StatusOK, createResp.Code)

	var artifact struct {
		SignedUploadURL string `json:"signedUploadUrl"`
		SignedURL       string `json:"signedUrl"`
	}
	assert.NoError(t, json.Unmarshal(createResp.Body.Bytes(), &artifact))

	// The current Azure client removes base64 padding when adding blob query parameters.
	uploadURL := strings.Replace(artifact.SignedUploadURL, "=&expires", "&expires", 1) + "&comp=block"
	uploadReq, _ := http.NewRequest(http.MethodPut, uploadURL, strings.NewReader("content"))
	uploadResp := httptest.NewRecorder()
	router.ServeHTTP(uploadResp, uploadReq)
	assert.Equal(t, http.StatusCreated, uploadResp.Code)

	getURLBody := `{"workflow_run_backend_id":"1","workflow_job_run_backend_id":"1","name":"direct.txt"}`
	getURLReq, _ := http.NewRequest(http.MethodPost, "http://localhost"+path.Join(ArtifactV4RouteBase, "GetSignedArtifactURL"), strings.NewReader(getURLBody))
	getURLResp := httptest.NewRecorder()
	router.ServeHTTP(getURLResp, getURLReq)
	require.Equal(t, http.StatusOK, getURLResp.Code)
	require.NoError(t, json.Unmarshal(getURLResp.Body.Bytes(), &artifact))

	downloadReq, _ := http.NewRequest(http.MethodGet, artifact.SignedURL, nil)
	downloadResp := httptest.NewRecorder()
	router.ServeHTTP(downloadResp, downloadReq)
	assert.Equal(t, http.StatusOK, downloadResp.Code)
	assert.Equal(t, "text/plain", downloadResp.Header().Get("Content-Type"))
	assert.Equal(t, "attachment; filename=direct.txt", downloadResp.Header().Get("Content-Disposition"))
	assert.Equal(t, "content", downloadResp.Body.String())
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
