package artifacts

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
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

// readWriteFSImpl serves paths below base through an os.Root, so a symlink
// inside the artifact directory cannot redirect a read or write outside it.
// Names are full paths that start with base.
type readWriteFSImpl struct {
	base string
}

func (fwfs readWriteFSImpl) open() (*os.Root, error) {
	if err := os.MkdirAll(fwfs.base, os.ModePerm); err != nil {
		return nil, err
	}
	return os.OpenRoot(fwfs.base)
}

func (fwfs readWriteFSImpl) rel(name string) (string, error) {
	rel, err := filepath.Rel(fwfs.base, name)
	if err != nil {
		return "", err
	}
	if !filepath.IsLocal(rel) {
		return "", &fs.PathError{Op: "open", Path: name, Err: fs.ErrPermission}
	}
	return rel, nil
}

func (fwfs readWriteFSImpl) Open(name string) (fs.File, error) {
	rel, err := fwfs.rel(name)
	if err != nil {
		return nil, err
	}
	root, err := fwfs.open()
	if err != nil {
		return nil, err
	}
	defer root.Close()
	return root.Open(rel)
}

func (fwfs readWriteFSImpl) openFile(name string, flag int) (*os.File, error) {
	rel, err := fwfs.rel(name)
	if err != nil {
		return nil, err
	}
	root, err := fwfs.open()
	if err != nil {
		return nil, err
	}
	defer root.Close()
	if dir := filepath.Dir(rel); dir != "." {
		if err := root.MkdirAll(dir, os.ModePerm); err != nil {
			return nil, err
		}
	}
	return root.OpenFile(rel, flag, 0o644)
}

func (fwfs readWriteFSImpl) OpenWritable(name string) (WritableFile, error) {
	return fwfs.openFile(name, os.O_CREATE|os.O_RDWR|os.O_TRUNC)
}

func (fwfs readWriteFSImpl) OpenAppendable(name string) (WritableFile, error) {
	file, err := fwfs.openFile(name, os.O_CREATE|os.O_RDWR)
	if err != nil {
		return nil, err
	}

	_, err = file.Seek(0, io.SeekEnd)
	if err != nil {
		file.Close()
		return nil, err
	}
	return file, nil
}

func (fwfs readWriteFSImpl) RemoveAll(name string) error {
	rel, err := fwfs.rel(name)
	if err != nil {
		return err
	}
	root, err := fwfs.open()
	if err != nil {
		return err
	}
	defer root.Close()
	return root.RemoveAll(rel)
}

var gzipExtension = ".gz__"

func safeResolve(baseDir string, relPath string) string {
	return filepath.Join(baseDir, filepath.Clean(filepath.Join(string(os.PathSeparator), relPath)))
}

