package artifacts

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/julienschmidt/httprouter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/nektos/act/pkg/common"
)

func newTestServer(t *testing.T) (*httptest.Server, string, []byte) {
	t.Helper()
	baseDir := t.TempDir()
	authKey, err := NewAuthKey()
	require.NoError(t, err)
	srv := httptest.NewServer(newHandler(baseDir, authKey))
	t.Cleanup(srv.Close)
	return srv, baseDir, authKey
}

func doRequest(t *testing.T, method, target, token, body string) (int, string) {
	t.Helper()
	req, err := http.NewRequest(method, target, strings.NewReader(body))
	require.NoError(t, err)
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp.StatusCode, string(data)
}

func TestNewAuthKeyIsRandom(t *testing.T) {
	a, err := NewAuthKey()
	require.NoError(t, err)
	b, err := NewAuthKey()
	require.NoError(t, err)
	assert.Len(t, a, 32)
	assert.NotEqual(t, a, b)
}

func TestServerRejectsRequestsWithoutValidRuntimeToken(t *testing.T) {
	srv, baseDir, authKey := newTestServer(t)
	require.NoError(t, os.MkdirAll(filepath.Join(baseDir, "1", "art"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(baseDir, "1", "art", "secret.txt"), []byte("secret"), 0o600))

	emptyKeyToken, err := common.CreateAuthorizationToken(1, 1, 1)
	require.NoError(t, err)
	otherKeyToken, err := common.CreateSignedAuthorizationToken([]byte("some other key"), 1, 1, 1)
	require.NoError(t, err)

	routes := []struct{ method, path, body string }{
		{http.MethodPost, "/_apis/pipelines/workflows/1/artifacts", ""},
		{http.MethodPatch, "/_apis/pipelines/workflows/1/artifacts", ""},
		{http.MethodGet, "/_apis/pipelines/workflows/1/artifacts", ""},
		{http.MethodPut, "/upload/1?itemPath=art/new.txt", "data"},
		{http.MethodGet, "/download/1?itemPath=art", ""},
		{http.MethodGet, "/artifact/1/art/secret.txt", ""},
		{http.MethodPost, ArtifactV4RouteBase + "/CreateArtifact", `{"workflowRunBackendId":"1","workflowJobRunBackendId":"1","name":"x","version":4}`},
		{http.MethodPost, ArtifactV4RouteBase + "/FinalizeArtifact", `{"workflowRunBackendId":"1","workflowJobRunBackendId":"1","name":"x"}`},
		{http.MethodPost, ArtifactV4RouteBase + "/ListArtifacts", `{"workflowRunBackendId":"1","workflowJobRunBackendId":"1"}`},
		{http.MethodPost, ArtifactV4RouteBase + "/GetSignedArtifactURL", `{"workflowRunBackendId":"1","workflowJobRunBackendId":"1","name":"art"}`},
		{http.MethodPost, ArtifactV4RouteBase + "/DeleteArtifact", `{"workflowRunBackendId":"1","workflowJobRunBackendId":"1","name":"art"}`},
	}
	tokens := map[string]string{
		"no token":         "",
		"empty key token":  emptyKeyToken,
		"other key token":  otherKeyToken,
		"garbage token":    "not-a-jwt",
		"raw key as token": base64.StdEncoding.EncodeToString(authKey),
	}

	for _, route := range routes {
		for name, token := range tokens {
			t.Run(fmt.Sprintf("%s %s/%s", route.method, route.path, name), func(t *testing.T) {
				status, _ := doRequest(t, route.method, srv.URL+route.path, token, route.body)
				assert.Equal(t, http.StatusUnauthorized, status)
			})
		}
	}

	// nothing was written or deleted by the rejected requests
	_, err = os.Stat(filepath.Join(baseDir, "1", "art", "new.txt"))
	assert.True(t, os.IsNotExist(err))
	data, err := os.ReadFile(filepath.Join(baseDir, "1", "art", "secret.txt"))
	require.NoError(t, err)
	assert.Equal(t, "secret", string(data))
}

func TestServerV3FlowWithRuntimeToken(t *testing.T) {
	srv, _, authKey := newTestServer(t)
	token, err := common.CreateSignedAuthorizationToken(authKey, 1, 1, 1)
	require.NoError(t, err)

	status, body := doRequest(t, http.MethodPut, srv.URL+"/upload/1?itemPath=art/file.txt", token, "hello")
	require.Equal(t, http.StatusOK, status, body)

	status, body = doRequest(t, http.MethodGet, srv.URL+"/_apis/pipelines/workflows/1/artifacts", token, "")
	require.Equal(t, http.StatusOK, status, body)
	list := NamedFileContainerResourceURLResponse{}
	require.NoError(t, json.Unmarshal([]byte(body), &list))
	require.Equal(t, 1, list.Count)
	assert.Equal(t, "art", list.Value[0].Name)

	status, body = doRequest(t, http.MethodGet, srv.URL+"/download/1?itemPath=art", token, "")
	require.Equal(t, http.StatusOK, status, body)
	items := ContainerItemResponse{}
	require.NoError(t, json.Unmarshal([]byte(body), &items))
	require.Len(t, items.Value, 1)

	status, body = doRequest(t, http.MethodGet, items.Value[0].ContentLocation, token, "")
	require.Equal(t, http.StatusOK, status, body)
	assert.Equal(t, "hello", body)
}

func TestServerV4SignedURLs(t *testing.T) {
	srv, baseDir, authKey := newTestServer(t)
	token, err := common.CreateSignedAuthorizationToken(authKey, 7, 7, 7)
	require.NoError(t, err)

	status, body := doRequest(t, http.MethodPost, srv.URL+ArtifactV4RouteBase+"/CreateArtifact", token,
		`{"workflowRunBackendId":"7","workflowJobRunBackendId":"7","name":"art","version":4}`)
	require.Equal(t, http.StatusOK, status, body)
	created := CreateArtifactResponse{}
	require.NoError(t, protojson.Unmarshal([]byte(body), &created))
	require.NotEmpty(t, created.SignedUploadUrl)

	// signed URLs are used without the runtime token (Azure blob client)
	status, body = doRequest(t, http.MethodPut, created.SignedUploadUrl+"&comp=block", "", "zipdata")
	require.Equal(t, http.StatusCreated, status, body)

	status, body = doRequest(t, http.MethodPost, srv.URL+ArtifactV4RouteBase+"/GetSignedArtifactURL", token,
		`{"workflowRunBackendId":"7","workflowJobRunBackendId":"7","name":"art"}`)
	require.Equal(t, http.StatusOK, status, body)
	signed := GetSignedArtifactURLResponse{}
	require.NoError(t, protojson.Unmarshal([]byte(body), &signed))

	status, body = doRequest(t, http.MethodGet, signed.SignedUrl, "", "")
	require.Equal(t, http.StatusOK, status, body)
	assert.Equal(t, "zipdata", body)

	// a URL signed with the formerly hardcoded key is rejected
	expires := time.Now().Add(time.Hour).Format("2006-01-02 15:04:05.999999999 -0700 MST")
	mac := hmac.New(sha256.New, []byte{0xba, 0xdb, 0xee, 0xf0})
	mac.Write([]byte("UploadArtifact"))
	mac.Write([]byte(expires))
	mac.Write([]byte("forged"))
	mac.Write([]byte("7"))
	forged := fmt.Sprintf("%s%s/UploadArtifact?sig=%s&expires=%s&artifactName=forged&taskID=7&comp=block",
		srv.URL, ArtifactV4RouteBase, base64.URLEncoding.EncodeToString(mac.Sum(nil)), url.QueryEscape(expires))
	status, _ = doRequest(t, http.MethodPut, forged, "", "payload")
	assert.Equal(t, http.StatusUnauthorized, status)
	_, err = os.Stat(filepath.Join(baseDir, "7", "forged", "forged.zip"))
	assert.True(t, os.IsNotExist(err))
}

func TestV4SigningKeyIsPerProcess(t *testing.T) {
	a := &artifactV4Routes{signingKey: newSigningKey()}
	b := &artifactV4Routes{signingKey: newSigningKey()}
	assert.NotEqual(t, a.buildSignature("UploadArtifact", "e", "n", 1), b.buildSignature("UploadArtifact", "e", "n", 1))
}

func TestV4DownloadMissingArtifact(t *testing.T) {
	route := &artifactV4Routes{
		rfs:        fstest.MapFS{},
		baseDir:    "artifact/server/path",
		prefix:     ArtifactV4RouteBase,
		signingKey: newSigningKey(),
		AppURL:     "localhost",
	}
	signedURL := route.buildArtifactURL("DownloadArtifact", "missing", 1)
	req := httptest.NewRequest(http.MethodGet, signedURL, nil)
	rr := httptest.NewRecorder()
	route.downloadArtifact(&ArtifactContext{Req: req, Resp: rr})
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestRejectBaseDirAsRunID(t *testing.T) {
	memfs := fstest.MapFS(map[string]*fstest.MapFile{
		"artifact/server/path/1/art/file":   {Data: []byte("one")},
		"artifact/server/path/2/other/file": {Data: []byte("two")},
	})

	router := httprouter.New()
	uploads(router, "artifact/server/path", writeMapFS{memfs})
	downloads(router, "artifact/server/path", memfs)

	for _, runID := range []string{".", ".."} {
		for _, tc := range []struct{ method, path string }{
			{http.MethodPost, "/_apis/pipelines/workflows/" + runID + "/artifacts"},
			{http.MethodGet, "/_apis/pipelines/workflows/" + runID + "/artifacts"},
			{http.MethodPut, "/upload/" + runID + "?itemPath=injected/file"},
			{http.MethodGet, "/download/" + runID},
		} {
			t.Run(tc.method+" "+tc.path, func(t *testing.T) {
				req := httptest.NewRequest(tc.method, "http://localhost"+tc.path, strings.NewReader("x"))
				rr := httptest.NewRecorder()
				router.ServeHTTP(rr, req)
				assert.Equal(t, http.StatusBadRequest, rr.Code)
			})
		}
	}
	assert.NotContains(t, memfs, "artifact/server/path/injected/file")
}

func TestDownloadContainerItemPathStaysInContainer(t *testing.T) {
	memfs := fstest.MapFS(map[string]*fstest.MapFile{
		"artifact/server/path/1/art/file":   {Data: []byte("one")},
		"artifact/server/path/2/other/file": {Data: []byte("two")},
	})

	router := httprouter.New()
	downloads(router, "artifact/server/path", memfs)

	req := httptest.NewRequest(http.MethodGet, "http://localhost/download/1?itemPath=..", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code)

	response := ContainerItemResponse{}
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &response))
	for _, item := range response.Value {
		assert.NotContains(t, item.ContentLocation, "other", "itemPath=.. must not list other runs")
	}
	require.Len(t, response.Value, 1)
	assert.Equal(t, "art/file", response.Value[0].Path)
	assert.Equal(t, "http://localhost/artifact/1//art/file", response.Value[0].ContentLocation)

	req = httptest.NewRequest(http.MethodGet, response.Value[0].ContentLocation, nil)
	rr = httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "one", rr.Body.String())
}

type closeTrackingFS struct {
	fs.FS
	open *int
}

type closeTrackingFile struct {
	fs.File
	open *int
}

func (f closeTrackingFile) Close() error {
	*f.open--
	return f.File.Close()
}

func (fsys closeTrackingFS) Open(name string) (fs.File, error) {
	f, err := fsys.FS.Open(name)
	if err != nil {
		return nil, err
	}
	*fsys.open++
	return closeTrackingFile{File: f, open: fsys.open}, nil
}

func TestDownloadArtifactFileClosesFile(t *testing.T) {
	open := 0
	memfs := closeTrackingFS{
		FS: fstest.MapFS(map[string]*fstest.MapFile{
			"artifact/server/path/1/some/file": {Data: []byte("content")},
		}),
		open: &open,
	}

	router := httprouter.New()
	downloads(router, "artifact/server/path", memfs)

	req := httptest.NewRequest(http.MethodGet, "http://localhost/artifact/1/some/file", nil)
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, "content", rr.Body.String())
	assert.Equal(t, 0, open, "file handle leaked")
}
