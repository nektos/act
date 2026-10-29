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
	assert.NotEqual(t, newSigningKey(), newSigningKey())
}

func TestServerRejectsRequestsWithoutValidRuntimeToken(t *testing.T) {
	srv, baseDir, _ := newTestServer(t)
	require.NoError(t, os.MkdirAll(filepath.Join(baseDir, "1", "art"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(baseDir, "1", "art", "secret.txt"), []byte("secret"), 0o600))

	emptyKeyToken, err := common.CreateSignedAuthorizationToken(nil, 1, 1, 1)
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
		"no token":        "",
		"empty key token": emptyKeyToken,
		"other key token": otherKeyToken,
		"garbage token":   "not-a-jwt",
	}

	for _, route := range routes {
		for name, token := range tokens {
			t.Run(fmt.Sprintf("%s %s/%s", route.method, route.path, name), func(t *testing.T) {
				status, _ := doRequest(t, route.method, srv.URL+route.path, token, route.body)
				assert.Equal(t, http.StatusUnauthorized, status)
			})
		}
	}

	_, err = os.Stat(filepath.Join(baseDir, "1", "art", "new.txt"))
	assert.True(t, os.IsNotExist(err))
	data, err := os.ReadFile(filepath.Join(baseDir, "1", "art", "secret.txt"))
	require.NoError(t, err)
	assert.Equal(t, "secret", string(data))
}

