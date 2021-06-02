package artifacts

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
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

func TestFullArtifactsTest(t *testing.T) {
	workflow := `
name: "Test that artifact uploads and downloads succeed"
on: push

jobs:
  test-artifacts:
    runs-on: ubuntu-latest
    steps:
      - run: mkdir -p path/to/artifact
      - run: echo hello > path/to/artifact/world.txt
      - uses: actions/upload-artifact@v2
        with:
          name: my-artifact
          path: path/to/artifact/world.txt

      - run: rm -rf path

      - uses: actions/download-artifact@v2
        with:
          name: my-artifact
      - name: Display structure of downloaded files
        run: ls -la

      # Test end-to-end by uploading two artifacts and then downloading them
      - name: Create artifact files
        run: |
          mkdir -p path/to/dir-1
          mkdir -p path/to/dir-2
          mkdir -p path/to/dir-3    
          echo "Lorem ipsum dolor sit amet" > path/to/dir-1/file1.txt
          echo "Hello world from file #2" > path/to/dir-2/file2.txt
          echo "This is a going to be a test for a large enough file that should get compressed with GZip. The @actions/artifact package uses GZip to upload files. This text should have a compression ratio greater than 100% so it should get uploaded using GZip" > path/to/dir-3/gzip.txt
      # Upload a single file artifact
      - name: 'Upload artifact #1'
        uses: actions/upload-artifact@v2
        with:
          name: 'Artifact-A'
          path: path/to/dir-1/file1.txt
      
      # Upload using a wildcard pattern, name should default to 'artifact' if not provided
      - name: 'Upload artifact #2'
        uses: actions/upload-artifact@v2
        with:
          path: path/**/dir*/
      
      # Upload a directory that contains a file that will be uploaded with GZip
      - name: 'Upload artifact #3'
        uses: actions/upload-artifact@v2
        with:
          name: 'GZip-Artifact'
          path: path/to/dir-3/
      
      # Upload a directory that contains a file that will be uploaded with GZip
      - name: 'Upload artifact #4'
        uses: actions/upload-artifact@v2
        with:
          name: 'Multi-Path-Artifact'
          path: |
            path/to/dir-1/*
            path/to/dir-[23]/*
            !path/to/dir-3/*.txt
      # Verify artifacts. Switch to download-artifact@v2 once it's out of preview
      
      # Download Artifact #1 and verify the correctness of the content
      - name: 'Download artifact #1'
        uses: actions/download-artifact@v2
        with:
          name: 'Artifact-A'
          path: some/new/path
      
      - name: 'Verify Artifact #1'
        run: |
          file="some/new/path/file1.txt"
          if [ ! -f $file ] ; then
            echo "Expected file does not exist"
            exit 1
          fi
          if [ "$(cat $file)" != "Lorem ipsum dolor sit amet" ] ; then
            echo "File contents of downloaded artifact are incorrect"
            exit 1
          fi
      
      # Download Artifact #2 and verify the correctness of the content
      - name: 'Download artifact #2'
        uses: actions/download-artifact@v2
        with:
          name: 'artifact'
          path: some/other/path
      
      - name: 'Verify Artifact #2'
        run: |
          file1="some/other/path/to/dir-1/file1.txt"
          file2="some/other/path/to/dir-2/file2.txt"
          if [ ! -f $file1 -o ! -f $file2 ] ; then
            echo "Expected files do not exist"
            exit 1
          fi
          if [ "$(cat $file1)" != "Lorem ipsum dolor sit amet" -o "$(cat $file2)" != "Hello world from file #2" ] ; then
            echo "File contents of downloaded artifacts are incorrect"
            exit 1
          fi
      
      # Download Artifact #3 and verify the correctness of the content
      - name: 'Download artifact #3'
        uses: actions/download-artifact@v2
        with:
          name: 'GZip-Artifact'
          path: gzip/artifact/path
      
      # Because a directory was used as input during the upload the parent directories, path/to/dir-3/, should not be included in the uploaded artifact
      - name: 'Verify Artifact #3'
        run: |
          gzipFile="gzip/artifact/path/gzip.txt"
          if [ ! -f $gzipFile ] ; then
            echo "Expected file do not exist"
            exit 1
          fi
          if [ "$(cat $gzipFile)" != "This is a going to be a test for a large enough file that should get compressed with GZip. The @actions/artifact package uses GZip to upload files. This text should have a compression ratio greater than 100% so it should get uploaded using GZip" ] ; then
            echo "File contents of downloaded artifact is incorrect"
            exit 1
          fi
      
      - name: 'Download artifact #4'
        uses: actions/download-artifact@v2
        with:
          name: 'Multi-Path-Artifact'
          path: multi/artifact
      
      - name: 'Verify Artifact #4'
        run: |
          file1="multi/artifact/dir-1/file1.txt"
          file2="multi/artifact/dir-2/file2.txt"
          if [ ! -f $file1 -o ! -f $file2 ] ; then
            echo "Expected files do not exist"
            exit 1
          fi
          if [ "$(cat $file1)" != "Lorem ipsum dolor sit amet" -o "$(cat $file2)" != "Hello world from file #2" ] ; then
            echo "File contents of downloaded artifacts are incorrect"
            exit 1
          fi
`

	file, err := os.Create("../../.github/workflows/test-artifacts.yml")
	if err != nil {
		t.Fatal(err)
	}

	_, err = file.WriteString(workflow)
	if err != nil {
		t.Fatal(err)
	}
}