// isValidRunID reports whether runID names one directory directly below the artifact root.
func isValidRunID(runID string) bool {
	return runID != "" && runID != "." && runID != ".." && !strings.ContainsAny(runID, `/\`)
}

func validArtifactName(name string) bool {
	return isValidRunID(name)
}

func hasPathTraversal(p string) bool {
	if unesc, err := url.PathUnescape(p); err == nil {
		p = unesc
	}
	p = strings.ReplaceAll(filepath.ToSlash(p), `\`, "/")
	for _, seg := range strings.Split(p, "/") {
		if seg == ".." {
			return true
		}
	}
	return false
}

// pathUnder resolves rel inside root. It rejects absolute paths and ".." segments.
func pathUnder(root, rel string) (string, bool) {
	if rel != "" && (filepath.IsAbs(rel) || hasPathTraversal(rel)) {
		return "", false
	}
	resolved := safeResolve(root, rel)
	loc, err := filepath.Rel(root, resolved)
	if err != nil {
		return "", false
	}
	if loc == ".." || strings.HasPrefix(loc, ".."+string(os.PathSeparator)) {
		return "", false
	}
	return resolved, true
}

func rejectRunID(w http.ResponseWriter, runID string) bool {
	if isValidRunID(runID) {
		return false
	}
	http.Error(w, "invalid run id", http.StatusBadRequest)
	return true
}

func rejectPath(w http.ResponseWriter) {
	http.Error(w, "invalid path", http.StatusBadRequest)
}

func uploads(router *httprouter.Router, baseDir string, fsys WriteFS) {
	router.POST("/_apis/pipelines/workflows/:runId/artifacts", func(w http.ResponseWriter, req *http.Request, params httprouter.Params) {
		runID := params.ByName("runId")
		if rejectRunID(w, runID) {
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
		if rejectRunID(w, runID) {
			return
		}

		if req.Header.Get("Content-Encoding") == "gzip" {
			itemPath += gzipExtension
		}

		safeRunPath := safeResolve(baseDir, runID)
		safePath, ok := pathUnder(safeRunPath, itemPath)
		if !ok {
			rejectPath(w)
			return
		}

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

	router.PATCH("/_apis/pipelines/workflows/:runId/artifacts", func(w http.ResponseWriter, _ *http.Request, params httprouter.Params) {
		if rejectRunID(w, params.ByName("runId")) {
			return
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
}

func downloads(router *httprouter.Router, baseDir string, fsys fs.FS) {
	router.GET("/_apis/pipelines/workflows/:runId/artifacts", func(w http.ResponseWriter, req *http.Request, params httprouter.Params) {
		runID := params.ByName("runId")
		if rejectRunID(w, runID) {
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
		if rejectRunID(w, container) {
			return
		}
		itemPath := req.URL.Query().Get("itemPath")
		safePath, ok := pathUnder(safeResolve(baseDir, container), itemPath)
		if !ok {
			rejectPath(w)
			return
		}

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
		raw := params.ByName("path")
		raw = strings.TrimPrefix(raw, "/")
		if hasPathTraversal(raw) {
			rejectPath(w)
			return
		}
		runID, rest, found := strings.Cut(raw, "/")
		if !found {
			http.Error(w, "invalid run id", http.StatusBadRequest)
			return
		}
		if rejectRunID(w, runID) {
			return
		}
		safePath, ok := pathUnder(safeResolve(baseDir, runID), rest)
		if !ok {
			rejectPath(w)
			return
		}

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

func isSignedURLRoute(req *http.Request) bool {
	switch req.URL.Path {
	case ArtifactV4RouteBase + "/UploadArtifact", ArtifactV4RouteBase + "/DownloadArtifact":
		return true
	default:
		return false
	}
}

func isV4RPC(urlPath string) bool {
	if !strings.HasPrefix(urlPath, ArtifactV4RouteBase+"/") {
		return false
	}
	switch strings.TrimPrefix(urlPath, ArtifactV4RouteBase+"/") {
	case "CreateArtifact", "FinalizeArtifact", "ListArtifacts", "GetSignedArtifactURL", "DeleteArtifact":
		return true
	default:
		return false
	}
}

func runIDFromURLPath(urlPath string) (string, bool) {
	switch {
	case strings.HasPrefix(urlPath, "/_apis/pipelines/workflows/"):
		rest := strings.TrimPrefix(urlPath, "/_apis/pipelines/workflows/")
		id, _, ok := strings.Cut(rest, "/")
		return id, ok && id != ""
	case strings.HasPrefix(urlPath, "/upload/"):
		id := strings.TrimPrefix(urlPath, "/upload/")
		id, _, _ = strings.Cut(id, "/")
		return id, id != ""
	case strings.HasPrefix(urlPath, "/download/"):
		id := strings.TrimPrefix(urlPath, "/download/")
		id, _, _ = strings.Cut(id, "/")
		return id, id != ""
	case strings.HasPrefix(urlPath, "/artifact/"):
		rest := strings.TrimPrefix(urlPath, "/artifact/")
		id, _, _ := strings.Cut(rest, "/")
		return id, id != ""
	default:
		return "", false
	}
}

func sameRun(tokenRunID int64, raw string) bool {
	return raw == strconv.FormatInt(tokenRunID, 10)
}

func workflowRunIDFromBody(body []byte) (string, bool) {
	if len(bytes.TrimSpace(body)) == 0 {
		return "", false
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(body, &raw); err != nil {
		return "", false
	}
	for _, key := range []string{"workflowRunBackendId", "workflow_run_backend_id"} {
		val, ok := raw[key]
		if !ok || bytes.Equal(val, []byte("null")) {
			continue
		}
		var id string
		if err := json.Unmarshal(val, &id); err != nil || id == "" {
			continue
		}
		return id, true
	}
	return "", false
}

func readLimitedBody(req *http.Request) ([]byte, error) {
	if req.Body == nil {
		return nil, nil
	}
	body, err := io.ReadAll(io.LimitReader(req.Body, 1<<20+1))
	if err != nil {
		return nil, err
	}
	if len(body) > 1<<20 {
		return nil, fmt.Errorf("request body too large")
	}
	req.Body = io.NopCloser(bytes.NewReader(body))
	return body, nil
}

func authorizeRequestRun(w http.ResponseWriter, req *http.Request, tokenRunID int64) bool {
	if raw, ok := runIDFromURLPath(req.URL.Path); ok {
		if rejectRunID(w, raw) {
			return false
		}
		if !sameRun(tokenRunID, raw) {
			http.Error(w, "forbidden", http.StatusForbidden)
			return false
		}
	}
	if !isV4RPC(req.URL.Path) {
		return true
	}
	body, err := readLimitedBody(req)
	if err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return false
	}
	raw, ok := workflowRunIDFromBody(body)
	if !ok {
		return true
	}
	if rejectRunID(w, raw) {
		return false
	}
	if !sameRun(tokenRunID, raw) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return false
	}
	return true
}

// requireRuntimeToken requires a bearer token signed with authKey on every route
// except the v4 upload and download URLs, which authorize themselves with the URL signature.
// The token's run ID must match the run named by the request.
func requireRuntimeToken(next http.Handler, authKey []byte) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if isSignedURLRoute(req) {
			next.ServeHTTP(w, req)
			return
		}
		runID, err := common.ValidateAuthorizationToken(authKey, req)
		if err != nil {
			common.Logger(req.Context()).Debugf("Rejected artifact server request method=%q path=%q: %v", req.Method, req.URL.Path, err)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if !authorizeRequestRun(w, req, runID) {
			return
		}
		next.ServeHTTP(w, req)
	})
}

// NewAuthKey returns a random key for Serve and runner.Config.ArtifactServerAuthKey.
func NewAuthKey() ([]byte, error) {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("generate artifact server auth key: %w", err)
	}
	return key, nil
}

func newHandler(artifactPath string, authKey []byte) http.Handler {
	router := httprouter.New()
	fsys := readWriteFSImpl{base: artifactPath}
	uploads(router, artifactPath, fsys)
	downloads(router, artifactPath, fsys)
	RoutesV4(router, artifactPath, fsys, fsys)
	return requireRuntimeToken(router, authKey)
}

// Serve starts the artifact server.
// Every request except the v4 upload and download URLs must carry an Authorization
// bearer token created by common.CreateSignedAuthorizationToken with authKey for the
// same run. Pass that key as runner.Config.ArtifactServerAuthKey so jobs receive it.
// An empty authKey rejects every bearer request. v4 upload and download URLs are
// authorized by a per-process signature instead.
func Serve(ctx context.Context, artifactPath string, addr string, port string, authKey []byte) context.CancelFunc {
	serverContext, cancel := context.WithCancel(ctx)
	logger := common.Logger(serverContext)

	if artifactPath == "" {
		return cancel
	}

	logger.Debugf("Artifacts base path '%s'", artifactPath)
	if len(authKey) == 0 {
		logger.Warn("Artifact server has no auth key; bearer requests will be rejected")
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
