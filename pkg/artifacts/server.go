package artifacts

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/julienschmidt/httprouter"
	"github.com/nektos/act/pkg/common"
)

type FileContainerResourceURL struct {
	FileContainerResourceURL string `json:"fileContainerResourceUrl"`
}

type NamedFileContainerResourceURL struct {
	Name                     string `json:"name"`
	FileContainerResourceURL string `json:"fileContainerResourceUrl"`
}

type NamedFileContainerResourceURLResponse struct {
	Count int                             `json:"count"`
	Value []NamedFileContainerResourceURL `json:"value"`
}

type ContainerItem struct {
	Path            string `json:"path"`
	ItemType        string `json:"itemType"`
	ContentLocation string `json:"contentLocation"`
}

type ContainerItemResponse struct {
	Value []ContainerItem `json:"value"`
}

type ResponseMessage struct {
	Message string `json:"message"`
}

type MkdirFS interface {
	fs.FS
	MkdirAll(path string, perm fs.FileMode) error
	Open(name string) (fs.File, error)
	OpenAtEnd(name string) (fs.File, error)
}

type MkdirFsImpl struct {
	dir string
	fs.FS
}

func (fsys MkdirFsImpl) MkdirAll(path string, perm fs.FileMode) error {
	return os.MkdirAll(fsys.dir+"/"+path, perm)
}

func (fsys MkdirFsImpl) Open(name string) (fs.File, error) {
	return os.OpenFile(fsys.dir+"/"+name, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0644)
}

func (fsys MkdirFsImpl) OpenAtEnd(name string) (fs.File, error) {
	file, err := os.OpenFile(fsys.dir+"/"+name, os.O_CREATE|os.O_RDWR, 0644)

	if err != nil {
		return nil, err
	}

	_, err = file.Seek(0, os.SEEK_END)
	if err != nil {
		return nil, err
	}

	return file, nil
}

var gzipExtension = ".gz__"

func uploads(router *httprouter.Router, fsys MkdirFS) {
	router.POST("/_apis/pipelines/workflows/:runId/artifacts", func(w http.ResponseWriter, req *http.Request, params httprouter.Params) {
		runID := params.ByName("runId")

		json, err := json.Marshal(FileContainerResourceURL{
			FileContainerResourceURL: fmt.Sprintf("http://%s/upload/%s", req.Host, runID),
		})
		if err != nil {
			panic(err)
		}

		_, err = w.Write(json)
		if err != nil {
			panic(err)
		}
	})

	router.PUT("/upload/:runId", func(w http.ResponseWriter, req *http.Request, params httprouter.Params) {
		itemPath := req.URL.Query().Get("itemPath")
		runID := params.ByName("runId")

		if req.Header.Get("Content-Encoding") == "gzip" {
			itemPath += gzipExtension
		}

		filePath := fmt.Sprintf("%s/%s", runID, itemPath)

		err := fsys.MkdirAll(path.Dir(filePath), os.ModePerm)
		if err != nil {
			panic(err)
		}

		file, err := func() (fs.File, error) {
			contentRange := req.Header.Get("Content-Range")
			if contentRange != "" && !strings.HasPrefix(contentRange, "bytes 0-") {
				return fsys.OpenAtEnd(filePath)
			}
			return fsys.Open(filePath)
		}()

		if err != nil {
			panic(err)
		}
		defer file.Close()

		writer, ok := file.(io.Writer)
		if !ok {
			panic(errors.New("File is not writable"))
		}

		if req.Body == nil {
			panic(errors.New("No body given"))
		}

		_, err = io.Copy(writer, req.Body)
		if err != nil {
			panic(err)
		}

		json, err := json.Marshal(ResponseMessage{
			Message: "success",
		})
		if err != nil {
			panic(err)
		}

		_, err = w.Write(json)
		if err != nil {
			panic(err)
		}
	})

	router.PATCH("/_apis/pipelines/workflows/:runId/artifacts", func(w http.ResponseWriter, req *http.Request, params httprouter.Params) {
		json, err := json.Marshal(ResponseMessage{
			Message: "success",
		})
		if err != nil {
			panic(err)
		}

		_, err = w.Write(json)
		if err != nil {
			panic(err)
		}
	})
}

