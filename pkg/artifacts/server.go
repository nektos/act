package artifacts

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
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

type WritableFile interface {
	io.WriteCloser
}

type WriteFS interface {
	OpenWritable(name string) (WritableFile, error)
	OpenAppendable(name string) (WritableFile, error)
}

type readWriteFSImpl struct {
}

func (fwfs readWriteFSImpl) Open(name string) (fs.File, error) {
	return os.Open(name)
}

func (fwfs readWriteFSImpl) OpenWritable(name string) (WritableFile, error) {
	if err := os.MkdirAll(filepath.Dir(name), os.ModePerm); err != nil {
		return nil, err
	}
	return os.OpenFile(name, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0o644)
}

func (fwfs readWriteFSImpl) OpenAppendable(name string) (WritableFile, error) {
	if err := os.MkdirAll(filepath.Dir(name), os.ModePerm); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(name, os.O_CREATE|os.O_RDWR, 0o644)

	if err != nil {
		return nil, err
	}

	_, err = file.Seek(0, io.SeekEnd)
	if err != nil {
		return nil, err
	}
	return file, nil
}

var gzipExtension = ".gz__"

func safeResolve(baseDir string, relPath string) string {
	return filepath.Join(baseDir, filepath.Clean(filepath.Join(string(os.PathSeparator), relPath)))
}

