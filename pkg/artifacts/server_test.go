package artifacts

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/julienschmidt/httprouter"
	"github.com/stretchr/testify/assert"
)

type MapFsImpl struct {
	fstest.MapFS
}

func (fsys MapFsImpl) MkdirAll(path string, perm fs.FileMode) error {
	// mocked no-op
	return nil
}

type WritableFile struct {
	fs.File
	fsys fstest.MapFS
	path string
}

func (file WritableFile) Write(data []byte) (int, error) {
	file.fsys[file.path].Data = data
	return len(data), nil
}

func (fsys MapFsImpl) Open(path string) (fs.File, error) {
	var file = fstest.MapFile{
		Data: []byte("content2"),
	}
	fsys.MapFS[path] = &file

	result, err := fsys.MapFS.Open(path)
	return WritableFile{result, fsys.MapFS, path}, err
}

func TestNewArtifactUploadPrepare(t *testing.T) {
	assert := assert.New(t)

	var memfs = fstest.MapFS(map[string]*fstest.MapFile{})

	router := httprouter.New()
	uploads(router, MapFsImpl{memfs})

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
	uploads(router, MapFsImpl{memfs})

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
	assert.Equal("content", string(memfs["1/some/file"].Data))
}

func TestFinalizeArtifactUpload(t *testing.T) {
	assert := assert.New(t)

	var memfs = fstest.MapFS(map[string]*fstest.MapFile{})

	router := httprouter.New()
	uploads(router, MapFsImpl{memfs})

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
		"1/file.txt": {
			Data: []byte(""),
		},
	})

	router := httprouter.New()
	downloads(router, memfs)

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
		"1/some/file": {
			Data: []byte(""),
		},
	})

	router := httprouter.New()
	downloads(router, memfs)

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
	assert.Equal("some/file/.", response.Value[0].Path)
	assert.Equal("file", response.Value[0].ItemType)
	assert.Equal("http://localhost/artifact/1/some/file/.", response.Value[0].ContentLocation)
}

func TestDownloadArtifactFile(t *testing.T) {
	assert := assert.New(t)

	var memfs = fstest.MapFS(map[string]*fstest.MapFile{
		"1/some/file": {
			Data: []byte("content"),
		},
	})

	router := httprouter.New()
	downloads(router, memfs)

	req, _ := http.NewRequest("GET", "http://localhost/artifact/1/some/file", nil)
	rr := httptest.NewRecorder()

	router.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		assert.FailNow(fmt.Sprintf("Wrong status: %d", status))
	}

	data := rr.Body.Bytes()

	assert.Equal("content", string(data))
}