func downloads(router *httprouter.Router, fsys fs.FS) {
	router.GET("/_apis/pipelines/workflows/:runId/artifacts", func(w http.ResponseWriter, req *http.Request, params httprouter.Params) {
		runID := params.ByName("runId")

		entries, err := fs.ReadDir(fsys, runID)
		if err != nil {
			panic(err)
		}

		var list []NamedFileContainerResourceURL
		for _, entry := range entries {
			list = append(list, NamedFileContainerResourceURL{
				Name:                     entry.Name(),
				FileContainerResourceURL: fmt.Sprintf("http://%s/download/%s", req.Host, runID),
			})
		}

		json, err := json.Marshal(NamedFileContainerResourceURLResponse{
			Count: len(list),
			Value: list,
		})
		if err != nil {
			panic(err)
		}

		_, err = w.Write(json)
		if err != nil {
			panic(err)
		}
	})

	router.GET("/download/:container", func(w http.ResponseWriter, req *http.Request, params httprouter.Params) {
		container := params.ByName("container")
		itemPath := req.URL.Query().Get("itemPath")
		dirPath := fmt.Sprintf("%s/%s", container, itemPath)

		var files []ContainerItem
		err := fs.WalkDir(fsys, dirPath, func(path string, entry fs.DirEntry, err error) error {
			if !entry.IsDir() {
				rel, err := filepath.Rel(dirPath, path)
				if err != nil {
					panic(err)
				}

				// if it was uploaded as gzip
				rel = strings.TrimSuffix(rel, gzipExtension)

				files = append(files, ContainerItem{
					Path:            fmt.Sprintf("%s/%s", itemPath, rel),
					ItemType:        "file",
					ContentLocation: fmt.Sprintf("http://%s/artifact/%s/%s/%s", req.Host, container, itemPath, rel),
				})
			}
			return nil
		})
		if err != nil {
			panic(err)
		}

		json, err := json.Marshal(ContainerItemResponse{
			Value: files,
		})
		if err != nil {
			panic(err)
		}

		_, err = w.Write(json)
		if err != nil {
			panic(err)
		}
	})

	router.GET("/artifact/*path", func(w http.ResponseWriter, req *http.Request, params httprouter.Params) {
		path := params.ByName("path")[1:]

		file, err := fsys.Open(path)
		if err != nil {
			// try gzip file
			file, err = fsys.Open(path + gzipExtension)
			if err != nil {
				panic(err)
			}
			w.Header().Add("Content-Encoding", "gzip")
		}

		_, err = io.Copy(w, file)
		if err != nil {
			panic(err)
		}
	})
}

func Serve(ctx context.Context, artifactPath string, port string) context.CancelFunc {
	serverContext, cancel := context.WithCancel(ctx)
	logger := common.Logger(serverContext)

	if artifactPath == "" {
		return cancel
	}

	router := httprouter.New()

	logger.Debugf("Artifacts base path '%s'", artifactPath)
	fs := os.DirFS(artifactPath)
	uploads(router, MkdirFsImpl{artifactPath, fs})
	downloads(router, fs)
	ip := common.GetOutboundIP().String()

	server := &http.Server{
		Addr:              fmt.Sprintf("%s:%s", ip, port),
		ReadHeaderTimeout: 2 * time.Second,
		Handler:           router,
	}

	// run server
	go func() {
		logger.Infof("Start server on http://%s:%s", ip, port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal(err)
		}
	}()

	// wait for cancel to gracefully shutdown server
	go func() {
		<-serverContext.Done()

		if err := server.Shutdown(ctx); err != nil {
			logger.Errorf("Failed shutdown gracefully - force shutdown: %v", err)
			server.Close()
		}
	}()

	return cancel
}