// isValidRunID reports whether runID names a single run directory below the
// artifact base directory. safeResolve keeps every path inside the base
// directory, but "", "." and ".." resolve to the base directory itself, which
// would let a caller list or write across all runs.
func isValidRunID(runID string) bool {
	return runID != "" && runID != "." && runID != ".." && !strings.ContainsAny(runID, `/\`)
}

func writeInvalidRunID(w http.ResponseWriter) {
	http.Error(w, "invalid run id", http.StatusBadRequest)
}

func uploads(router *httprouter.Router, baseDir string, fsys WriteFS) {
	router.POST("/_apis/pipelines/workflows/:runId/artifacts", func(w http.ResponseWriter, req *http.Request, params httprouter.Params) {
		runID := params.ByName("runId")
		if !isValidRunID(runID) {
			writeInvalidRunID(w)
			return
		}

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
		if !isValidRunID(runID) {
			writeInvalidRunID(w)
			return
		}

		if req.Header.Get("Content-Encoding") == "gzip" {
			itemPath += gzipExtension
		}

		safeRunPath := safeResolve(baseDir, runID)
		safePath := safeResolve(safeRunPath, itemPath)

		file, err := func() (WritableFile, error) {
			contentRange := req.Header.Get("Content-Range")
			if contentRange != "" && !strings.HasPrefix(contentRange, "bytes 0-") {
				return fsys.OpenAppendable(safePath)
			}
			return fsys.OpenWritable(safePath)
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

	router.PATCH("/_apis/pipelines/workflows/:runId/artifacts", func(w http.ResponseWriter, _ *http.Request, _ httprouter.Params) {
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

func downloads(router *httprouter.Router, baseDir string, fsys fs.FS) {
	router.GET("/_apis/pipelines/workflows/:runId/artifacts", func(w http.ResponseWriter, req *http.Request, params httprouter.Params) {
		runID := params.ByName("runId")
		if !isValidRunID(runID) {
			writeInvalidRunID(w)
			return
		}

		safePath := safeResolve(baseDir, runID)

		entries, err := fs.ReadDir(fsys, safePath)
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
		if !isValidRunID(container) {
			writeInvalidRunID(w)
			return
		}
		// resolve itemPath inside the container, so that e.g. itemPath=.. cannot
		// walk the whole artifact store
		itemPath := strings.TrimPrefix(filepath.ToSlash(filepath.Clean("/"+req.URL.Query().Get("itemPath"))), "/")
		safePath := safeResolve(safeResolve(baseDir, container), itemPath)

		var files []ContainerItem
		err := fs.WalkDir(fsys, safePath, func(path string, entry fs.DirEntry, _ error) error {
			if !entry.IsDir() {
				rel, err := filepath.Rel(safePath, path)
				if err != nil {
					panic(err)
				}

				// if it was upload as gzip
				rel = strings.TrimSuffix(rel, gzipExtension)
				path := filepath.Join(itemPath, rel)

				rel = filepath.ToSlash(rel)
				path = filepath.ToSlash(path)

				files = append(files, ContainerItem{
					Path:            path,
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

	router.GET("/artifact/*path", func(w http.ResponseWriter, _ *http.Request, params httprouter.Params) {
		path := params.ByName("path")[1:]

		safePath := safeResolve(baseDir, path)

		file, err := fsys.Open(safePath)
		if err != nil {
			// try gzip file
			file, err = fsys.Open(safePath + gzipExtension)
			if err != nil {
				panic(err)
			}
			w.Header().Add("Content-Encoding", "gzip")
		}
		defer file.Close()

		_, err = io.Copy(w, file)
		if err != nil {
			panic(err)
		}
	})
}

// isSignedURLRoute reports whether the request targets one of the v4 blob
// routes. Those are called by the Azure blob client / plain HTTP client of
// @actions/artifact without the runtime token, and are instead authorized by
// the HMAC signature of the URL handed out by CreateArtifact and
// GetSignedArtifactURL.
func isSignedURLRoute(req *http.Request) bool {
	switch req.URL.Path {
	case ArtifactV4RouteBase + "/UploadArtifact", ArtifactV4RouteBase + "/DownloadArtifact":
		return true
	}
	return false
}

// requireRuntimeToken only lets requests through that carry an
// ACTIONS_RUNTIME_TOKEN signed with authKey (see common.CreateSignedAuthorizationToken),
// except for the v4 signed-URL routes which verify their own signature.
func requireRuntimeToken(next http.Handler, authKey []byte) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if !isSignedURLRoute(req) {
			if err := common.ValidateAuthorizationToken(authKey, req); err != nil {
				common.Logger(req.Context()).Debugf("Rejected unauthorized artifact server request %s %s: %v", req.Method, req.URL.Path, err)
				http.Error(w, "unauthorized", http.StatusUnauthorized)
				return
			}
		}
		next.ServeHTTP(w, req)
	})
}

// NewAuthKey returns a random key suitable for Serve and runner.Config.ArtifactServerAuthKey.
func NewAuthKey() ([]byte, error) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("generate artifact server auth key: %w", err)
	}
	return key, nil
}

func newHandler(artifactPath string, authKey []byte) http.Handler {
	router := httprouter.New()

	fsys := readWriteFSImpl{}
	uploads(router, artifactPath, fsys)
	downloads(router, artifactPath, fsys)
	RoutesV4(router, artifactPath, fsys, fsys)

	return requireRuntimeToken(router, authKey)
}

// Serve starts the artifact server. All requests, apart from the v4 signed
// upload/download URLs, must carry an "Authorization: Bearer" token created by
// common.CreateSignedAuthorizationToken with authKey; pass the same key as
// runner.Config.ArtifactServerAuthKey so that jobs receive such a token in
// ACTIONS_RUNTIME_TOKEN. authKey should be random per process and must not be
// empty, otherwise every request is rejected.
func Serve(ctx context.Context, artifactPath string, addr string, port string, authKey []byte) context.CancelFunc {
	serverContext, cancel := context.WithCancel(ctx)
	logger := common.Logger(serverContext)

	if artifactPath == "" {
		return cancel
	}

	logger.Debugf("Artifacts base path '%s'", artifactPath)
	if len(authKey) == 0 {
		logger.Warn("Artifact server started without an auth key, all requests will be rejected")
	}

	server := &http.Server{
		Addr:              fmt.Sprintf("%s:%s", addr, port),
		ReadHeaderTimeout: 2 * time.Second,
		Handler:           newHandler(artifactPath, authKey),
	}

	// run server
	go func() {
		logger.Infof("Start server on http://%s:%s", addr, port)
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