func TestServerRejectsEmptyAuthKey(t *testing.T) {
	baseDir := t.TempDir()
	srv := httptest.NewServer(newHandler(baseDir, nil))
	t.Cleanup(srv.Close)
	token, err := common.CreateSignedAuthorizationToken(nil, 1, 1, 1)
	require.NoError(t, err)
	status, _ := doRequest(t, http.MethodGet, srv.URL+"/_apis/pipelines/workflows/1/artifacts", token, "")
	assert.Equal(t, http.StatusUnauthorized, status)
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

func TestServerRejectsOtherRun(t *testing.T) {
	srv, baseDir, authKey := newTestServer(t)
	require.NoError(t, os.MkdirAll(filepath.Join(baseDir, "2", "art"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(baseDir, "2", "art", "secret.txt"), []byte("other"), 0o600))
	token, err := common.CreateSignedAuthorizationToken(authKey, 1, 1, 1)
	require.NoError(t, err)

	for _, target := range []struct{ method, path string }{
		{http.MethodGet, "/_apis/pipelines/workflows/2/artifacts"},
		{http.MethodPut, "/upload/2?itemPath=art/new.txt"},
		{http.MethodGet, "/download/2?itemPath=art"},
		{http.MethodGet, "/artifact/2/art/secret.txt"},
	} {
		t.Run(target.method+" "+target.path, func(t *testing.T) {
			status, body := doRequest(t, target.method, srv.URL+target.path, token, "data")
			assert.Equal(t, http.StatusForbidden, status, body)
			assert.NotContains(t, body, "other")
		})
	}

	status, body := doRequest(t, http.MethodPost, srv.URL+ArtifactV4RouteBase+"/CreateArtifact", token,
		`{"workflowRunBackendId":"2","workflowJobRunBackendId":"2","name":"x","version":4}`)
	assert.Equal(t, http.StatusForbidden, status, body)
	_, err = os.Stat(filepath.Join(baseDir, "2", "x"))
	assert.True(t, os.IsNotExist(err))
	_, err = os.Stat(filepath.Join(baseDir, "2", "art", "new.txt"))
	assert.True(t, os.IsNotExist(err))
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

	status, body = doRequest(t, http.MethodPut, created.SignedUploadUrl+"&comp=block", "", "zipdata")
	require.Equal(t, http.StatusCreated, status, body)

	status, body = doRequest(t, http.MethodPost, srv.URL+ArtifactV4RouteBase+"/GetSignedArtifactURL", token,
		`{"workflow_run_backend_id":"7","workflow_job_run_backend_id":"7","name":"art"}`)
	require.Equal(t, http.StatusOK, status, body)
	signed := GetSignedArtifactURLResponse{}
	require.NoError(t, protojson.Unmarshal([]byte(body), &signed))

	status, body = doRequest(t, http.MethodGet, signed.SignedUrl, "", "")
	require.Equal(t, http.StatusOK, status, body)
	assert.Equal(t, "zipdata", body)

	expires := time.Now().Add(time.Hour).Format("2006-01-02 15:04:05.999999999 -0700 MST")
	mac := hmac.New(sha256.New, []byte{0xba, 0xdb, 0xee, 0xf0})
	mac.Write([]byte("UploadArtifact"))
	mac.Write([]byte(expires))
	mac.Write([]byte("forged"))
	mac.Write([]byte("7"))
	forged := fmt.Sprintf("%s%s/UploadArtifact?sig=%s&expires=%s&artifactName=forged&taskID=7&comp=block",
		srv.URL, ArtifactV4RouteBase, url.QueryEscape(base64.URLEncoding.EncodeToString(mac.Sum(nil))), url.QueryEscape(expires))
	status, _ = doRequest(t, http.MethodPut, forged, "", "payload")
	assert.Equal(t, http.StatusUnauthorized, status)
	_, err = os.Stat(filepath.Join(baseDir, "7", "forged", "forged.zip"))
	assert.True(t, os.IsNotExist(err))
}

func TestV4SignatureFieldsAreNotInterchangeable(t *testing.T) {
	route := &artifactV4Routes{
		rfs:        fstest.MapFS{},
		baseDir:    t.TempDir(),
		prefix:     ArtifactV4RouteBase,
		signingKey: newSigningKey(),
		AppURL:     "localhost",
	}
	signed := route.buildArtifactURL("localhost", "DownloadArtifact", "a1", 2)
	untampered := httptest.NewRequest(http.MethodGet, signed, nil)
	okRR := httptest.NewRecorder()
	route.downloadArtifact(&ArtifactContext{Req: untampered, Resp: okRR})
	assert.Equal(t, http.StatusNotFound, okRR.Code)

	parsed, err := url.Parse(signed)
	require.NoError(t, err)
	query := parsed.Query()
	query.Set("artifactName", "a")
	query.Set("taskID", "12")
	parsed.RawQuery = query.Encode()
	tampered := httptest.NewRequest(http.MethodGet, parsed.String(), nil)
	rr := httptest.NewRecorder()
	route.downloadArtifact(&ArtifactContext{Req: tampered, Resp: rr})
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestV4DownloadMissingArtifact(t *testing.T) {
	route := &artifactV4Routes{
		rfs:        fstest.MapFS{},
		baseDir:    t.TempDir(),
		prefix:     ArtifactV4RouteBase,
		signingKey: newSigningKey(),
		AppURL:     "localhost",
	}
	signedURL := route.buildArtifactURL("localhost", "DownloadArtifact", "missing", 1)
	req := httptest.NewRequest(http.MethodGet, signedURL, nil)
	rr := httptest.NewRecorder()
	route.downloadArtifact(&ArtifactContext{Req: req, Resp: rr})
	assert.Equal(t, http.StatusNotFound, rr.Code)
}

func TestRejectTraversalAndBaseDirRunID(t *testing.T) {
	srv, baseDir, authKey := newTestServer(t)
	require.NoError(t, os.MkdirAll(filepath.Join(baseDir, "1", "art"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(baseDir, "1", "art", "file"), []byte("one"), 0o600))
	require.NoError(t, os.MkdirAll(filepath.Join(baseDir, "2", "other"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(baseDir, "2", "other", "file"), []byte("two"), 0o600))
	token, err := common.CreateSignedAuthorizationToken(authKey, 1, 1, 1)
	require.NoError(t, err)

	for _, target := range []struct{ method, path string }{
		{http.MethodGet, "/_apis/pipelines/workflows/./artifacts"},
		{http.MethodPost, "/_apis/pipelines/workflows/./artifacts"},
		{http.MethodGet, "/_apis/pipelines/workflows/../artifacts"},
		{http.MethodPut, "/upload/."},
		{http.MethodPut, "/upload/..?itemPath=injected/file"},
		{http.MethodGet, "/download/."},
		{http.MethodGet, "/download/.."},
	} {
		t.Run(target.method+" "+target.path, func(t *testing.T) {
			status, _ := doRequest(t, target.method, srv.URL+target.path, token, "x")
			assert.Equal(t, http.StatusBadRequest, status)
		})
	}

	traversals := []struct{ method, path string }{
		{http.MethodPut, "/upload/1?itemPath=../../2/other/file"},
		{http.MethodPut, "/upload/1?itemPath=../2/other/file"},
		{http.MethodGet, "/download/1?itemPath=.."},
		{http.MethodGet, "/download/1?itemPath=../2/other"},
		{http.MethodGet, "/artifact/1/../../2/other/file"},
		{http.MethodGet, "/artifact/1/../2/other/file"},
	}
	for _, tc := range traversals {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			status, body := doRequest(t, tc.method, srv.URL+tc.path, token, "x")
			assert.Equal(t, http.StatusBadRequest, status, body)
			assert.NotContains(t, body, "two")
		})
	}
	_, err = os.Stat(filepath.Join(baseDir, "injected"))
	assert.True(t, os.IsNotExist(err))
	data, err := os.ReadFile(filepath.Join(baseDir, "2", "other", "file"))
	require.NoError(t, err)
	assert.Equal(t, "two", string(data))

	status, body := doRequest(t, http.MethodPost, srv.URL+ArtifactV4RouteBase+"/DeleteArtifact", token,
		`{"workflowRunBackendId":"1","name":"../2"}`)
	assert.Equal(t, http.StatusBadRequest, status, body)
	_, err = os.Stat(filepath.Join(baseDir, "2", "other", "file"))
	require.NoError(t, err)
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

func TestReadWriteFSStaysInsideBase(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "artifacts")
	outside := filepath.Join(dir, "outside")
	require.NoError(t, os.MkdirAll(filepath.Join(base, "1"), 0o755))
	require.NoError(t, os.MkdirAll(outside, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(outside, "secret"), []byte("s"), 0o600))
	require.NoError(t, os.Symlink(outside, filepath.Join(base, "1", "link")))

	fsys := readWriteFSImpl{base: base}
	_, err := fsys.OpenWritable(filepath.Join(base, "1", "link", "evil"))
	assert.Error(t, err)
	_, err = os.Stat(filepath.Join(outside, "evil"))
	assert.True(t, os.IsNotExist(err))

	_, err = fsys.Open(filepath.Join(base, "1", "link", "secret"))
	assert.Error(t, err)
	_, err = fsys.Open(filepath.Join(dir, "outside", "secret"))
	assert.Error(t, err)

	assert.NoError(t, fsys.RemoveAll(filepath.Join(base, "1", "link")))
	_, err = os.Stat(filepath.Join(outside, "secret"))
	assert.NoError(t, err)

	f, err := fsys.OpenWritable(filepath.Join(base, "1", "art", "file"))
	require.NoError(t, err)
	_, err = f.Write([]byte("ok"))
	assert.NoError(t, err)
	assert.NoError(t, f.Close())
	got, err := os.ReadFile(filepath.Join(base, "1", "art", "file"))
	assert.NoError(t, err)
	assert.Equal(t, "ok", string(got))
}

func TestV4SignatureRejectsFarFutureExpiry(t *testing.T) {
	route := &artifactV4Routes{
		rfs:        fstest.MapFS{},
		baseDir:    t.TempDir(),
		prefix:     ArtifactV4RouteBase,
		signingKey: newSigningKey(),
	}
	expires := time.Now().Add(24 * 365 * time.Hour).Format("2006-01-02 15:04:05.999999999 -0700 MST")
	sig := base64.URLEncoding.EncodeToString(route.buildSignature("DownloadArtifact", expires, "a1", 1))
	target := fmt.Sprintf("http://localhost%s/DownloadArtifact?sig=%s&expires=%s&artifactName=a1&taskID=1",
		ArtifactV4RouteBase, url.QueryEscape(sig), url.QueryEscape(expires))
	rr := httptest.NewRecorder()
	route.downloadArtifact(&ArtifactContext{Req: httptest.NewRequest(http.MethodGet, target, nil), Resp: rr})
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
}

func TestV4SignedURLHost(t *testing.T) {
	route := artifactV4Routes{}
	for host, want := range map[string]bool{
		"127.0.0.1:34567":  true,
		"act.local":        true,
		"[::1]:8080":       true,
		"":                 false,
		"evil.test/path":   false,
		"user@evil.test":   false,
		"evil.test?x=1":    false,
		"evil.test#frag":   false,
		"evil.test:80/../": false,
	} {
		req := httptest.NewRequest(http.MethodPost, "http://placeholder/", nil)
		req.Host = host
		got, ok := route.signedURLHost(req)
		assert.Equal(t, want, ok, host)
		if want {
			assert.Equal(t, host, got)
		}
	}

	fixed := artifactV4Routes{AppURL: "10.0.0.5:34567/"}
	got, ok := fixed.signedURLHost(httptest.NewRequest(http.MethodPost, "http://evil.test/", nil))
	assert.True(t, ok)
	assert.Equal(t, "10.0.0.5:34567", got)
}
